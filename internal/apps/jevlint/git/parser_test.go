package git

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseAdditionAndReplacement(t *testing.T) {
	raw := []byte(":000000 100644 0000000 abcdef0 A\x00new.txt\x00:100644 100644 abcdef0 fedcba0 M\x00edit.txt\x00")
	patch := []byte("diff --git a/new.txt b/new.txt\nnew file mode 100644\nindex 0000000..abcdef0\n--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1,2 @@\n+one\n+two\ndiff --git a/edit.txt b/edit.txt\nindex abcdef0..fedcba0 100644\n--- a/edit.txt\n+++ b/edit.txt\n@@ -2 +2 @@\n-old\n+new\n")
	files, err := parseRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := parsePatch(patch, files)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := changes[0].spans, []changedSpan{{side: Right, start: 1, end: 2, hunk: 1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("addition spans = %+v, want %+v", got, want)
	}
	if got, want := changes[1].spans, []changedSpan{{side: Left, start: 2, end: 2, hunk: 1}, {side: Right, start: 2, end: 2, hunk: 1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("replacement spans = %+v, want %+v", got, want)
	}
}

func TestParseMalformedAndTruncated(t *testing.T) {
	files, err := parseRaw([]byte(":100644 100644 aaaaaaa bbbbbbb M\x00f.txt\x00"))
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []string{
		"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1 +1 @@\n-old\n",
		"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1 +1 @@\n-old\n+new",
		"diff --git a/f.txt b/f.txt\n--- a/other.txt\n+++ b/f.txt\n@@ -1 +1 @@\n-old\n+new\n",
	} {
		if _, err := parsePatch([]byte(patch), files); !errors.Is(err, ErrMalformedDiff) {
			t.Fatalf("patch %q error = %v", patch, err)
		}
	}
	if _, err := parseRaw([]byte(":100644 100644 aaaaaaa bbbbbbb M\x00f.txt")); !errors.Is(err, ErrMalformedDiff) {
		t.Fatalf("truncated raw error = %v", err)
	}
	for _, raw := range []string{
		":100649 100644 aaaaaaa bbbbbbb M\x00f.txt\x00",
		":100644 100644 aaaaaaa bbbbbbb Rxx\x00f.txt\x00g.txt\x00",
	} {
		if _, err := parseRaw([]byte(raw)); !errors.Is(err, ErrMalformedDiff) {
			t.Fatalf("malformed raw %q error = %v", raw, err)
		}
	}
}

func TestDeletionMultipleHunksAndNoNewline(t *testing.T) {
	raw := []byte(":100644 000000 abcdef0 0000000 D\x00old.txt\x00")
	patch := []byte("diff --git a/old.txt b/old.txt\ndeleted file mode 100644\nindex abcdef0..0000000\n--- a/old.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-first\n@@ -4 +0,0 @@\n-last\n\\ No newline at end of file\n")
	files, err := parseRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := parsePatch(patch, files)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := changes[0].spans, []changedSpan{{side: Left, start: 1, end: 1, hunk: 1}, {side: Left, start: 4, end: 4, hunk: 2}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("deletion spans = %+v, want %+v", got, want)
	}
}

func TestRenameOnlyAndBinary(t *testing.T) {
	raw := []byte(":100644 100644 abcdef0 abcdef0 R100\x00old.txt\x00new.txt\x00:100644 100644 abcdef0 fedcba0 M\x00blob.bin\x00")
	patch := []byte("diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\ndiff --git a/blob.bin b/blob.bin\nindex abcdef0..fedcba0 100644\nBinary files a/blob.bin and b/blob.bin differ\n")
	files, err := parseRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := parsePatch(patch, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || len(changes[0].spans) != 0 || changes[0].binary || !changes[1].binary {
		t.Fatalf("rename and binary = %+v", changes)
	}
}

func TestParseRealDiffWithUnusualPathsAndRename(t *testing.T) {
	dir := testRepo(t)
	name := "a space\tname.txt"
	base := testCommit(t, dir, name, "one\ntwo\nthree\nfour\n")
	renamed := "new space\tname.txt"
	runTestGit(t, dir, "mv", name, renamed)
	if err := os.WriteFile(filepath.Join(dir, renamed), []byte("one\ntwo\nthree\nfive\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", renamed)
	runTestGit(t, dir, "commit", "-qm", "rename and edit")
	head := runTestGit(t, dir, "rev-parse", "HEAD")
	r := commandRunner{dir: dir}
	raw, err := r.run(t.Context(), defaultOutputBytes, "diff", "--raw", "-z", "--find-renames", "--no-ext-diff", "--no-textconv", "--no-color", base, head)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := r.run(t.Context(), defaultOutputBytes, "diff", "--patch", "--unified=0", "--find-renames", "--no-ext-diff", "--no-textconv", "--no-color", base, head)
	if err != nil {
		t.Fatal(err)
	}
	files, err := parseRaw(raw)
	if err != nil {
		t.Fatalf("parse raw: %v, %q", err, raw)
	}
	changes, err := parsePatch(patch, files)
	if err != nil {
		t.Fatalf("parse patch: %v, %q", err, patch)
	}
	if len(changes) != 1 || changes[0].file.oldPath != name || changes[0].file.newPath != renamed {
		t.Fatalf("rename changes = %+v", changes)
	}
	if got := changes[0].spans; len(got) != 2 || got[0].side != Left || got[1].side != Right {
		t.Fatalf("rename spans = %+v", got)
	}
}

func FuzzParsePatch(f *testing.F) {
	f.Add([]byte("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n-old\n+new\n"))
	f.Add([]byte("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -0,0 +1 @@\n+new\n"))
	f.Add([]byte("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ malformed @@\n+bad\n"))
	f.Add([]byte("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n+$(touch /tmp/jevlint-fuzz-sentinel)\n"))
	f.Add([]byte("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -0,0 +1 @@\n+" + strings.Repeat("x", maxDiffLineBytes+1) + "\n"))
	raw := []rawFile{{oldPath: "f", newPath: "f", oldMode: "100644", newMode: "100644", status: 'M'}}
	f.Fuzz(func(t *testing.T, patch []byte) {
		if len(patch) > 2<<20 {
			t.Skip()
		}
		changes, err := parsePatch(patch, raw)
		if err != nil && len(changes) != 0 {
			t.Fatal("partial parsed patch")
		}
	})
}
