package packs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const githubAPI = "https://api.github.com"
const githubRaw = "https://raw.githubusercontent.com"
const maxCommitResponse = 64 << 10

var githubComponent = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
var githubRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)

type githubSource struct {
	owner string
	repo  string
	dir   string
	ref   string
}

// ImportGitHub resolves an explicit public GitHub ref to one commit, then
// installs only rules.yaml from that commit. It never executes source content.
func ImportGitHub(ctx context.Context, root, name, spec string, update bool) (Entry, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	return importGitHub(ctx, root, name, spec, update, client, githubAPI, githubRaw)
}

func importGitHub(ctx context.Context, root, name, spec string, update bool, client *http.Client, apiBase, rawBase string) (Entry, error) {
	source, err := parseGitHub(spec)
	if err != nil {
		return Entry{}, err
	}
	if client == nil {
		return Entry{}, ErrSource
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	commitURL := apiBase + "/repos/" + source.owner + "/" + source.repo + "/commits/" + url.PathEscape(source.ref)
	commitBody, err := githubGet(ctx, &copyClient, commitURL, maxCommitResponse)
	if err != nil {
		return Entry{}, err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if json.Unmarshal(commitBody, &commit) != nil || !commitSHA.MatchString(commit.SHA) {
		return Entry{}, ErrSource
	}
	path := "rules.yaml"
	if source.dir != "" {
		path = source.dir + "/" + path
	}
	rawURL := rawBase + "/" + source.owner + "/" + source.repo + "/" + commit.SHA + "/" + path
	data, err := githubGet(ctx, &copyClient, rawURL, maxPackBytes)
	if err != nil {
		return Entry{}, err
	}
	locator := "github:" + source.owner + "/" + source.repo
	if source.dir != "" {
		locator += "//" + source.dir
	}
	return Install(ctx, root, name, data, Source{Kind: "github", Locator: locator,
		RequestedRef: source.ref, ResolvedCommit: commit.SHA}, update)
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
