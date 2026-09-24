package rulesource

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadGitHubMarkdownPinsCommitAndSubdirectory(t *testing.T) {
	sha := strings.Repeat("a", 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/rules/commits/v1":
			_, _ = w.Write([]byte(`{"sha":"` + sha + `"}`))
		case "/repos/acme/rules/git/trees/" + sha:
			_, _ = w.Write([]byte(`{"truncated":false,"tree":[{"path":"go/errors.md","mode":"100644","type":"blob","size":15}]}`))
		case "/acme/rules/" + sha + "/go/errors.md":
			_, _ = w.Write([]byte("Handle errors.\n"))
		default:
			t.Errorf("unexpected request: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	mandates, commit, err := readGitHubMarkdown(t.Context(), "github:acme/rules//go@v1", server.Client(), server.URL, server.URL)
	if err != nil || commit != sha || len(mandates) != 1 || mandates[0].ID != "errors.md" || mandates[0].Body != "Handle errors.\n" {
		t.Fatalf("pinned source: %+v, %s, %v", mandates, commit, err)
	}
}

func TestReadGitHubMarkdownRejectsUnsafeSpecs(t *testing.T) {
	for _, spec := range []string{
		"github:acme/rules", "github:acme/rules@", "github:acme/rules//../bad@v1",
		"github:acme/rules//go//bad@v1", "github:acme/rules@../main", "https://github.com/acme/rules@v1",
	} {
		if _, _, err := readGitHubMarkdown(t.Context(), spec, http.DefaultClient, "http://127.0.0.1:1", "http://127.0.0.1:1"); !errors.Is(err, ErrSource) {
			t.Fatalf("accepted %q: %v", spec, err)
		}
	}
}
