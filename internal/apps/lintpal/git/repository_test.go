package git

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestCompareUsesCommittedObjects(t *testing.T) {
	dir := testRepo(t)
	testCommit(t, dir, "edit.txt", "old\n")
	base := testCommit(t, dir, "gone.txt", "deleted\n")
	if err := os.WriteFile(filepath.Join(dir, "edit.txt"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "rm", "-q", "gone.txt")
	if err := os.WriteFile(filepath.Join(dir, "added.txt"), []byte("added\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-qm", "change files")
	head := runTestGit(t, dir, "rev-parse", "HEAD")
	repo, err := NewRepository(dir, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.Compare(t.Context(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 4 {
		t.Fatalf("items = %+v; skips = %+v", first.Items, first.Skips)
	}
	sources := make(map[Side][]byte)
	for _, item := range first.Items {
		if item.ID == "" || item.StartLine != 1 || item.EndLine != 1 {
			t.Fatalf("invalid item = %+v", item)
		}
		source, err := first.Source(item)
		if err != nil {
			t.Fatal(err)
		}
		if item.Path == "edit.txt" {
			sources[item.Side] = source
		}
		if item.Path == "gone.txt" && (item.Side != Left || !bytes.Equal(source, []byte("deleted\n"))) {
			t.Fatalf("deleted LEFT source = %q, item = %+v", source, item)
		}
	}
	if !bytes.Equal(sources[Left], []byte("old\n")) || !bytes.Equal(sources[Right], []byte("new\n")) {
		t.Fatalf("edit sources: LEFT=%q RIGHT=%q", sources[Left], sources[Right])
	}
	if err := os.WriteFile(filepath.Join(dir, "edit.txt"), []byte("dirty worktree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gone.txt"), []byte("untracked replacement\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := repo.Compare(t.Context(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Items, second.Items) || !reflect.DeepEqual(first.sources, second.sources) {
		t.Fatalf("working tree changed committed result\nfirst=%+v\nsecond=%+v", first, second)
	}
	copySource, err := first.Source(first.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	copySource[0] = 'X'
	again, _ := first.Source(first.Items[0])
	if again[0] == 'X' {
		t.Fatal("Source leaked mutable result backing bytes")
	}
}

func TestCompareLimitsReturnNoPartialResult(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "a.txt", "old\n")
	head := testCommit(t, dir, "a.txt", "new\n")
	for _, limits := range []Limits{{MaxPatchBytes: 10}, {MaxBlobBytes: 2}, {MaxItems: 1}} {
		repo, err := NewRepository(dir, limits)
		if err != nil {
			t.Fatal(err)
		}
		result, err := repo.Compare(t.Context(), base, head)
		if !errors.Is(err, ErrLimit) || len(result.Items) != 0 {
			t.Fatalf("limits %+v: result=%+v error=%v", limits, result, err)
		}
	}
}

func TestCompareRenameOnlyHasNoAnchor(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "old name.txt", "same content\nsecond line\n")
	runTestGit(t, dir, "mv", "old name.txt", "new name.txt")
	runTestGit(t, dir, "commit", "-qm", "rename only")
	head := runTestGit(t, dir, "rev-parse", "HEAD")
	repo, err := NewRepository(dir, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.Compare(t.Context(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 0 || len(result.Skips) != 1 || result.Skips[0].Reason != SkipNoLines {
		t.Fatalf("rename-only result = %+v", result)
	}
}

func TestCompareUnusualCommittedPath(t *testing.T) {
	dir := testRepo(t)
	name := "odd\nname:part.txt"
	if runtime.GOOS == "windows" {
		name = "odd name part.txt"
	}
	base := testCommit(t, dir, name, "before\n")
	head := testCommit(t, dir, name, "after\n")
	repo, err := NewRepository(dir, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.Compare(t.Context(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %+v", result.Items)
	}
	for _, item := range result.Items {
		if item.Path != name {
			t.Fatalf("path = %q, want %q", item.Path, name)
		}
		if _, err := result.Source(item); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompareUncommitted(t *testing.T) {
	dir := testRepo(t)
	testCommit(t, dir, ".gitignore", "*.tmp\n")
	testCommit(t, dir, "committed.txt", "line1\nline2\n")

	// 1. Unstaged modification to tracked file
	if err := os.WriteFile(filepath.Join(dir, "committed.txt"), []byte("line1\nline2 modified\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// 2. Staged new file
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", "staged.txt")

	// 3. Untracked regular file
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("untracked line 1\nuntracked line 2\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// 4. Untracked empty file
	if err := os.WriteFile(filepath.Join(dir, "empty.txt"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	// 5. Untracked binary file
	if err := os.WriteFile(filepath.Join(dir, "binary.bin"), []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}

	// 6. Ignored file (should not appear)
	if err := os.WriteFile(filepath.Join(dir, "scratch.tmp"), []byte("temporary\n"), 0600); err != nil {
		t.Fatal(err)
	}

	repo, err := NewRepository(dir, Limits{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := repo.CompareUncommitted(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if result.Revisions.Head != "UNCOMMITTED" {
		t.Fatalf("head = %q, want UNCOMMITTED", result.Revisions.Head)
	}

	itemMap := make(map[string][]WorkItem)
	for _, item := range result.Items {
		itemMap[item.Path] = append(itemMap[item.Path], item)
		source, err := result.Source(item)
		if err != nil {
			t.Fatalf("Source(%+v): %v", item, err)
		}
		if len(source) == 0 {
			t.Fatalf("empty source for %+v", item)
		}
	}

	if len(itemMap["committed.txt"]) == 0 {
		t.Fatal("missing work items for committed.txt")
	}
	if len(itemMap["staged.txt"]) == 0 {
		t.Fatal("missing work items for staged.txt")
	}
	if len(itemMap["untracked.txt"]) == 0 {
		t.Fatal("missing work items for untracked.txt")
	}
	if len(itemMap["scratch.tmp"]) != 0 {
		t.Fatal("ignored scratch.tmp was included in work items")
	}

	// Verify untracked file work item
	untrackedItems := itemMap["untracked.txt"]
	if len(untrackedItems) != 1 || untrackedItems[0].Side != Right || untrackedItems[0].StartLine != 1 || untrackedItems[0].EndLine != 2 {
		t.Fatalf("unexpected untracked items: %+v", untrackedItems)
	}
	untrackedSource, _ := result.Source(untrackedItems[0])
	if string(untrackedSource) != "untracked line 1\nuntracked line 2\n" {
		t.Fatalf("untracked source = %q", string(untrackedSource))
	}

	// Verify skips
	skipMap := make(map[string]SkipReason)
	for _, skip := range result.Skips {
		path := skip.NewPath
		if path == "" {
			path = skip.OldPath
		}
		skipMap[path] = skip.Reason
	}
	if skipMap["empty.txt"] != SkipNoLines {
		t.Fatalf("empty.txt skip = %v, want %v", skipMap["empty.txt"], SkipNoLines)
	}
	if skipMap["binary.bin"] != SkipBinary {
		t.Fatalf("binary.bin skip = %v, want %v", skipMap["binary.bin"], SkipBinary)
	}
}
