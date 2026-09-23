package contextplan

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
)

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func testResult(t *testing.T) (git.Result, string) {
	t.Helper()
	dir := t.TempDir()
	testGit(t, dir, "init", "-q")
	testGit(t, dir, "config", "user.email", "test@example.invalid")
	testGit(t, dir, "config", "user.name", "Test")
	before := make([]string, 20)
	after := make([]string, 20)
	for i := range before {
		before[i] = "line" + strconv.Itoa(i)
		after[i] = before[i]
	}
	after[1] = "changed-first"
	after[17] = "changed-second"
	path := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(path, []byte(strings.Join(before, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", "source.txt")
	testGit(t, dir, "commit", "-qm", "before")
	base := testGit(t, dir, "rev-parse", "HEAD")
	if err := os.WriteFile(path, []byte(strings.Join(after, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", "source.txt")
	testGit(t, dir, "commit", "-qm", "after")
	head := testGit(t, dir, "rev-parse", "HEAD")
	repo, err := git.NewRepository(dir, git.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.Compare(t.Context(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	return result, dir
}

func TestAssembleCommittedAndDeterministic(t *testing.T) {
	result, dir := testResult(t)
	first, err := Assemble(t.Context(), result, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(result.Items) != 4 {
		t.Fatalf("groups=%d items=%d", len(first), len(result.Items))
	}
	seen := map[string]int{}
	for _, group := range first {
		if group.ID == "" || group.State == "" {
			t.Fatal("empty group")
		}
		for _, item := range group.Items {
			seen[item.ID]++
		}
	}
	for _, item := range result.Items {
		if seen[item.ID] != 1 {
			t.Fatalf("item %s coverage=%d", item.ID, seen[item.ID])
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "source.txt"), []byte("dirty worktree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 30; trial++ {
		rng.Shuffle(len(result.Items), func(i, j int) { result.Items[i], result.Items[j] = result.Items[j], result.Items[i] })
		second, err := Assemble(t.Context(), result, Limits{})
		if err != nil || !reflect.DeepEqual(first, second) {
			t.Fatalf("not deterministic on trial %d: %v", trial, err)
		}
	}
}

func TestAssembleAtomicLimitsAndCancellation(t *testing.T) {
	result, _ := testResult(t)
	groups, err := Assemble(t.Context(), result, Limits{MaxStateBytes: 20})
	if !errors.Is(err, ErrLimit) || len(groups) != 0 {
		t.Fatalf("small state: %v, %+v", err, groups)
	}
	groups, err = Assemble(t.Context(), result, Limits{MaxGroups: 1, MaxStateBytes: 150})
	if !errors.Is(err, ErrLimit) || len(groups) != 0 {
		t.Fatalf("group cap: %v, %+v", err, groups)
	}
	groups, err = Assemble(t.Context(), result, Limits{MaxTotalStateBytes: 100})
	if !errors.Is(err, ErrLimit) || len(groups) != 0 {
		t.Fatalf("total state cap: %v, %+v", err, groups)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	groups, err = Assemble(ctx, result, Limits{})
	if !errors.Is(err, context.Canceled) || len(groups) != 0 {
		t.Fatalf("canceled: %v, %+v", err, groups)
	}
	result.Items[0].EndLine = 999
	groups, err = Assemble(t.Context(), result, Limits{})
	if !errors.Is(err, ErrInvalidInput) || len(groups) != 0 {
		t.Fatalf("invalid: %v, %+v", err, groups)
	}
}
