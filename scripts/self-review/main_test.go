package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMissingRefsDoNotBuildOrCreateArtifact(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := run(root, root, func(string) string { return "" }, &bytes.Buffer{}, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "BASE and HEAD") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".artifacts")); !os.IsNotExist(err) {
		t.Fatalf("artifact created before input validation: %v", err)
	}
}

func TestSelfReviewSmoke(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("config", "user.name", "Smoke")
	git("config", "user.email", "smoke@example.invalid")
	path := filepath.Join(root, "example.go")
	if err := os.WriteFile(path, []byte("package example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "example.go")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(path, []byte("package example\nvar X=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "example.go")
	git("commit", "-qm", "head")
	head := git("rev-parse", "HEAD")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer smoke-secret" {
			t.Errorf("unexpected provider request")
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
			answers[id] = map[string]any{"type": "noul", "noul": 0.99}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"model": request.Model, "answers": answers,
			"usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	t.Setenv("LINTPAL_BASE_URL", server.URL)
	t.Setenv("LINTPAL_TOKEN", "smoke-secret")
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	getenv := func(gate string) func(string) string {
		return func(name string) string {
			return map[string]string{"BASE": base, "HEAD": head, "PROVIDER": "custom", "FAIL_ON": gate}[name]
		}
	}
	for _, tc := range []struct {
		gate string
		code int
	}{
		{"none", 0},
		{"medium", 10},
	} {
		var stdout, stderr bytes.Buffer
		code := run(sourceRoot, root, getenv(tc.gate), &stdout, &stderr)
		reportPath := filepath.Join(root, ".artifacts", "lintpal", "self-review.json")
		report, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Version  string            `json:"version"`
			Findings []json.RawMessage `json:"findings"`
		}
		if err := json.Unmarshal(report, &decoded); err != nil {
			t.Fatal(err)
		}
		if code != tc.code || decoded.Version != "v5" || len(decoded.Findings) == 0 ||
			!bytes.Equal(report, stdout.Bytes()) || strings.Contains(stdout.String()+stderr.String(), "smoke-secret") {
			t.Fatalf("gate=%s code=%d report=%s stderr=%q", tc.gate, code, report, stderr.String())
		}
	}
	before := calls.Load()
	var stderr bytes.Buffer
	code := run(sourceRoot, root, func(string) string { return "" }, io.Discard, &stderr)
	if code != 2 || calls.Load() != before {
		t.Fatalf("missing refs reached provider: code=%d calls=%d", code, calls.Load())
	}
}
