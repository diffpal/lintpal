package ruleimport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportLocalCollisionForceAndPrefix(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeFile(t, filepath.Join(source, "go", "errors.md"), "Handle new errors.\n")
	current := filepath.Join(root, ".lintpal", "rules", "go", "errors.md")
	writeFile(t, current, "Handle old errors.\n")
	notes := filepath.Join(root, ".lintpal", "rules", "README.txt")
	writeFile(t, notes, "keep me\n")
	if _, err := Import(t.Context(), root, source, "", false); !errors.Is(err, ErrConflict) {
		t.Fatalf("collision: %v", err)
	}
	if got, err := os.ReadFile(current); err != nil || string(got) != "Handle old errors.\n" {
		t.Fatalf("collision changed rule: %q, %v", got, err)
	}
	result, err := Import(t.Context(), root, source, "", true)
	if err != nil || len(result.Overwritten) != 1 || result.Overwritten[0] != "go/errors.md" {
		t.Fatalf("force import: %+v, %v", result, err)
	}
	if got, err := os.ReadFile(current); err != nil || string(got) != "Handle new errors.\n" {
		t.Fatalf("force did not replace rule: %q, %v", got, err)
	}
	result, err = Import(t.Context(), root, source, "", true)
	if err != nil || len(result.Unchanged) != 1 || result.Unchanged[0] != "go/errors.md" {
		t.Fatalf("identical force import: %+v, %v", result, err)
	}
	if got, err := os.ReadFile(notes); err != nil || string(got) != "keep me\n" {
		t.Fatalf("force changed unrelated file: %q, %v", got, err)
	}
	result, err = Import(t.Context(), root, source, "vendor", false)
	if err != nil || len(result.Created) != 1 || result.Created[0] != "vendor/go/errors.md" {
		t.Fatalf("prefix import: %+v, %v", result, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, ".lintpal", "rules", "vendor", "go", "errors.md")); err != nil || string(got) != "Handle new errors.\n" {
		t.Fatalf("prefixed rule: %q, %v", got, err)
	}
	if _, err := Import(t.Context(), root, source, "../escape", false); err == nil {
		t.Fatal("accepted unsafe prefix")
	}
}

func TestImportInvalidSourceLeavesDestinationUntouched(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeFile(t, filepath.Join(source, "invalid.md"), "---\nseverity: extreme\n---\nBad rule.\n")
	current := filepath.Join(root, ".lintpal", "rules", "existing.md")
	writeFile(t, current, "Keep existing.\n")
	if _, err := Import(t.Context(), root, source, "", false); err == nil {
		t.Fatal("accepted invalid source")
	}
	if got, err := os.ReadFile(current); err != nil || string(got) != "Keep existing.\n" {
		t.Fatalf("failed import changed destination: %q, %v", got, err)
	}
}

func TestSwapRuleTreeRestoresPriorTreeOnWriteFailure(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "rules")
	stage := filepath.Join(base, "stage")
	writeFile(t, filepath.Join(root, "old.md"), "Old.\n")
	writeFile(t, filepath.Join(stage, "new.md"), "New.\n")
	calls := 0
	rename := func(from, to string) error {
		calls++
		if calls == 2 {
			return errors.New("injected failure")
		}
		return os.Rename(from, to)
	}
	if err := swapRuleTree(t.Context(), root, stage, true, rename); !errors.Is(err, ErrStorage) {
		t.Fatalf("failed swap: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "old.md")); err != nil || string(got) != "Old.\n" {
		t.Fatalf("rollback lost old rule: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "new.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed swap installed new rule: %v", err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestImportRejectsSymlinkedRoot(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeFile(t, filepath.Join(source, "rule.md"), "Use the rule.\n")
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".lintpal"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".lintpal", "rules")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Import(t.Context(), root, source, "", false); !errors.Is(err, ErrStorage) {
		t.Fatalf("symlink root: %v", err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("symlink target changed: %v, %v", entries, err)
	}
}

func TestImportCombinedLimitLeavesDestinationUntouched(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	large := strings.Repeat("x", 7<<10)
	writeFile(t, filepath.Join(source, "new.md"), large)
	current := filepath.Join(root, ".lintpal", "rules", "old-00.md")
	for i := 0; i < 36; i++ {
		writeFile(t, filepath.Join(root, ".lintpal", "rules", fmt.Sprintf("old-%02d.md", i)), large)
	}
	if _, err := Import(t.Context(), root, source, "", false); err == nil {
		t.Fatal("combined catalog exceeded limit")
	}
	if got, err := os.ReadFile(current); err != nil || string(got) != large {
		t.Fatalf("limit failure changed destination: %d bytes, %v", len(got), err)
	}
}
