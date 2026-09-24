package packs

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

func TestMarkdownPackLocalSnapshotAndDrift(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "go"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "go", "errors.md"), []byte("Handle returned errors.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	entry, err := ImportLocalMarkdown(t.Context(), root, "team", source, false)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := LoadMarkdown(t.Context(), root, "team")
	if err != nil || len(pack.Rules()) != 1 || pack.Rules()[0].ID != "go/errors.md" {
		t.Fatalf("load: %+v, %v", pack.Rules(), err)
	}
	if err := os.WriteFile(filepath.Join(source, "go", "errors.md"), []byte("New source, not active."), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMarkdown(t.Context(), root, "team"); err != nil {
		t.Fatalf("source edit affected snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lintpal", filepath.FromSlash(entry.Path), "go", "errors.md"), []byte("drift"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMarkdown(t.Context(), root, "team"); !errors.Is(err, ErrDrift) {
		t.Fatalf("accepted drift: %v", err)
	}
	if _, err := ImportLocalMarkdown(t.Context(), root, "team", source, true); err != nil {
		t.Fatal(err)
	}
	before, err := GetMarkdown(root, "team")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "go", "errors.md"), []byte(" "), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportLocalMarkdown(t.Context(), root, "team", source, true); err == nil {
		t.Fatal("accepted invalid update")
	}
	after, err := GetMarkdown(root, "team")
	if err != nil || before != after {
		t.Fatalf("failed update switched lock: before=%+v after=%+v err=%v", before, after, err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lintpal", filepath.FromSlash(after.Path), "extra.txt"), []byte("untracked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMarkdown(t.Context(), root, "team"); !errors.Is(err, ErrDrift) {
		t.Fatalf("accepted added file: %v", err)
	}
}

func TestMarkdownPackDigestIgnoresEnumerationOrder(t *testing.T) {
	a := []rules.Mandate{{ID: "b.md", Body: "B"}, {ID: "a.md", Body: "A"}}
	b := []rules.Mandate{{ID: "a.md", Body: "A"}, {ID: "b.md", Body: "B"}}
	root1, root2 := t.TempDir(), t.TempDir()
	one, err := InstallMarkdown(t.Context(), root1, "team", a, Source{Kind: "local", Locator: "rules"}, false)
	if err != nil {
		t.Fatal(err)
	}
	two, err := InstallMarkdown(t.Context(), root2, "team", b, Source{Kind: "local", Locator: "rules"}, false)
	if err != nil || one.SHA256 != two.SHA256 {
		t.Fatalf("unstable digest: %+v, %+v, %v", one, two, err)
	}
}

func TestMarkdownPackGitHubPinnedTree(t *testing.T) {
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
	root := t.TempDir()
	entry, err := importGitHubMarkdown(t.Context(), root, "team", "github:acme/rules//go@v1", false, server.Client(), server.URL, server.URL)
	if err != nil || entry.ResolvedCommit != sha {
		t.Fatalf("import: %+v, %v", entry, err)
	}
	server.Close()
	if _, err := VerifyMarkdown(t.Context(), root, "team"); err != nil {
		t.Fatalf("offline verify: %v", err)
	}
}

func TestMarkdownPackRejectsLegacyLock(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".lintpal"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lintpal", "packs.lock.json"), []byte(`{"schema":"lintpal.packs.lock.v1","packs":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := GetMarkdown(root, "team"); !errors.Is(err, ErrLegacyFormat) {
		t.Fatalf("legacy lock: %v", err)
	}
}
