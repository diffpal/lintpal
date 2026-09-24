package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyAbsentAndInvalidLock(t *testing.T) {
	root := t.TempDir()
	count, err := verify(root)
	if err != nil || count != 0 {
		t.Fatalf("absent lock: count=%d err=%v", count, err)
	}
	if err := os.Mkdir(filepath.Join(root, ".lintpal"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lintpal", "packs.lock.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := verify(root); err == nil {
		t.Fatal("invalid lock accepted")
	}
}
