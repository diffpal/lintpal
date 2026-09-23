package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
	reportpkg "github.com/diffpal/jevlint/internal/apps/jevlint/report"
	"github.com/diffpal/jevlint/internal/apps/jevlint/rules"
)

type lintProvider struct {
	mu                  sync.Mutex
	active, peak, calls int
	block               <-chan struct{}
	malformed           bool
}

type reorderProvider struct {
	mu         sync.Mutex
	secondDone chan struct{}
	completion []string
	failSecond bool
}

func (p *reorderProvider) Evaluate(ctx context.Context, request jev.Request) (jev.Response, error) {
	var id string
	for key := range request.Questions {
		id = key
	}
	if strings.HasSuffix(id, "/correctness.ignored-error") {
		select {
		case <-ctx.Done():
			return jev.Response{}, ctx.Err()
		case <-p.secondDone:
		}
		p.mu.Lock()
		p.completion = append(p.completion, "first")
		p.mu.Unlock()
	} else {
		p.mu.Lock()
		p.completion = append(p.completion, "second")
		p.mu.Unlock()
		close(p.secondDone)
		if p.failSecond {
			return jev.Response{}, errors.New("provider failed")
		}
	}
	return jev.Response{Model: request.Model, Answers: map[string]jev.Answer{id: jev.NoulAnswer{Probability: 1}}, Usage: jev.Usage{InputTokens: 1, OutputTokens: 1}}, nil
}

func (f *lintProvider) Evaluate(ctx context.Context, request jev.Request) (jev.Response, error) {
	f.mu.Lock()
	f.active++
	f.calls++
	if f.active > f.peak {
		f.peak = f.active
	}
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.active--; f.mu.Unlock() }()
	if f.block != nil {
		select {
		case <-ctx.Done():
			return jev.Response{}, ctx.Err()
		case <-f.block:
		}
	}
	answers := make(map[string]jev.Answer)
	for id := range request.Questions {
		answers[id] = jev.NoulAnswer{Probability: 1}
	}
	if f.malformed {
		answers = nil
	}
	return jev.Response{Model: request.Model, Answers: answers, Usage: jev.Usage{InputTokens: 2, OutputTokens: 3}}, nil
}

func TestLintCommittedPipeline(t *testing.T) {
	repo, dir, base, head := lintRepo(t)
	provider := &lintProvider{}
	linter, err := NewLinter(repo, provider, rules.BuiltIn())
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Base: base, Head: head, Model: "model", ProviderName: "systemone"}
	report, err := linter.Lint(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.BaseSHA != base || report.HeadSHA != head || len(report.Diagnostics) != 2 || report.Stats.InputTokens != 2 || report.Stats.OutputTokens != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
	// Working-tree edits cannot change committed-source findings.
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err := linter.Lint(context.Background(), request)
	if err != nil || len(again.Diagnostics) != len(report.Diagnostics) {
		t.Fatalf("working tree changed run: %v, %+v", err, again)
	}
	var firstJSON, secondJSON bytes.Buffer
	if err := reportpkg.WriteJSON(&firstJSON, report); err != nil {
		t.Fatal(err)
	}
	if err := reportpkg.WriteJSON(&secondJSON, again); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON.Bytes(), secondJSON.Bytes()) || bytes.Contains(firstJSON.Bytes(), []byte("package example")) {
		t.Fatal("report changed or leaked source")
	}
	provider.malformed = true
	partial, err := linter.Lint(context.Background(), request)
	if err == nil || partial.SchemaVersion != "" {
		t.Fatalf("malformed response returned report: %+v, %v", partial, err)
	}
}

func TestLintNoApplicableRules(t *testing.T) {
	repo, _, base, head := lintRepo(t)
	pack, err := rules.Load(strings.NewReader("schema: jevlint.rules.v1\nrules:\n  - id: docs-only\n    type: noul\n    instructions: Is this wrong?\n    threshold: 0.9\n    severity: high\n    title: Docs\n    message: Docs issue.\n    paths: ['*.md']\n"))
	if err != nil {
		t.Fatal(err)
	}
	provider := &lintProvider{}
	linter, err := NewLinter(repo, provider, pack)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := linter.Lint(context.Background(), Request{Base: base, Head: head, Model: "model", ProviderName: "systemone"})
	if err != nil || len(artifact.Diagnostics) != 0 || artifact.Stats.Questions != 0 || provider.calls != 0 {
		t.Fatalf("no-rule run: %+v %v calls=%d", artifact, err, provider.calls)
	}
}

func TestLintDeadlineAndConcurrency(t *testing.T) {
	repo, _, base, head := lintRepo(t)
	block := make(chan struct{})
	provider := &lintProvider{block: block}
	linter, err := NewLinter(repo, provider, rules.BuiltIn())
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Base: base, Head: head, Model: "model", ProviderName: "systemone", Limits: Limits{Concurrency: 2, Timeout: time.Second}}
	request.Limits.Context.MaxQuestionsPerBatch = 1
	partial, err := linter.Lint(context.Background(), request)
	if !errors.Is(err, context.DeadlineExceeded) || partial.SchemaVersion != "" {
		t.Fatalf("deadline: %+v, %v", partial, err)
	}
	provider.mu.Lock()
	peak := provider.peak
	provider.mu.Unlock()
	if peak < 1 || peak > 2 {
		t.Fatalf("concurrency peak: %d", peak)
	}
	close(block)
	request.Limits.Timeout = 0
	request.Limits.MaxReportBytes = 1
	partial, err = linter.Lint(context.Background(), request)
	if !errors.Is(err, ErrReportLimit) || partial.SchemaVersion != "" {
		t.Fatalf("report limit: %+v, %v", partial, err)
	}
}

func TestLintParentCancellation(t *testing.T) {
	repo, _, base, head := lintRepo(t)
	provider := &lintProvider{block: make(chan struct{})}
	linter, err := NewLinter(repo, provider, rules.BuiltIn())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	partial, err := linter.Lint(ctx, Request{Base: base, Head: head, Model: "model", ProviderName: "systemone"})
	if !errors.Is(err, context.Canceled) || partial.SchemaVersion != "" {
		t.Fatalf("cancellation: %+v, %v", partial, err)
	}
}

func TestLintOutOfOrderBatchDeterminism(t *testing.T) {
	repo, _, base, head := lintRepo(t)
	request := Request{Base: base, Head: head, Model: "model", ProviderName: "systemone", Limits: Limits{Concurrency: 2}}
	request.Limits.Context.MaxQuestionsPerBatch = 1
	var expected []byte
	for run := 0; run < 3; run++ {
		provider := &reorderProvider{secondDone: make(chan struct{})}
		linter, err := NewLinter(repo, provider, rules.BuiltIn())
		if err != nil {
			t.Fatal(err)
		}
		artifact, err := linter.Lint(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if artifact.Stats.Batches != 2 || artifact.Stats.InputTokens != 2 || artifact.Stats.OutputTokens != 2 {
			t.Fatalf("batch stats: %+v", artifact.Stats)
		}
		provider.mu.Lock()
		order := append([]string(nil), provider.completion...)
		provider.mu.Unlock()
		if len(order) != 2 || order[0] != "second" || order[1] != "first" {
			t.Fatalf("completion order: %v", order)
		}
		var output bytes.Buffer
		if err := reportpkg.WriteJSON(&output, artifact); err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			expected = append([]byte(nil), output.Bytes()...)
		} else if !bytes.Equal(expected, output.Bytes()) {
			t.Fatal("report depends on completion order")
		}
	}
	provider := &reorderProvider{secondDone: make(chan struct{}), failSecond: true}
	linter, err := NewLinter(repo, provider, rules.BuiltIn())
	if err != nil {
		t.Fatal(err)
	}
	partial, err := linter.Lint(context.Background(), request)
	if err == nil || partial.SchemaVersion != "" {
		t.Fatalf("sibling failure exposed report: %+v, %v", partial, err)
	}
}

func TestLintDeleteAndRename(t *testing.T) {
	repo, dir, _, head := lintRepo(t)
	provider := &lintProvider{}
	linter, err := NewLinter(repo, provider, rules.BuiltIn())
	if err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "rm", "example.go")
	gitInRepo(t, dir, "commit", "-qm", "delete")
	deletedHead := gitInRepo(t, dir, "rev-parse", "HEAD")
	deleted, err := linter.Lint(context.Background(), Request{Base: head, Head: deletedHead, Model: "model", ProviderName: "systemone"})
	if err != nil || len(deleted.Diagnostics) == 0 {
		t.Fatalf("delete report: %+v %v", deleted, err)
	}
	for _, d := range deleted.Diagnostics {
		if d.Side != git.Left || d.Path != "example.go" {
			t.Fatalf("delete anchor: %+v", d)
		}
	}
	var old strings.Builder
	old.WriteString("package example\n")
	for n := 0; n < 20; n++ {
		old.WriteString("var X = 1\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "old.go"), []byte(old.String()), 0600); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "old.go")
	gitInRepo(t, dir, "commit", "-qm", "old name")
	oldHead := gitInRepo(t, dir, "rev-parse", "HEAD")
	gitInRepo(t, dir, "mv", "old.go", "new.go")
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte(old.String()+"var Y = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "new.go")
	gitInRepo(t, dir, "commit", "-qm", "rename and edit")
	newHead := gitInRepo(t, dir, "rev-parse", "HEAD")
	if status := gitInRepo(t, dir, "diff", "--name-status", "--find-renames", oldHead, newHead); !strings.HasPrefix(status, "R") {
		t.Fatalf("expected rename: %q", status)
	}
	renamed, err := linter.Lint(context.Background(), Request{Base: oldHead, Head: newHead, Model: "model", ProviderName: "systemone"})
	if err != nil || len(renamed.Diagnostics) == 0 {
		t.Fatalf("rename report: %+v %v", renamed, err)
	}
	for _, d := range renamed.Diagnostics {
		if d.Side != git.Right || d.Path != "new.go" {
			t.Fatalf("rename anchor: %+v", d)
		}
	}
}

func lintRepo(t *testing.T) (*git.Repository, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	gitCommand := func(args ...string) string { return gitInRepo(t, dir, args...) }
	gitCommand("init", "-q")
	gitCommand("config", "user.name", "Test")
	gitCommand("config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCommand("add", "example.go")
	gitCommand("commit", "-qm", "base")
	base := gitCommand("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package example\nvar X = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCommand("add", "example.go")
	gitCommand("commit", "-qm", "head")
	head := gitCommand("rev-parse", "HEAD")
	repo, err := git.NewRepository(dir, git.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return repo, dir, base, head
}

func gitInRepo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
