package di

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/cli"
	"github.com/diffpal/lintpal/internal/apps/lintpal/packs"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

func TestLintFxCommittedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	gitCommand := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = dir
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitCommand("init", "-q")
	gitCommand("config", "user.name", "Test")
	gitCommand("config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCommand("add", "example.go")
	gitCommand("commit", "-qm", "base")
	base := gitCommand("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package example\nvar X=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCommand("add", "example.go")
	gitCommand("commit", "-qm", "head")
	head := gitCommand("rev-parse", "HEAD")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		var request struct {
			Model     string                     `json:"model"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		answers := make(map[string]any, len(request.Questions))
		for id := range request.Questions {
			answers[id] = map[string]any{"type": "noul", "noul": .99}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"model": request.Model, "answers": answers, "usage": map[string]int{"input_tokens": 3, "output_tokens": 1}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	t.Setenv("LINTPAL_TOKEN", "test-token")
	opts := cli.Options{Base: base, Head: head, Provider: "custom", Model: "jev-latest", BaseURL: server.URL, AuthTokenEnv: "LINTPAL_TOKEN"}
	artifact, err := Lint(t.Context(), dir, opts)
	if err != nil || len(artifact.Diagnostics) != 2 || artifact.Stats.InputTokens != 3 {
		t.Fatalf("Fx lint: %+v, %v", artifact, err)
	}
}

func TestMarkdownFrontmatterPolicyPrecedence(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	ruleRoot := filepath.Join(root, "rules")
	if err := os.Mkdir(ruleRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ruleRoot, "rule.md"), []byte("---\nseverity: high\nthreshold: 0.8\ntitle: Check cleanup\n---\nChanged code must clean up.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw := cli.RawOptions{Base: "a", Head: "b", Rules: ruleRoot,
		Changed: map[string]bool{"base": true, "head": true, "rules": true}}
	options, err := cli.Resolve(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := loadPack(t.Context(), root, options)
	if err != nil || pack.Rules()[0].Severity != rules.High || pack.Rules()[0].Threshold != .8 || pack.Rules()[0].Title != "Check cleanup" {
		t.Fatalf("frontmatter policy: %+v, %v", pack.Rules(), err)
	}
	raw.RuleSeverity, raw.RuleThreshold = "critical", "0.6"
	raw.Changed["rule-severity"], raw.Changed["rule-threshold"] = true, true
	options, err = cli.Resolve(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	pack, err = loadPack(t.Context(), root, options)
	if err != nil || pack.Rules()[0].Severity != rules.Critical || pack.Rules()[0].Threshold != .6 || pack.Rules()[0].Title != "Check cleanup" {
		t.Fatalf("CLI policy: %+v, %v", pack.Rules(), err)
	}
}

func TestManagedPathCannotBypassLockThroughSymlink(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "rules.yaml"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".lintpal"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".lintpal", "packs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	path := filepath.Join(root, ".lintpal", "packs", "rules.yaml")
	if _, err := loadPack(t.Context(), root, cli.Options{Rules: path}); !errors.Is(err, packs.ErrDrift) {
		t.Fatalf("managed path bypassed lock: %v", err)
	}
}
