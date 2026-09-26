package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGitClientOpen(t *testing.T) {
	dir := testRepo(t)
	client, err := newGitClient(dir)
	if err != nil {
		t.Fatalf("failed to open valid git client: %v", err)
	}
	if client == nil || client.repo == nil {
		t.Fatal("client or repo is nil")
	}

	nonExistent := filepath.Join(t.TempDir(), "not-a-repo")
	_ = os.MkdirAll(nonExistent, 0700)
	if _, err := newGitClient(nonExistent); err == nil {
		t.Fatal("expected error opening non-existent git repository, got nil")
	}
}

func TestGitClientResolveCommit(t *testing.T) {
	dir := testRepo(t)
	commitSHA := testCommit(t, dir, "file.txt", "hello\n")
	client, err := newGitClient(dir)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Direct 40-hex SHA
	c, err := client.resolveCommit(t.Context(), commitSHA)
	if err != nil || c.Hash.String() != commitSHA {
		t.Fatalf("resolve direct SHA failed: commit=%v, err=%v", c, err)
	}

	// 2. HEAD ref
	c, err = client.resolveCommit(t.Context(), "HEAD")
	if err != nil || c.Hash.String() != commitSHA {
		t.Fatalf("resolve HEAD failed: commit=%v, err=%v", c, err)
	}

	// 3. Invalid inputs
	for _, bad := range []string{"", "--bad", "-flag", "non-existent-ref", "HEAD\x00extra"} {
		if _, err := client.resolveCommit(t.Context(), bad); !errors.Is(err, ErrInvalidRevision) {
			t.Fatalf("resolve %q: expected ErrInvalidRevision, got %v", bad, err)
		}
	}

	// 4. Cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.resolveCommit(ctx, commitSHA); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestGitClientResolveMergeBase(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "a.txt", "one\n")
	head := testCommit(t, dir, "a.txt", "two\n")
	client, err := newGitClient(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Single common ancestor
	got, err := client.resolve(t.Context(), base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	want := Revisions{Base: base, Head: head, MergeBase: base}
	if got != want {
		t.Fatalf("resolved = %+v, want %+v", got, want)
	}

	// Cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.resolve(ctx, base, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resolve error = %v", err)
	}

	// Unrelated histories
	runTestGit(t, dir, "checkout", "-q", "--orphan", "orphan")
	runTestGit(t, dir, "rm", "-qrf", ".")
	orphanHead := testCommit(t, dir, "b.txt", "two\n")
	if _, err := client.resolve(t.Context(), base, orphanHead); !errors.Is(err, ErrAmbiguousBase) {
		t.Fatalf("unrelated histories: expected ErrAmbiguousBase, got %v", err)
	}
}

func TestGitClientMultipleMergeBases(t *testing.T) {
	dir := testRepo(t)
	root := testCommit(t, dir, "root.txt", "root\n")
	a := testCommit(t, dir, "a.txt", "a\n")
	runTestGit(t, dir, "checkout", "-q", "-b", "other", root)
	b := testCommit(t, dir, "b.txt", "b\n")
	aTree := runTestGit(t, dir, "rev-parse", a+"^{tree}")
	bTree := runTestGit(t, dir, "rev-parse", b+"^{tree}")
	first := runTestGit(t, dir, "commit-tree", aTree, "-p", a, "-p", b, "-m", "first merge")
	second := runTestGit(t, dir, "commit-tree", bTree, "-p", b, "-p", a, "-m", "second merge")
	client, err := newGitClient(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.resolve(t.Context(), first, second)
	if !errors.Is(err, ErrAmbiguousBase) {
		t.Fatalf("multiple bases error = %v, expected ErrAmbiguousBase", err)
	}
}

func TestDiffCommits(t *testing.T) {
	dir := testRepo(t)
	testCommit(t, dir, "edit.txt", "line1\nline2\nline3\n")
	base := runTestGit(t, dir, "rev-parse", "HEAD")
	testCommit(t, dir, "edit.txt", "line1\nmodified2\nline3\nline4\n")
	head := runTestGit(t, dir, "rev-parse", "HEAD")

	client, err := newGitClient(dir)
	if err != nil {
		t.Fatal(err)
	}
	baseCommit, err := client.resolveCommit(t.Context(), base)
	if err != nil {
		t.Fatal(err)
	}
	headCommit, err := client.resolveCommit(t.Context(), head)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := client.diffCommits(t.Context(), baseCommit, headCommit, defaultOutputBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	fc := changes[0]
	if fc.file.newPath != "edit.txt" || fc.file.status != 'M' {
		t.Fatalf("unexpected change: %+v", fc.file)
	}
	if len(fc.spans) != 3 {
		t.Fatalf("expected 3 spans (hunk 1 left, hunk 1 right, hunk 2 right), got %d: %+v", len(fc.spans), fc.spans)
	}
}

func TestGoGitWorktreeStatus(t *testing.T) {
	dir := testRepo(t)
	testCommit(t, dir, "tracked.txt", "line1\n")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("line1\nline2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("newfile\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}

	client, err := newGitClient(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := client.repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	status, err := w.Status()
	if err != nil {
		t.Fatal(err)
	}
	for path, fs := range status {
		t.Logf("path: %s, staging: %c, worktree: %c", path, fs.Staging, fs.Worktree)
	}
	if _, ok := status["ignored.txt"]; ok {
		t.Fatal("ignored.txt should be excluded by .gitignore")
	}
	if _, ok := status["untracked.txt"]; !ok {
		t.Fatal("untracked.txt should be present in status")
	}
	if _, ok := status["tracked.txt"]; !ok {
		t.Fatal("tracked.txt should be present in status")
	}
}
