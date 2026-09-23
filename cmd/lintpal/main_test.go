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
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProcessExitAndStreams(t *testing.T) {
	binary := buildBinary(t)
	dir, base, head := committedRepo(t)
	server, calls := modelServer(t, http.StatusOK, false, nil)
	defer server.Close()
	common := []string{"lint", "--base", base, "--head", head, "--provider", "custom", "--base-url", server.URL, "--format", "json"}
	stdout, stderr, code := runBinary(t, binary, dir, append(append([]string{}, common...), "--fail-on", "none"))
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"schema_version": "lintpal.report.v1"`) || !strings.Contains(stdout, `"diagnostics": [`) {
		t.Fatalf("success code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertGoldenReport(t, "report.json", stdout, base, head)
	metricsOut, metricsErr, metricsCode := runBinary(t, binary, dir, append(append([]string{}, common...), "--fail-on", "none", "--metrics"))
	if metricsCode != 0 || metricsOut != stdout ||
		!strings.Contains(metricsErr, "metric stage=compare status=ok count=1 duration_ms=") ||
		!strings.Contains(metricsErr, "metric stage=evaluate status=ok count=1 duration_ms=") ||
		!strings.Contains(metricsErr, "metric stage=write status=ok count=2 duration_ms=") ||
		strings.Contains(metricsErr, "secret-sentinel") || strings.Contains(metricsErr, "example.go") ||
		strings.Contains(metricsErr, server.URL) {
		t.Fatalf("metrics code=%d stdout=%q stderr=%q", metricsCode, metricsOut, metricsErr)
	}
	humanArgs := []string{"lint", "--base", base, "--head", head, "--provider", "custom", "--base-url", server.URL, "--fail-on", "none"}
	humanOut, humanErr, humanCode := runBinary(t, binary, dir, humanArgs)
	if humanCode != 0 || humanErr != "" || !strings.HasPrefix(humanOut, "lintpal lintpal.report.v1 ") || !strings.Contains(humanOut, "high RIGHT") || !strings.Contains(humanOut, "stats work_items=") {
		t.Fatalf("human code=%d stdout=%q stderr=%q", humanCode, humanOut, humanErr)
	}
	assertGoldenReport(t, "report.txt", humanOut, base, head)
	artifact := filepath.Join(dir, "report.json")
	stdout, stderr, code = runBinary(t, binary, dir, append(append([]string{}, common...), "--out", artifact))
	body, err := os.ReadFile(artifact)
	if err != nil || code != 10 || !bytes.Equal([]byte(stdout), body) || !strings.Contains(stderr, "severity gate") {
		t.Fatalf("gate code=%d artifact=%q stdout=%q stderr=%q err=%v", code, body, stdout, stderr, err)
	}
	before := calls.Load()
	stdout, stderr, code = runBinary(t, binary, dir, []string{"lint", "--base", base, "--head", head, "--provider", "custom", "--base-url", server.URL, "--fail-on", "bad"})
	if code != 2 || stdout != "" || calls.Load() != before || strings.Contains(stderr, "secret-sentinel") {
		t.Fatalf("invalid options code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = runBinary(t, binary, dir, []string{"unknown"})
	if code != 2 || stdout != "" {
		t.Fatalf("unknown command code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	before = calls.Load()
	stdout, stderr, code = runBinary(t, binary, dir, append(append([]string{}, common...), "--out", dir))
	if code != 2 || stdout != "" || calls.Load() != before {
		t.Fatalf("directory output called provider: code=%d stdout=%q", code, stdout)
	}
	badRules := filepath.Join(dir, "bad.rules.yaml")
	if err := os.WriteFile(badRules, []byte("schema: lintpal.rules.v1\nprovider: https://attacker.invalid\nrules: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before = calls.Load()
	stdout, stderr, code = runBinary(t, binary, dir, append(append([]string{}, common...), "--rules", badRules))
	if code != 2 || stdout != "" || calls.Load() != before || strings.Contains(stderr, "attacker.invalid") {
		t.Fatalf("rule trust code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = runBinary(t, binary, dir, append(append([]string{}, common...), "--out", filepath.Join(dir, "missing", "report.json")))
	if code != 4 || stdout != "" || !strings.Contains(stderr, "export") {
		t.Fatalf("export code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, aux := range [][]string{{"version"}, {"completion", "bash"}, {"doctor", "--provider", "custom", "--base-url", server.URL}} {
		before = calls.Load()
		stdout, stderr, code = runBinary(t, binary, dir, aux)
		if code != 0 || stdout == "" || stderr != "" || calls.Load() != before || strings.Contains(stdout, "secret-sentinel") {
			t.Fatalf("aux %v code=%d stdout=%q stderr=%q", aux, code, stdout, stderr)
		}
	}
	transient, _ := modelServer(t, http.StatusServiceUnavailable, false, nil)
	defer transient.Close()
	stdout, stderr, code = runBinary(t, binary, dir, []string{"lint", "--base", base, "--head", head, "--provider", "custom", "--base-url", transient.URL})
	if code != 3 || stdout != "" || !strings.Contains(stderr, "temporarily") {
		t.Fatalf("transient code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	invalid, _ := modelServer(t, http.StatusOK, true, nil)
	defer invalid.Close()
	stdout, stderr, code = runBinary(t, binary, dir, []string{"lint", "--base", base, "--head", head, "--provider", "custom", "--base-url", invalid.URL})
	if code != 5 || stdout != "" || !strings.Contains(stderr, "lint failed") {
		t.Fatalf("protocol code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

var itemHash = regexp.MustCompile(`\b[0-9a-f]{64}\b`)

func assertGoldenReport(t *testing.T, name, output, base, head string) {
	t.Helper()
	normalized := strings.ReplaceAll(output, base, "<base>")
	normalized = strings.ReplaceAll(normalized, head, "<head>")
	normalized = itemHash.ReplaceAllString(normalized, "<item>")
	path := filepath.Join("testdata", "golden", name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(normalized), 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if normalized != string(want) {
		t.Fatalf("%s differs from golden\nwant:\n%s\ngot:\n%s", name, want, normalized)
	}
}

func TestProcessInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT process test uses Unix signaling")
	}
	binary := buildBinary(t)
	dir, base, head := committedRepo(t)
	entered := make(chan struct{}, 1)
	server, _ := modelServer(t, http.StatusOK, false, entered)
	defer server.Close()
	command := exec.Command(binary, "lint", "--base", base, "--head", head, "--provider", "custom", "--base-url", server.URL)
	command.Dir = dir
	command.Env = testEnv()
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		command.Process.Kill()
		t.Fatal("provider call did not start")
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	err := command.Wait()
	if code := processCode(err); code != 130 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "interrupted") {
		t.Fatalf("interrupt code=%d stdout=%q stderr=%q err=%v", processCode(err), stdout.String(), stderr.String(), err)
	}
}

func TestProcessHostileInputsStayLocalAndSecretFree(t *testing.T) {
	binary := buildBinary(t)
	dir, base, _ := committedRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package example\nvar Secret = \"source-sentinel\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "example.go"}, {"commit", "-qm", "source sentinel"}} {
		command := exec.Command("git", args...)
		command.Dir = dir
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = dir
	headBytes, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headBytes))
	server, calls := modelServer(t, http.StatusOK, false, nil)
	defer server.Close()
	pack := "schema: lintpal.rules.v1\nrules:\n  - id: demo.secret\n    type: noul\n    instructions: question-sentinel\n    threshold: 0.9\n    severity: high\n    title: Safe title\n    message: Safe message\n"
	rulesPath := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(rulesPath, []byte(pack), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"lint", "--base", base, "--head", head, "--provider", "custom", "--base-url", server.URL,
		"--rules", rulesPath, "--format", "json", "--fail-on", "none", "--metrics"}
	stdout, stderr, code := runBinary(t, binary, dir, args)
	if code != 0 || calls.Load() != 1 || !strings.Contains(stdout, `"rule_id": "demo.secret"`) ||
		!strings.Contains(stderr, "metric stage=write") {
		t.Fatalf("safe run code=%d calls=%d stdout=%q stderr=%q", code, calls.Load(), stdout, stderr)
	}
	for _, secret := range []string{"source-sentinel", "question-sentinel", "secret-sentinel", server.URL} {
		if strings.Contains(stdout+stderr, secret) {
			t.Fatalf("local output contains %q", secret)
		}
	}

	if err := os.WriteFile(rulesPath, []byte(strings.Replace(pack, "title: Safe title", "title: secret-sentinel", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "blocked.json")
	blockedArgs := append(append([]string{}, args...), "--out", artifact)
	stdout, stderr, code = runBinary(t, binary, dir, blockedArgs)
	if code != 4 || stdout != "" || !strings.Contains(stderr, "export") || strings.Contains(stderr, "secret-sentinel") {
		t.Fatalf("credential guard code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("unsafe artifact exists: %v", err)
	}

	captureCalls := new(atomic.Int32)
	capture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captureCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer capture.Close()
	hostile := "schema: lintpal.rules.v1\nprovider_url: " + capture.URL + "\ntoken_env: TYPESAFE_API_KEY\nrules: []\n"
	if err := os.WriteFile(rulesPath, []byte(hostile), 0600); err != nil {
		t.Fatal(err)
	}
	hostileCommand := exec.Command(binary, "lint", "--base", base, "--head", head, "--provider", "jev", "--rules", rulesPath)
	hostileCommand.Dir = dir
	hostileCommand.Env = append(testEnv(), "TYPESAFE_API_KEY=preset-secret-sentinel")
	var hostileOut, hostileErr bytes.Buffer
	hostileCommand.Stdout, hostileCommand.Stderr = &hostileOut, &hostileErr
	hostileCode := processCode(hostileCommand.Run())
	if hostileCode != 2 || hostileOut.Len() != 0 || captureCalls.Load() != 0 ||
		strings.Contains(hostileErr.String(), capture.URL) || strings.Contains(hostileErr.String(), "preset-secret-sentinel") {
		t.Fatalf("hostile rule code=%d calls=%d stderr=%q", hostileCode, captureCalls.Load(), hostileErr.String())
	}

	badBody := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "provider-body-sentinel source-sentinel question-sentinel secret-sentinel")
	}))
	defer badBody.Close()
	if err := os.WriteFile(rulesPath, []byte(pack), 0600); err != nil {
		t.Fatal(err)
	}
	badArgs := append([]string{}, args...)
	badArgs[8] = badBody.URL
	stdout, stderr, code = runBinary(t, binary, dir, badArgs)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "invalid lint input") {
		t.Fatalf("provider body code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, secret := range []string{"provider-body-sentinel", "source-sentinel", "question-sentinel", "secret-sentinel"} {
		if strings.Contains(stderr, secret) {
			t.Fatalf("provider error leaked %q", secret)
		}
	}
}

func TestProcessOversizedCommittedBlobIsAtomic(t *testing.T) {
	binary := buildBinary(t)
	dir, base, _ := committedRepo(t)
	large := "package example\n" + strings.Repeat("var X = 0\n", 230000)
	if err := os.WriteFile(filepath.Join(dir, "large.go"), []byte(large), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "large.go"}, {"commit", "-qm", "large committed blob"}} {
		command := exec.Command("git", args...)
		command.Dir = dir
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = dir
	headBytes, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	server, calls := modelServer(t, http.StatusOK, false, nil)
	defer server.Close()
	stdout, stderr, code := runBinary(t, binary, dir, []string{"lint", "--base", base, "--head", strings.TrimSpace(string(headBytes)),
		"--provider", "custom", "--base-url", server.URL, "--metrics"})
	if code != 2 || stdout != "" || calls.Load() != 0 || !strings.Contains(stderr, "invalid lint input") ||
		strings.Contains(stderr, "large.go") || strings.Contains(stderr, "var X") {
		t.Fatalf("oversized blob code=%d calls=%d stdout=%q stderr=%q", code, calls.Load(), stdout, stderr)
	}
}

func TestOptionalLiveEvalUsesPinnedLocalProvider(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("optional evaluation helper uses sha256sum")
	}
	for _, program := range []string{"jq", "python3", "sha256sum"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Skipf("optional evaluation helper requires %s", program)
		}
	}
	binary := buildBinary(t)
	server, calls := modelServer(t, http.StatusOK, false, nil)
	defer server.Close()
	output := filepath.Join(t.TempDir(), "live.json")
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "eval-live.sh"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(script)
	command.Env = append(testEnv(), "LINTPAL_EVAL_BIN="+binary, "LINTPAL_EVAL_PROVIDER=custom",
		"LINTPAL_EVAL_BASE_URL="+server.URL, "LINTPAL_EVAL_MODEL=jev-1.13.0", "LINTPAL_EVAL_OUTPUT="+output,
		"LINTPAL_EVAL_INPUT_USD_PER_MILLION=1", "LINTPAL_EVAL_OUTPUT_USD_PER_MILLION=2",
		"LINTPAL_RULES=/missing-rules.yaml", "LINTPAL_OUT=/unexpected-report.json")
	combined, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("local evaluation: %v %s", err, combined)
	}
	var manifest struct {
		SchemaVersion string `json:"schema_version"`
		BinarySHA     string `json:"binary_sha256"`
		CorpusSHA     string `json:"corpus_sha256"`
		Provider      string `json:"provider"`
		Model         string `json:"model"`
		Cases         []struct {
			ID        string `json:"id"`
			Predicted bool   `json:"predicted"`
		} `json:"cases"`
		TruePositive  int      `json:"true_positive"`
		FalsePositive int      `json:"false_positive"`
		InputTokens   int      `json:"input_tokens"`
		OutputTokens  int      `json:"output_tokens"`
		ElapsedMS     int      `json:"elapsed_ms"`
		CostUSD       *float64 `json:"cost_usd"`
	}
	data, err := os.ReadFile(output)
	if err != nil || json.Unmarshal(data, &manifest) != nil {
		t.Fatalf("manifest: %v %q", err, data)
	}
	if calls.Load() != 16 || manifest.SchemaVersion != "lintpal.eval.live.v1" ||
		len(manifest.BinarySHA) != 64 || len(manifest.CorpusSHA) != 64 ||
		manifest.Provider != "custom" || manifest.Model != "jev-1.13.0" ||
		len(manifest.Cases) != 8 || manifest.TruePositive != 4 || manifest.FalsePositive != 4 ||
		manifest.InputTokens != 16 || manifest.OutputTokens != 16 || manifest.ElapsedMS < 1 ||
		manifest.CostUSD == nil || *manifest.CostUSD < 0.0000479 || *manifest.CostUSD > 0.0000481 ||
		strings.Contains(string(data), "secret-sentinel") || strings.Contains(string(data), "userCommand") ||
		strings.Contains(string(data), server.URL) {
		t.Fatalf("unsafe or incomplete live manifest: calls=%d manifest=%q", calls.Load(), data)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	name := "lintpal"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", path, ".")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	return path
}

func committedRepo(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) string {
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
	git("init", "-q")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "example.go")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("package example\nvar X=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "example.go")
	git("commit", "-qm", "head")
	return dir, base, git("rev-parse", "HEAD")
}

func modelServer(t *testing.T, status int, malformed bool, entered chan<- struct{}) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	calls := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer secret-sentinel" {
			t.Errorf("wrong provider request")
		}
		if entered != nil {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case entered <- struct{}{}:
			default:
			}
			<-r.Context().Done()
			return
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		if malformed {
			io.WriteString(w, "not-json")
			return
		}
		var request struct {
			Model     string                     `json:"model"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		answers := map[string]any{}
		for id := range request.Questions {
			answers[id] = map[string]any{"type": "noul", "noul": .99}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"model": request.Model, "answers": answers, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}); err != nil {
			t.Error(err)
		}
	}))
	return server, calls
}

func runBinary(t *testing.T, binary, dir string, args []string) (string, string, int) {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Dir = dir
	command.Env = testEnv()
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), processCode(err)
}

func processCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	return -1
}

func testEnv() []string {
	env := make([]string, 0)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "LINTPAL_") || key == "TYPESAFE_API_KEY" || key == "OPENROUTER_API_KEY" {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "LINTPAL_TOKEN=secret-sentinel", "LINTPAL_PROVIDER=jev", "LINTPAL_BASE=invalid-env-base")
}
