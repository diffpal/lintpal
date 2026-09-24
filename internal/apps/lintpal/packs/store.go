// Package packs installs and verifies project-local declarative rule packs.
package packs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

var ErrSource = errors.New("invalid pack source")
var ErrLock = errors.New("invalid pack lockfile")
var ErrConflict = errors.New("pack name conflict")
var ErrDrift = errors.New("installed pack differs from lockfile")
var ErrStorage = errors.New("pack storage failed")

const lockSchema = "lintpal.packs.lock.v1"
const maxPackBytes = 256 << 10
const maxLockBytes = 1 << 20

var packName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
var commitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
var contentSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Source struct {
	Kind           string
	Locator        string
	RequestedRef   string
	ResolvedCommit string
}

type Entry struct {
	Name           string `json:"name"`
	SourceKind     string `json:"source_kind"`
	Source         string `json:"source"`
	RequestedRef   string `json:"requested_ref,omitempty"`
	ResolvedCommit string `json:"resolved_commit,omitempty"`
	Path           string `json:"path"`
	SHA256         string `json:"sha256"`
}

type lockfile struct {
	Schema string  `json:"schema"`
	Packs  []Entry `json:"packs"`
}

// ImportLocal reads one rules.yaml in dir and installs it under name.
func ImportLocal(ctx context.Context, root, name, dir string, update bool) (Entry, error) {
	if !packName.MatchString(name) || dir == "" {
		return Entry{}, ErrSource
	}
	abs := dir
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	abs = filepath.Clean(abs)
	if err := checkPath(abs, abs, true); err != nil {
		return Entry{}, ErrSource
	}
	file := filepath.Join(abs, "rules.yaml")
	if err := checkPath(abs, file, false); err != nil {
		return Entry{}, ErrSource
	}
	data, err := readBounded(ctx, file, maxPackBytes)
	if err != nil {
		return Entry{}, err
	}
	locator := abs
	if relative, err := filepath.Rel(root, abs); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		locator = filepath.ToSlash(relative)
	}
	return Install(ctx, root, name, data, Source{Kind: "local", Locator: locator}, update)
}

// Install validates a pack and atomically activates its content through the lockfile.
func Install(ctx context.Context, root, name string, data []byte, source Source, update bool) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	if !packName.MatchString(name) || !validSource(source) || len(data) > maxPackBytes {
		return Entry{}, ErrSource
	}
	if _, err := rules.LoadContext(ctx, bytes.NewReader(data)); err != nil {
		return Entry{}, err
	}
	lock, err := readLock(root, true)
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
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	entry := Entry{Name: name, SourceKind: source.Kind, Source: source.Locator,
		RequestedRef: source.RequestedRef, ResolvedCommit: source.ResolvedCommit,
		Path: "packs/" + name + "/" + digest + ".yaml", SHA256: digest}
	base := filepath.Join(root, ".lintpal")
	dir := filepath.Join(base, "packs", name)
	if err := ensureDirectory(root, base); err != nil {
		return Entry{}, err
	}
	if err := ensureDirectory(root, filepath.Join(base, "packs")); err != nil {
		return Entry{}, err
	}
	if err := ensureDirectory(root, dir); err != nil {
		return Entry{}, err
	}
	file := filepath.Join(base, filepath.FromSlash(entry.Path))
	created, err := writeContent(root, file, data, update)
	if err != nil {
		return Entry{}, err
	}
	if index < 0 {
		lock.Packs = append(lock.Packs, entry)
	} else {
		lock.Packs[index] = entry
	}
	sort.Slice(lock.Packs, func(i, j int) bool { return lock.Packs[i].Name < lock.Packs[j].Name })
	if err := writeLock(root, base, lock); err != nil {
		if created {
			_ = os.Remove(file)
		}
		return Entry{}, err
	}
	return entry, nil
}

// Get returns one validated lock entry for explicit updates.
func Get(root, name string) (Entry, error) {
	if !packName.MatchString(name) {
		return Entry{}, ErrSource
	}
	lock, err := readLock(root, false)
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

// Verify checks installed bytes and parsed rules without network access.
func Verify(ctx context.Context, root, name string) ([]Entry, error) {
	lock, err := readLock(root, false)
	if err != nil {
		return nil, err
	}
	var verified []Entry
	for _, entry := range lock.Packs {
		if name != "" && entry.Name != name {
			continue
		}
		if _, err := verifyEntry(ctx, root, entry); err != nil {
			return nil, err
		}
		verified = append(verified, entry)
	}
	if name != "" && len(verified) == 0 {
		return nil, ErrSource
	}
	return verified, nil
}

// Load verifies and parses a named managed pack for offline linting.
func Load(ctx context.Context, root, name string) (rules.Pack, error) {
	entry, err := Get(root, name)
	if err != nil {
		return rules.Pack{}, err
	}
	return verifyEntry(ctx, root, entry)
}

func verifyEntry(ctx context.Context, root string, entry Entry) (rules.Pack, error) {
	file := filepath.Join(root, ".lintpal", filepath.FromSlash(entry.Path))
	if err := checkPath(root, file, false); err != nil {
		return rules.Pack{}, ErrDrift
	}
	data, err := readBounded(ctx, file, maxPackBytes)
	if err != nil {
		if ctx.Err() != nil {
			return rules.Pack{}, ctx.Err()
		}
		return rules.Pack{}, ErrDrift
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != entry.SHA256 {
		return rules.Pack{}, ErrDrift
	}
	pack, err := rules.LoadContext(ctx, bytes.NewReader(data))
	if err != nil {
		if ctx.Err() != nil {
			return rules.Pack{}, ctx.Err()
		}
		return rules.Pack{}, ErrDrift
	}
	return pack, nil
}

func readLock(root string, missingAllowed bool) (lockfile, error) {
	file := filepath.Join(root, ".lintpal", "packs.lock.json")
	if err := checkPath(root, file, false); err != nil {
		if missingAllowed && errors.Is(err, os.ErrNotExist) {
			return lockfile{Schema: lockSchema}, nil
		}
		return lockfile{}, ErrLock
	}
	reader, err := os.Open(file)
	if err != nil {
		return lockfile{}, ErrLock
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, maxLockBytes+1))
	if err != nil || len(data) > maxLockBytes {
		return lockfile{}, ErrLock
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var lock lockfile
	if decoder.Decode(&lock) != nil || decoder.Decode(new(any)) != io.EOF || lock.Schema != lockSchema || len(lock.Packs) == 0 {
		return lockfile{}, ErrLock
	}
	seen := make(map[string]bool, len(lock.Packs))
	for _, entry := range lock.Packs {
		if seen[entry.Name] || !validEntry(entry) {
			return lockfile{}, ErrLock
		}
		seen[entry.Name] = true
	}
	return lock, nil
}

func validEntry(entry Entry) bool {
	return packName.MatchString(entry.Name) && contentSHA.MatchString(entry.SHA256) &&
		entry.Path == "packs/"+entry.Name+"/"+entry.SHA256+".yaml" &&
		validSource(Source{Kind: entry.SourceKind, Locator: entry.Source,
			RequestedRef: entry.RequestedRef, ResolvedCommit: entry.ResolvedCommit})
}

func validSource(source Source) bool {
	if source.Locator == "" || strings.ContainsRune(source.Locator, 0) {
		return false
	}
	switch source.Kind {
	case "local":
		return source.RequestedRef == "" && source.ResolvedCommit == ""
	case "github":
		if !commitSHA.MatchString(source.ResolvedCommit) {
			return false
		}
		_, err := parseGitHub(source.Locator + "@" + source.RequestedRef)
		return err == nil
	default:
		return false
	}
}

func readBounded(ctx context.Context, file string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reader, err := os.Open(file)
	if err != nil {
		return nil, ErrSource
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || len(data) > int(limit) {
		return nil, ErrSource
	}
	return data, nil
}

func checkPath(root, path string, directory bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return ErrSource
	}
	clean, err := filepath.Abs(path)
	if err != nil {
		return ErrSource
	}
	relative, err := filepath.Rel(root, clean)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrSource
	}
	for part := clean; ; part = filepath.Dir(part) {
		info, err := os.Lstat(part)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrSource
		}
		if part == clean && info.IsDir() != directory {
			return ErrSource
		}
		if part == root {
			break
		}
	}
	return nil
}

func ensureDirectory(root, dir string) error {
	if err := checkPath(root, root, true); err != nil {
		return ErrStorage
	}
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return ErrStorage
	}
	if err := checkPath(root, dir, true); err != nil {
		return ErrStorage
	}
	return nil
}

func writeContent(root, path string, data []byte, repair bool) (bool, error) {
	hadExisting := false
	if err := checkPath(root, path, false); err == nil {
		hadExisting = true
		existing, err := os.ReadFile(path)
		if err != nil {
			return false, ErrDrift
		}
		if bytes.Equal(existing, data) {
			return false, nil
		}
		if !repair {
			return false, ErrDrift
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, ErrStorage
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".pack-*")
	if err != nil {
		return false, ErrStorage
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return false, ErrStorage
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return false, ErrStorage
	}
	if err := file.Close(); err != nil {
		return false, ErrStorage
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return false, ErrStorage
	}
	return !hadExisting, nil
}

func writeLock(root, base string, lock lockfile) error {
	path := filepath.Join(base, "packs.lock.json")
	if err := checkPath(root, path, false); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrLock
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil || len(data) > maxLockBytes {
		return ErrLock
	}
	data = append(data, '\n')
	file, err := os.CreateTemp(base, ".packs-lock-*")
	if err != nil {
		return ErrStorage
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return ErrStorage
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return ErrStorage
	}
	if err := file.Close(); err != nil {
		return ErrStorage
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return ErrStorage
	}
	return nil
}
