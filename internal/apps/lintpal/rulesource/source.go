// Package rulesource resolves a worktree and reads pinned remote Markdown rules.
package rulesource

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

var ErrSource = errors.New("invalid rule source")

const githubAPI = "https://api.github.com"
const githubRaw = "https://raw.githubusercontent.com"
const maxCommitResponse = 64 << 10
const maxGitHubTreeBytes = 2 << 20

var githubComponent = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
var githubRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)
var commitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

type githubSource struct {
	owner string
	repo  string
	dir   string
	ref   string
}

// ReadGitHubMarkdown returns one commit-pinned Markdown tree without installing it.
func ReadGitHubMarkdown(ctx context.Context, spec string) ([]rules.Mandate, string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	return readGitHubMarkdown(ctx, spec, client, githubAPI, githubRaw)
}

func readGitHubMarkdown(ctx context.Context, spec string, client *http.Client, apiBase, rawBase string) ([]rules.Mandate, string, error) {
	source, err := parseGitHub(spec)
	if err != nil || client == nil {
		return nil, "", ErrSource
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	base := apiBase + "/repos/" + source.owner + "/" + source.repo
	commitData, err := githubGet(ctx, &copyClient, base+"/commits/"+url.PathEscape(source.ref), maxCommitResponse)
	if err != nil {
		return nil, "", err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if json.Unmarshal(commitData, &commit) != nil || !commitSHA.MatchString(commit.SHA) {
		return nil, "", ErrSource
	}
	treeData, err := githubGet(ctx, &copyClient, base+"/git/trees/"+commit.SHA+"?recursive=1", maxGitHubTreeBytes)
	if err != nil {
		return nil, "", err
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
		return nil, "", ErrSource
	}
	var mandates []rules.Mandate
	for _, item := range tree.Tree {
		if err := ctx.Err(); err != nil {
			return nil, "", err
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
			return nil, "", ErrSource
		}
		if item.Type == "tree" && item.Mode == "040000" {
			continue
		}
		if item.Type != "blob" || item.Mode != "100644" {
			return nil, "", ErrSource
		}
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		if len(mandates) >= 256 || item.Size < 1 || item.Size > 8<<10 {
			return nil, "", ErrSource
		}
		parts := strings.Split(item.Path, "/")
		for i := range parts {
			parts[i] = url.PathEscape(parts[i])
		}
		data, err := githubGet(ctx, &copyClient, rawBase+"/"+source.owner+"/"+source.repo+"/"+commit.SHA+"/"+strings.Join(parts, "/"), 8<<10)
		if err != nil {
			return nil, "", err
		}
		if int64(len(data)) != item.Size {
			return nil, "", ErrSource
		}
		mandates = append(mandates, rules.Mandate{ID: rel, Body: string(data)})
	}
	if _, err := rules.CompileMandates(mandates); err != nil {
		return nil, "", err
	}
	return mandates, commit.SHA, nil
}

func fsValidMarkdownPath(value string) bool {
	return len(value) <= 63 && !path.IsAbs(value) && path.Clean(value) == value &&
		value != "." && !strings.Contains(value, "\\") && !strings.ContainsRune(value, 0)
}

func parseGitHub(spec string) (githubSource, error) {
	rest, ok := strings.CutPrefix(spec, "github:")
	if !ok {
		return githubSource{}, ErrSource
	}
	at := strings.LastIndexByte(rest, '@')
	if at < 0 || at == len(rest)-1 {
		return githubSource{}, ErrSource
	}
	ref := rest[at+1:]
	if !githubRef.MatchString(ref) || strings.Contains(ref, "..") || strings.Contains(ref, "//") || strings.HasSuffix(ref, "/") {
		return githubSource{}, ErrSource
	}
	location := rest[:at]
	repoPart, dir, hasDir := strings.Cut(location, "//")
	parts := strings.Split(repoPart, "/")
	if len(parts) != 2 || !githubComponent.MatchString(parts[0]) || !githubComponent.MatchString(parts[1]) ||
		parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
		return githubSource{}, ErrSource
	}
	if hasDir {
		if dir == "" || strings.Contains(dir, "//") {
			return githubSource{}, ErrSource
		}
		for _, part := range strings.Split(dir, "/") {
			if !githubComponent.MatchString(part) || part == "." || part == ".." {
				return githubSource{}, ErrSource
			}
		}
	}
	return githubSource{owner: parts[0], repo: parts[1], dir: dir, ref: ref}, nil
}

func githubGet(ctx context.Context, client *http.Client, target string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, ErrSource
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "lintpal")
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrSource
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, ErrSource
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || len(data) > int(limit) {
		return nil, ErrSource
	}
	return data, nil
}
