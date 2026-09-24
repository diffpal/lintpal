package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

func TestRuleCatalogCommands(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	nested := filepath.Join(root, "src")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	writeRule := func(id, body string) {
		t.Helper()
		path := filepath.Join(root, ".lintpal", "rules", filepath.FromSlash(id))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeRule("z/last.md", "Do the last thing.\n")
	writeRule("a/first.md", "---\nseverity: high\nthreshold: 0.8\ntitle: First requirement\n---\n\nHandle failures.\n")
	called := false
	run := func(args ...string) (string, error) {
		t.Helper()
		command := NewRoot(func(context.Context, Options, io.Writer, io.Writer) error { called = true; return nil }, "dev")
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(io.Discard)
		command.SetArgs(args)
		err := command.ExecuteContext(t.Context())
		return output.String(), err
	}
	if got, err := run("rule", "list"); err != nil || got != "a/first.md\nz/last.md\n" {
		t.Fatalf("list = %q, %v", got, err)
	}
	if got, err := run("rule", "view", "a/first.md"); err != nil ||
		!strings.Contains(got, "Severity: high\nThreshold: 0.8\n") ||
		!strings.Contains(got, "Title: First requirement\n") ||
		!strings.Contains(got, "\n\nHandle failures.\n") || strings.Contains(got, "---") {
		t.Fatalf("view = %q, %v", got, err)
	}
	if got, err := run("rule", "validate"); err != nil || got != "validated 2 rule(s)\n" {
		t.Fatalf("validate = %q, %v", got, err)
	}
	for _, id := range []string{"missing.md", "../escape.md"} {
		if got, err := run("rule", "view", id); !errors.Is(err, ErrInvalidOptions) || got != "" {
			t.Fatalf("view %q = %q, %v", id, got, err)
		}
	}
	if called {
		t.Fatal("rule command invoked lint")
	}
	writeRule("bad.md", "---\nseverity: extreme\n---\nBad rule.\n")
	if got, err := run("rule", "validate"); !errors.Is(err, rules.ErrInvalidRule) || got != "" {
		t.Fatalf("invalid catalog = %q, %v", got, err)
	}
}

func TestRuleImportCommandUsesCatalogRoot(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "requirement.md"), []byte("Handle errors.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	command := NewRoot(nil, "dev")
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"rule", "import", source, "--prefix", "vendor"})
	if err := command.ExecuteContext(t.Context()); err != nil || output.String() != "created vendor/requirement.md\n" {
		t.Fatalf("import output: %q, %v", output.String(), err)
	}
	path := filepath.Join(root, ".lintpal", "rules", "vendor", "requirement.md")
	if body, err := os.ReadFile(path); err != nil || string(body) != "Handle errors.\n" {
		t.Fatalf("imported rule: %q, %v", body, err)
	}
}
