package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommittedPatchLimitsIncludeRawFallback(t *testing.T) {
	for _, path := range []string{"file.txt", "control\nfile.txt"} {
		for _, oversizedLine := range []bool{false, true} {
			t.Run(path+"/"+map[bool]string{false: "bytes", true: "line"}[oversizedLine], func(t *testing.T) {
				dir := testRepo(t)
				base := testCommit(t, dir, path, "old\n")
				content := "new\n"
				limits := Limits{MaxPatchBytes: 10}
				if oversizedLine {
					content = strings.Repeat("x", maxDiffLineBytes) + "\n"
					limits = Limits{}
				}
				head := testCommit(t, dir, path, content)
				repo, err := NewRepository(dir, limits)
				if err != nil {
					t.Fatal(err)
				}
				result, err := repo.Compare(t.Context(), base, head)
				if !errors.Is(err, ErrLimit) || len(result.Items) != 0 || len(result.Skips) != 0 {
					t.Fatalf("partial or unlimited result: %+v, %v", result, err)
				}
			})
		}
	}
}

func TestUncommittedPatchLimits(t *testing.T) {
	for _, kind := range []string{"modified", "deleted", "untracked", "staged"} {
		for _, oversizedLine := range []bool{false, true} {
			t.Run(kind+"/"+map[bool]string{false: "bytes", true: "line"}[oversizedLine], func(t *testing.T) {
				dir := testRepo(t)
				content := "new\n"
				limits := Limits{MaxPatchBytes: 10}
				if oversizedLine {
					content = strings.Repeat("x", maxDiffLineBytes) + "\n"
					limits = Limits{}
				}
				initial := "old\n"
				if kind == "deleted" {
					initial = content
				}
				testCommit(t, dir, "file.txt", initial)
				path := filepath.Join(dir, "file.txt")
				if kind == "untracked" || kind == "staged" {
					path = filepath.Join(dir, "new.txt")
				}
				if kind == "deleted" {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				if kind == "staged" {
					runTestGit(t, dir, "add", "new.txt")
				}
				repo, err := NewRepository(dir, limits)
				if err != nil {
					t.Fatal(err)
				}
				result, err := repo.CompareUncommitted(t.Context())
				if !errors.Is(err, ErrLimit) || len(result.Items) != 0 || len(result.Skips) != 0 {
					t.Fatalf("partial or unlimited result: %+v, %v", result, err)
				}
			})
		}
	}
}

func TestRawPatchBudgetIsSharedAcrossFiles(t *testing.T) {
	file := rawFile{status: 'A', oldMode: "000000", newMode: "100644", newPath: "a.txt"}
	measure := patchBudget{remaining: defaultOutputBytes}
	if _, err := measure.addRaw(file, nil, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	size := defaultOutputBytes - measure.remaining
	budget := patchBudget{remaining: size}
	if _, err := budget.addRaw(file, nil, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	file.newPath = "b.txt"
	if _, err := budget.addRaw(file, nil, []byte("new\n")); !errors.Is(err, ErrLimit) {
		t.Fatalf("aggregate budget: %v", err)
	}
}

func TestPatchPathAndLineBoundaries(t *testing.T) {
	for _, path := range []string{strings.Repeat("x", maxGitPathBytes), strings.Repeat("x", maxGitPathBytes+1)} {
		budget := patchBudget{remaining: defaultOutputBytes}
		_, err := budget.addRaw(rawFile{status: 'A', newMode: "100644", newPath: path}, nil, []byte("x\n"))
		if errors.Is(err, ErrLimit) != (len(path) > maxGitPathBytes) {
			t.Fatalf("path %d: %v", len(path), err)
		}
	}
	budget := patchBudget{remaining: defaultOutputBytes}
	if _, err := budget.Write([]byte(strings.Repeat("x", maxDiffLineBytes))); err != nil {
		t.Fatal(err)
	}
	if _, err := budget.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := budget.Write([]byte(strings.Repeat("x", maxDiffLineBytes))); err != nil {
		t.Fatal(err)
	}
	if _, err := budget.Write([]byte("x")); !errors.Is(err, ErrLimit) {
		t.Fatalf("split line limit: %v", err)
	}
}
