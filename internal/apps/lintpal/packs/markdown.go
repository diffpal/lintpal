package packs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

const markdownLockSchema = "lintpal.packs.lock.v2"
const maxGitHubTreeBytes = 2 << 20

// ImportLocalMarkdown installs all Markdown mandates beneath a local directory.
func ImportLocalMarkdown(ctx context.Context, root, name, dir string, update bool) (Entry, error) {
	if !packName.MatchString(name) || dir == "" {
		return Entry{}, ErrSource
	}
	abs := dir
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	abs = filepath.Clean(abs)
	mandates, err := rules.ReadMandates(ctx, abs)
	if err != nil {
		return Entry{}, err
	}
	locator := abs
	if relative, err := filepath.Rel(root, abs); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		locator = filepath.ToSlash(relative)
	}
	return InstallMarkdown(ctx, root, name, mandates, Source{Kind: "local", Locator: locator}, update)
}

// InstallMarkdown atomically activates an immutable directory of Markdown files.
func InstallMarkdown(ctx context.Context, root, name string, mandates []rules.Mandate, source Source, update bool) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	if !packName.MatchString(name) || !validSource(source) {
		return Entry{}, ErrSource
	}
	if _, err := rules.CompileMandates(mandates); err != nil {
		return Entry{}, err
	}
	ordered := append([]rules.Mandate(nil), mandates...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	digest := hashMandates(ordered)
	lock, err := readMarkdownLock(root, true)
	if err != nil {
		return Entry{}, err
	}
	index := -1
	for i, entry := range lock.Packs {
		if entry.Name == name {
			index = i
			break
		}
	}
	if (index >= 0) != update {
		return Entry{}, ErrConflict
	}
	entry := Entry{Name: name, SourceKind: source.Kind, Source: source.Locator,
		RequestedRef: source.RequestedRef, ResolvedCommit: source.ResolvedCommit,
		Path: "packs/" + name + "/" + digest, SHA256: digest}
	base := filepath.Join(root, ".lintpal")
	packDir := filepath.Join(base, "packs", name)
	for _, dir := range []string{base, filepath.Join(base, "packs"), packDir} {
		if err := ensureDirectory(root, dir); err != nil {
			return Entry{}, err
		}
	}
	target := filepath.Join(base, filepath.FromSlash(entry.Path))
	if _, err := os.Lstat(target); err == nil {
		if _, err := verifyMarkdownEntry(ctx, root, entry); err != nil {
			return Entry{}, ErrDrift
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Entry{}, ErrStorage
	} else {
		temp, err := os.MkdirTemp(packDir, ".pack-*")
		if err != nil {
			return Entry{}, ErrStorage
		}
		defer func() { _ = os.RemoveAll(temp) }()
		for _, mandate := range ordered {
			if err := ctx.Err(); err != nil {
				return Entry{}, err
			}
			file := filepath.Join(temp, filepath.FromSlash(mandate.ID))
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				return Entry{}, ErrStorage
			}
			writer, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return Entry{}, ErrStorage
			}
			_, writeErr := writer.WriteString(mandate.Body)
			syncErr := writer.Sync()
			closeErr := writer.Close()
			if writeErr != nil || syncErr != nil || closeErr != nil {
				return Entry{}, ErrStorage
			}
		}
		if err := os.Rename(temp, target); err != nil {
			return Entry{}, ErrStorage
		}
	}
	if index < 0 {
		lock.Packs = append(lock.Packs, entry)
	} else {
		lock.Packs[index] = entry
	}
	sort.Slice(lock.Packs, func(i, j int) bool { return lock.Packs[i].Name < lock.Packs[j].Name })
	if err := writeLock(root, base, lock); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func hashMandates(mandates []rules.Mandate) string {
	h := sha256.New()
	var size [8]byte
	for _, mandate := range mandates {
		binary.BigEndian.PutUint64(size[:], uint64(len(mandate.ID)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(mandate.ID))
		binary.BigEndian.PutUint64(size[:], uint64(len(mandate.Body)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(mandate.Body))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func readMarkdownLock(root string, missingAllowed bool) (lockfile, error) {
	file := filepath.Join(root, ".lintpal", "packs.lock.json")
	if err := checkPath(root, file, false); err != nil {
		if missingAllowed && errors.Is(err, os.ErrNotExist) {
			return lockfile{Schema: markdownLockSchema}, nil
		}
		return lockfile{}, ErrLock
	}
	data, err := readBounded(context.Background(), file, maxLockBytes)
	if err != nil {
		return lockfile{}, ErrLock
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var lock lockfile
	if decoder.Decode(&lock) != nil || decoder.Decode(new(any)) != io.EOF {
		return lockfile{}, ErrLock
	}
	if lock.Schema == lockSchema {
		return lockfile{}, ErrLegacyFormat
	}
	if lock.Schema != markdownLockSchema || (len(lock.Packs) == 0 && !missingAllowed) {
		return lockfile{}, ErrLock
	}
	seen := make(map[string]bool, len(lock.Packs))
	for _, entry := range lock.Packs {
		if seen[entry.Name] || !validMarkdownEntry(entry) {
			return lockfile{}, ErrLock
		}
		seen[entry.Name] = true
	}
	return lock, nil
}

func validMarkdownEntry(entry Entry) bool {
	return packName.MatchString(entry.Name) && contentSHA.MatchString(entry.SHA256) &&
		entry.Path == "packs/"+entry.Name+"/"+entry.SHA256 &&
		validSource(Source{Kind: entry.SourceKind, Locator: entry.Source,
			RequestedRef: entry.RequestedRef, ResolvedCommit: entry.ResolvedCommit})
}

// GetMarkdown returns one validated v2 lock entry.
func GetMarkdown(root, name string) (Entry, error) {
	if !packName.MatchString(name) {
		return Entry{}, ErrSource
	}
	lock, err := readMarkdownLock(root, false)
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range lock.Packs {
		if entry.Name == name {
			return entry, nil
		}
	}
	return Entry{}, ErrSource
}

// VerifyMarkdown verifies all or one installed pack without network access.
func VerifyMarkdown(ctx context.Context, root, name string) ([]Entry, error) {
	lock, err := readMarkdownLock(root, false)
	if err != nil {
		return nil, err
	}
	var verified []Entry
	for _, entry := range lock.Packs {
		if name != "" && name != entry.Name {
			continue
		}
		if _, err := verifyMarkdownEntry(ctx, root, entry); err != nil {
			return nil, err
		}
		verified = append(verified, entry)
	}
	if name != "" && len(verified) == 0 {
		return nil, ErrSource
	}
	return verified, nil
}

// LoadMarkdown returns one verified managed pack for offline linting.
func LoadMarkdown(ctx context.Context, root, name string) (rules.Pack, error) {
	entry, err := GetMarkdown(root, name)
	if err != nil {
		return rules.Pack{}, err
	}
	return verifyMarkdownEntry(ctx, root, entry)
}

func verifyMarkdownEntry(ctx context.Context, root string, entry Entry) (rules.Pack, error) {
	dir := filepath.Join(root, ".lintpal", filepath.FromSlash(entry.Path))
	if err := checkPath(root, dir, true); err != nil {
		return rules.Pack{}, ErrDrift
	}
	entries := 0
	if err := filepath.WalkDir(dir, func(file string, item fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > 4096 {
			return ErrDrift
		}
		if walkErr != nil || item.Type()&os.ModeSymlink != 0 {
			return ErrDrift
		}
		if !item.IsDir() && (!item.Type().IsRegular() || !strings.HasSuffix(file, ".md")) {
			return ErrDrift
		}
		return nil
	}); err != nil {
		if ctx.Err() != nil {
			return rules.Pack{}, ctx.Err()
		}
		return rules.Pack{}, ErrDrift
	}
	mandates, err := rules.ReadMandates(ctx, dir)
	if err != nil {
		if ctx.Err() != nil {
			return rules.Pack{}, ctx.Err()
		}
		return rules.Pack{}, ErrDrift
	}
	if hashMandates(mandates) != entry.SHA256 {
		return rules.Pack{}, ErrDrift
	}
	pack, err := rules.CompileMandates(mandates)
	if err != nil {
		return rules.Pack{}, ErrDrift
	}
	return pack, nil
}

// ImportGitHubMarkdown pins a public source to a commit and imports its Markdown tree.
func ImportGitHubMarkdown(ctx context.Context, root, name, spec string, update bool) (Entry, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	return importGitHubMarkdown(ctx, root, name, spec, update, client, githubAPI, githubRaw)
}

func importGitHubMarkdown(ctx context.Context, root, name, spec string, update bool, client *http.Client, apiBase, rawBase string) (Entry, error) {
	source, err := parseGitHub(spec)
	if err != nil || client == nil {
		return Entry{}, ErrSource
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	base := apiBase + "/repos/" + source.owner + "/" + source.repo
	commitData, err := githubGet(ctx, &copyClient, base+"/commits/"+url.PathEscape(source.ref), maxCommitResponse)
	if err != nil {
		return Entry{}, err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if json.Unmarshal(commitData, &commit) != nil || !commitSHA.MatchString(commit.SHA) {
		return Entry{}, ErrSource
	}
	treeData, err := githubGet(ctx, &copyClient, base+"/git/trees/"+commit.SHA+"?recursive=1", maxGitHubTreeBytes)
	if err != nil {
		return Entry{}, err
	}
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
			Type string `json:"type"`
			Size int64  `json:"size"`
		} `json:"tree"`
	}
	if json.Unmarshal(treeData, &tree) != nil || tree.Truncated || len(tree.Tree) == 0 {
		return Entry{}, ErrSource
	}
	var mandates []rules.Mandate
	for _, item := range tree.Tree {
		if err := ctx.Err(); err != nil {
			return Entry{}, err
		}
		rel := item.Path
		if source.dir != "" {
			var ok bool
			rel, ok = strings.CutPrefix(rel, source.dir+"/")
			if !ok {
				continue
			}
		}
		if rel == "" || !fsValidMarkdownPath(rel) {
			return Entry{}, ErrSource
		}
		if item.Type == "tree" && item.Mode == "040000" {
			continue
		}
		if item.Type != "blob" || item.Mode != "100644" {
			return Entry{}, ErrSource
		}
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		if len(mandates) >= 256 || item.Size < 1 || item.Size > 8<<10 {
			return Entry{}, ErrSource
		}
		parts := strings.Split(item.Path, "/")
		for i := range parts {
			parts[i] = url.PathEscape(parts[i])
		}
		data, err := githubGet(ctx, &copyClient, rawBase+"/"+source.owner+"/"+source.repo+"/"+commit.SHA+"/"+strings.Join(parts, "/"), 8<<10)
		if err != nil {
			return Entry{}, err
		}
		if int64(len(data)) != item.Size {
			return Entry{}, ErrSource
		}
		mandates = append(mandates, rules.Mandate{ID: rel, Body: string(data)})
	}
	locator := "github:" + source.owner + "/" + source.repo
	if source.dir != "" {
		locator += "//" + source.dir
	}
	return InstallMarkdown(ctx, root, name, mandates, Source{Kind: "github", Locator: locator,
		RequestedRef: source.ref, ResolvedCommit: commit.SHA}, update)
}

func fsValidMarkdownPath(value string) bool {
	return len(value) <= 63 && !path.IsAbs(value) && path.Clean(value) == value &&
		value != "." && !strings.Contains(value, "\\") && !strings.ContainsRune(value, 0)
}
