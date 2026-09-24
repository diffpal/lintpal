package packs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validRules = "schema: lintpal.rules.v1\nrules:\n  - id: demo.one\n    type: noul\n    instructions: Is this wrong?\n    threshold: 0.9\n    severity: high\n    title: Demo\n    message: Demo issue.\n"

func writeLocalPack(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLocalImportVerifyAndUpdate(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeLocalPack(t, source, validRules)
	entry, err := ImportLocal(t.Context(), root, "demo", source, false)
	if err != nil || entry.SourceKind != "local" || entry.Source != "source" || entry.SHA256 == "" {
		t.Fatalf("import: %+v, %v", entry, err)
	}
	if _, err := Verify(t.Context(), root, "demo"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if pack, err := Load(t.Context(), root, "demo"); err != nil || len(pack.Rules()) != 1 {
		t.Fatalf("load: %v", err)
	}
	if _, err := ImportLocal(t.Context(), root, "demo", source, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate import: %v", err)
	}
	writeLocalPack(t, source, strings.Replace(validRules, "Demo issue.", "New issue.", 1))
	updated, err := ImportLocal(t.Context(), root, "demo", source, true)
	if err != nil || updated.SHA256 == entry.SHA256 {
		t.Fatalf("update: %+v, %v", updated, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".lintpal", filepath.FromSlash(entry.Path))); err != nil {
		t.Fatalf("previous content lost: %v", err)
	}
	writeLocalPack(t, source, "invalid yaml")
	if _, err := ImportLocal(t.Context(), root, "demo", source, true); err == nil {
		t.Fatal("invalid update accepted")
	}
	current, err := Get(root, "demo")
	if err != nil || current.SHA256 != updated.SHA256 {
		t.Fatalf("failed update changed lock: %+v, %v", current, err)
	}
}

func TestLockAndSourceTamperingFailClosed(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeLocalPack(t, source, validRules)
	for _, name := range []string{"../escape", "bad/name", "Bad"} {
		if _, err := ImportLocal(t.Context(), root, name, source, false); !errors.Is(err, ErrSource) {
			t.Fatalf("unsafe name %q: %v", name, err)
		}
	}
	entry, err := ImportLocal(t.Context(), root, "demo", source, false)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(root, ".lintpal", filepath.FromSlash(entry.Path))
	if err := os.WriteFile(copyPath, []byte(validRules+"# edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(t.Context(), root, "demo"); !errors.Is(err, ErrDrift) {
		t.Fatalf("edited copy: %v", err)
	}
	if _, err := Load(t.Context(), root, "demo"); !errors.Is(err, ErrDrift) {
		t.Fatalf("edited copy load: %v", err)
	}
	if _, err := ImportLocal(t.Context(), root, "demo", source, true); err != nil {
		t.Fatalf("explicit update did not repair drift: %v", err)
	}
	if _, err := Verify(t.Context(), root, "demo"); err != nil {
		t.Fatalf("repaired pack still invalid: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lintpal", "packs.lock.json"), []byte(`{"schema":"lintpal.packs.lock.v1","packs":[{"name":"../escape"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(t.Context(), root, "demo"); !errors.Is(err, ErrLock) {
		t.Fatalf("tampered lock: %v", err)
	}
}

func TestRejectSymlinkAndCancellation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeLocalPack(t, source, validRules)
	linked := filepath.Join(root, "linked")
	if err := os.Symlink(source, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportLocal(t.Context(), root, "demo", linked, false); !errors.Is(err, ErrSource) {
		t.Fatalf("linked directory: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ImportLocal(ctx, root, "demo", source, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".lintpal")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed import created storage: %v", err)
	}
}

func TestPublishedExamplePack(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cwd, "..", "..", "..", "..", "examples", "rules", "go-review")
	root := t.TempDir()
	entry, err := ImportLocalMarkdown(t.Context(), root, "go-review", path, false)
	if err != nil {
		t.Fatalf("published example does not import: %v", err)
	}
	if entry.SHA256 == "" {
		t.Fatal("published example has no content hash")
	}
	pack, err := LoadMarkdown(t.Context(), root, "go-review")
	if err != nil || len(pack.Rules()) != 3 {
		t.Fatalf("published example: %v", err)
	}
}

func TestImportThroughSymlinkedParent(t *testing.T) {
	real := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root := filepath.Join(alias, "repo")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source")
	writeLocalPack(t, source, validRules)
	if _, err := ImportLocal(t.Context(), root, "demo", source, false); err != nil {
		t.Fatalf("import through parent alias: %v", err)
	}
	if _, err := Verify(t.Context(), root, "demo"); err != nil {
		t.Fatalf("verify through parent alias: %v", err)
	}
}
