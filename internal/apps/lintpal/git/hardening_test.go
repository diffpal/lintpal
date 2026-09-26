package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHostileDiffHelpersDoNotRun(t *testing.T) {
	dir := testRepo(t)
	marker := filepath.Join(t.TempDir(), "helper-ran")
	script := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf invoked > \""+marker+"\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "config", "diff.evil.command", script)
	runTestGit(t, dir, "config", "diff.evil.textconv", script)
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("*.txt diff=evil\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-qm", "base")
	base := runTestGit(t, dir, "rev-parse", "HEAD")
	head := testCommit(t, dir, "file.txt", "new\n")
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
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("external helper executed: stat error = %v", err)
	}
}

func TestBinarySkipAndCancellation(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "base.txt", "base\n")
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", "blob.bin")
	runTestGit(t, dir, "commit", "-qm", "binary")
	head := runTestGit(t, dir, "rev-parse", "HEAD")
	repo, err := NewRepository(dir, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.Compare(t.Context(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 0 || len(result.Skips) != 1 || result.Skips[0].Reason != SkipBinary {
		t.Fatalf("binary result = %+v", result)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err = repo.Compare(ctx, base, head)
	if !errors.Is(err, context.Canceled) || len(result.Items) != 0 {
		t.Fatalf("canceled result = %+v, err = %v", result, err)
	}
}

func TestTotalSourceAndHardLimits(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "a.txt", "old\n")
	head := testCommit(t, dir, "a.txt", "new\n")
	repo, err := NewRepository(dir, Limits{MaxTotalBlobBytes: 7})
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.Compare(t.Context(), base, head)
	if !errors.Is(err, ErrLimit) || len(result.Items) != 0 {
		t.Fatalf("total source limit: result=%+v error=%v", result, err)
	}
	for _, limits := range []Limits{{MaxPatchBytes: hardMaxPatchBytes + 1}, {MaxBlobBytes: hardMaxBlobBytes + 1}, {MaxTotalBlobBytes: hardMaxTotalBlobBytes + 1}, {MaxItems: hardMaxItems + 1}} {
		if _, err := NewRepository(dir, limits); !errors.Is(err, ErrInvalidLimits) {
			t.Fatalf("limits %+v error = %v", limits, err)
		}
	}
}

func TestZeroSubprocessWithEmptyPath(t *testing.T) {
	dir := testRepo(t)
	base := testCommit(t, dir, "a.txt", "one\n")
	head := testCommit(t, dir, "a.txt", "two\n")
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("draft\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Disable all subprocesses by emptying PATH
	t.Setenv("PATH", "")

	repo, err := NewRepository(dir, Limits{})
	if err != nil {
		t.Fatalf("NewRepository failed with empty PATH: %v", err)
	}

	// Compare committed
	result, err := repo.Compare(t.Context(), base, head)
	if err != nil {
		t.Fatalf("Compare failed with empty PATH: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	for _, item := range result.Items {
		src, err := result.Source(item)
		if err != nil || len(src) == 0 {
			t.Fatalf("failed to read source for item %+v with empty PATH: %v", item, err)
		}
	}

	// Compare uncommitted
	uncommittedResult, err := repo.CompareUncommitted(t.Context())
	if err != nil {
		t.Fatalf("CompareUncommitted failed with empty PATH: %v", err)
	}
	if len(uncommittedResult.Items) == 0 {
		t.Fatal("expected uncommitted items, got 0")
	}
}
