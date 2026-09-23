package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func testRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runTestGit(t, dir, "init", "-q")
	runTestGit(t, dir, "config", "user.name", "Lintpal Test")
	runTestGit(t, dir, "config", "user.email", "test@example.invalid")
	return dir
}

func testCommit(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", name)
	runTestGit(t, dir, "commit", "-qm", name)
	return runTestGit(t, dir, "rev-parse", "HEAD")
}

func TestResolveCommittedRange(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "a.txt", "one\n")
	head := testCommit(t, dir, "a.txt", "two\n")
	r := commandRunner{dir: dir}
	got, err := r.resolve(t.Context(), base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Revisions{Base: base, Head: head, MergeBase: base}) {
		t.Fatalf("resolved = %+v", got)
	}
	if _, err := r.resolve(t.Context(), "--bad", "HEAD"); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("invalid revision error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.resolve(ctx, base, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resolve error = %v", err)
	}
}

func TestUnrelatedHistories(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "a.txt", "one\n")
	runTestGit(t, dir, "checkout", "-q", "--orphan", "other")
	runTestGit(t, dir, "rm", "-qrf", ".")
	head := testCommit(t, dir, "b.txt", "two\n")
	_, err := (commandRunner{dir: dir}).resolve(t.Context(), base, head)
	if !errors.Is(err, ErrAmbiguousBase) {
		t.Fatalf("unrelated histories error = %v", err)
	}
}

func TestMultipleMergeBases(t *testing.T) {
	dir := testRepo(t)
	root := testCommit(t, dir, "root.txt", "root\n")
	a := testCommit(t, dir, "a.txt", "a\n")
	runTestGit(t, dir, "checkout", "-q", "-b", "other", root)
	b := testCommit(t, dir, "b.txt", "b\n")
	aTree := runTestGit(t, dir, "rev-parse", a+"^{tree}")
	bTree := runTestGit(t, dir, "rev-parse", b+"^{tree}")
	first := runTestGit(t, dir, "commit-tree", aTree, "-p", a, "-p", b, "-m", "first merge")
	second := runTestGit(t, dir, "commit-tree", bTree, "-p", b, "-p", a, "-m", "second merge")
	_, err := (commandRunner{dir: dir}).resolve(t.Context(), first, second)
	if !errors.Is(err, ErrAmbiguousBase) {
		t.Fatalf("multiple bases error = %v", err)
	}
}

func TestBoundedCommandOutput(t *testing.T) {
	dir := testRepo(t)
	_, err := (commandRunner{dir: dir}).run(t.Context(), 2, "version")
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("bounded run error = %v", err)
	}
}
