package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyRejectsUnformattedGo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main(){println(1)}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(root); err == nil {
		t.Fatal("unformatted Go file accepted")
	}
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() { println(1) }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(root); err != nil {
		t.Fatal(err)
	}
}
