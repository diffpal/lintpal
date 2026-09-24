package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRejectsBrokenLinkAndRuleExample(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "[docs](docs/guide.md)\n")
	write("CONTRIBUTING.md", "# Contributing\n")
	write("docs/guide.md", "[missing](missing.md)\n")
	write("examples/rules/sample/rule.md", " \n")
	if err := verify(root); err == nil || !strings.Contains(err.Error(), "missing.md") {
		t.Fatalf("want broken link error, got %v", err)
	}
	write("docs/guide.md", "# Guide\n")
	if err := verify(root); err == nil || !strings.Contains(err.Error(), "sample") {
		t.Fatalf("want invalid rule error, got %v", err)
	}
}

func TestVerifyRejectsBrokenContributorLink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Readme\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CONTRIBUTING.md"), []byte("[missing](missing.md)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "examples", "rules", "sample"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := verify(root); err == nil || !strings.Contains(err.Error(), "missing.md") {
		t.Fatalf("broken contributor link accepted: %v", err)
	}
}
