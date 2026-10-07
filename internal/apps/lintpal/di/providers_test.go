package di

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/cli"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	adkruntime "github.com/diffpal/lintpal/internal/apps/lintpal/runtime/adk"
)

type providerRoundTrip func(*http.Request) (*http.Response, error)

func (f providerRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConfiguredProviderDestinations(t *testing.T) {
	for _, tc := range []struct {
		options  cli.Options
		url      string
		response string
	}{
		{cli.Options{Provider: "openai"}, "https://api.openai.com/v1/decisions", `{"model":"test-model","answers":[{"type":"predicate","name":"check","probability":0.3}],"usage":{"input_tokens":1,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens":0}}`},
		{cli.Options{Provider: "custom", BaseURL: "https://example.test/api/", APIPath: "/decisions", AuthTokenEnv: "MY_KEY"}, "https://example.test/api/decisions", `{"model":"test-model","answers":{"check":{"type":"noul","noul":0.3}},"usage":{"input_tokens":1,"output_tokens":1}}`},
		{cli.Options{Provider: "custom", BaseURL: "https://example.test/api", AuthTokenEnv: "MY_KEY"}, "https://example.test/api/v1/systemone", `{"model":"test-model","answers":{"check":{"type":"noul","noul":0.3}},"usage":{"input_tokens":1,"output_tokens":1}}`},
	} {
		t.Run(tc.options.Provider+tc.options.APIPath, func(t *testing.T) {
			tc.options.Credential = "bound-secret"
			tc.options.CredentialResolved = true
			calls := 0
			client := &http.Client{Transport: providerRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != tc.url || r.Header.Get("Authorization") != "Bearer bound-secret" {
					t.Error("wrong destination or bound credential")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.response))}, nil
			})}
			provider, err := newProvider(tc.options, client)
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Evaluate(t.Context(), jev.Request{Model: "test-model", State: "state", Questions: map[string]jev.Question{"check": jev.NoulQuestion{Instructions: "Check state"}}})
			if err != nil || calls != 1 {
				t.Fatalf("calls = %d, error = %v", calls, err)
			}
		})
	}
}

func TestOpenAIEndToEndReviewMetrics(t *testing.T) {
	dir, base, head := providerTestRepository(t)
	var calls atomic.Int64
	client := &http.Client{Transport: providerRoundTrip(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		if request.URL.String() != "https://api.openai.com/v1/decisions" || request.Header.Get("Authorization") != "Bearer bound-secret" {
			t.Error("wrong OpenAI destination or credential")
		}
		var payload struct {
			Model     string `json:"model"`
			Questions []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("decode OpenAI request: %w", err)
		}
		answers := make([]map[string]any, 0, len(payload.Questions))
		for _, question := range payload.Questions {
			if question.Type != "predicate" {
				return nil, fmt.Errorf("question type = %q", question.Type)
			}
			answers = append(answers, map[string]any{"type": "predicate", "name": question.Name, "probability": 1.0})
		}
		body, err := json.Marshal(map[string]any{
			"model": payload.Model, "answers": answers,
			"usage": map[string]any{
				"input_tokens": 100, "input_tokens_details": map[string]int{"cached_tokens": 20, "cache_write_tokens": 10},
				"output_tokens": 5, "output_tokens_details": map[string]int{"reasoning_tokens": 0}, "total_tokens": 105,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("encode OpenAI response: %w", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	previousClient := http.DefaultClient
	http.DefaultClient = client
	t.Cleanup(func() { http.DefaultClient = previousClient })

	options := cli.Options{Base: base, Head: head, Provider: "openai", Model: "gpt-6-luna",
		Credential: "bound-secret", CredentialResolved: true}
	artifact, err := lintWithRuntime(t.Context(), dir, options, adkruntime.New())
	if err != nil {
		t.Fatal(err)
	}
	callCount := int(calls.Load())
	wantCost := float64(callCount) * 0.00001095
	if callCount == 0 || artifact.SchemaVersion != report.SchemaVersion || artifact.Stats.Review == nil ||
		artifact.Stats.Review.RequestCount != callCount || artifact.Stats.Review.CostUSD == nil ||
		math.Abs(*artifact.Stats.Review.CostUSD-wantCost) > 1e-15 {
		t.Fatalf("calls=%d report=%+v", callCount, artifact)
	}
	var output bytes.Buffer
	if err := report.WriteJSON(&output, artifact); err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Version string `json:"version"`
		Stats   struct {
			Review *report.ReviewMetrics `json:"review"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(output.Bytes(), &wire); err != nil || wire.Version != "v5" || wire.Stats.Review == nil ||
		wire.Stats.Review.RequestCount != callCount || wire.Stats.Review.CostUSD == nil {
		t.Fatalf("wire report: %+v, %v", wire, err)
	}
}

func providerTestRepository(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	gitCommand := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = dir
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	gitCommand("init", "-q")
	gitCommand("config", "user.name", "Test")
	gitCommand("config", "user.email", "test@example.invalid")
	rulesDir := filepath.Join(dir, ".lintpal", "rules")
	if err := os.MkdirAll(rulesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "review.md"), []byte("Changed code must be safe.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "example.go")
	if err := os.WriteFile(path, []byte("package example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand("add", ".")
	gitCommand("commit", "-qm", "base")
	base := gitCommand("rev-parse", "HEAD")
	if err := os.WriteFile(path, []byte("package example\nvar X = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand("add", "example.go")
	gitCommand("commit", "-qm", "head")
	return dir, base, gitCommand("rev-parse", "HEAD")
}
