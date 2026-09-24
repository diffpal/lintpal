package packs

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGitHubImportPinsCommitAndBytes(t *testing.T) {
	root := t.TempDir()
	sha := strings.Repeat("a", 40)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/repos/acme/rules/commits/v1.2.0":
			_, _ = w.Write([]byte(`{"sha":"` + sha + `"}`))
		case "/acme/rules/" + sha + "/go/rules.yaml":
			_, _ = w.Write([]byte(validRules))
		default:
			t.Errorf("unexpected GitHub path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	entry, err := importGitHub(t.Context(), root, "go", "github:acme/rules//go@v1.2.0", false, server.Client(), server.URL, server.URL)
	if err != nil || entry.Source != "github:acme/rules//go" || entry.RequestedRef != "v1.2.0" || entry.ResolvedCommit != sha || calls.Load() != 2 {
		t.Fatalf("GitHub import: %+v, calls=%d, err=%v", entry, calls.Load(), err)
	}
	server.Close()
	if _, err := Verify(t.Context(), root, "go"); err != nil {
		t.Fatalf("offline verify: %v", err)
	}
	if _, err := Load(t.Context(), root, "go"); err != nil {
		t.Fatalf("offline load: %v", err)
	}
}

func TestGitHubImportRejectsUnsafeSourcesAndResponses(t *testing.T) {
	for _, spec := range []string{
		"github:acme/rules", "github:acme/rules@", "github:acme/rules//../bad@v1",
		"github:acme/rules//go//bad@v1", "github:acme/rules@../main", "https://github.com/acme/rules@v1",
	} {
		if _, err := parseGitHub(spec); !errors.Is(err, ErrSource) {
			t.Fatalf("unsafe source %q: %v", spec, err)
		}
	}
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://example.invalid/steal")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	if _, err := importGitHub(t.Context(), root, "go", "github:acme/rules@v1", false, server.Client(), server.URL, server.URL); !errors.Is(err, ErrSource) {
		t.Fatalf("redirect accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".lintpal")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed import wrote storage: %v", err)
	}
}
