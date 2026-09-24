package cli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
)

func TestFeedbackMarkdownUsesStoredBlocking(t *testing.T) {
	input := filepath.Join("..", "..", "..", "..", "docs", "schema", "testdata", "diffpal-left.json")
	run := func(args ...string) (string, error) {
		t.Helper()
		var output bytes.Buffer
		root := NewRoot(nil, "test")
		root.SetArgs(append([]string{"feedback", "markdown", "--in", input}, args...))
		root.SetOut(&output)
		root.SetErr(&bytes.Buffer{})
		err := root.Execute()
		return output.String(), err
	}
	output, err := run()
	if err != nil || !strings.Contains(output, "(LEFT)") || !strings.Contains(output, "(blocking)") || !strings.Contains(output, "Evidence: deleted validation branch") {
		t.Fatalf("stored feedback: %q, %v", output, err)
	}
	output, err = run("--gate")
	if !errors.Is(err, report.ErrGate) || output == "" {
		t.Fatalf("gate happened before output: %q, %v", output, err)
	}
	outPath := filepath.Join(t.TempDir(), "feedback.md")
	output, err = run("--out", outPath, "--gate")
	artifact, readErr := os.ReadFile(outPath)
	if !errors.Is(err, report.ErrGate) || readErr != nil || output != "" || !strings.Contains(string(artifact), "(LEFT)") {
		t.Fatalf("file feedback: %q, %q, %v, %v", output, artifact, err, readErr)
	}
	if _, err := run("--out", input); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("allowed input overwrite: %v", err)
	}
}

func TestFeedbackRejectsInvalidBeforeOutput(t *testing.T) {
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"version":"v5","findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "feedback.md")
	root := NewRoot(nil, "test")
	root.SetArgs([]string{"feedback", "markdown", "--in", invalid, "--out", output, "--gate"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); !errors.Is(err, report.ErrInvalidReport) {
		t.Fatalf("invalid report accepted: %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("invalid report produced feedback: %v", err)
	}
}

type brokenFeedbackWriter struct{}

func (brokenFeedbackWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFeedbackOutputFailurePrecedesGate(t *testing.T) {
	input := filepath.Join("..", "..", "..", "..", "docs", "schema", "testdata", "diffpal-left.json")
	root := NewRoot(nil, "test")
	root.SetArgs([]string{"feedback", "markdown", "--in", input, "--gate"})
	root.SetOut(brokenFeedbackWriter{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); !errors.Is(err, report.ErrExport) || errors.Is(err, report.ErrGate) {
		t.Fatalf("output failure confused with gate: %v", err)
	}
}

func TestFeedbackGitHubDryRunRendersGateAndInlineWithoutToken(t *testing.T) {
	input := filepath.Join("..", "..", "..", "..", "docs", "schema", "testdata", "diffpal-left.json")
	bundle, err := report.ReadBundle(input)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_TOKEN", "")
	var output bytes.Buffer
	root := NewRoot(nil, "test")
	root.SetArgs([]string{"feedback", "github", "--in", input, "--repo", "owner/repo", "--pr-number", "7",
		"--base", bundle.BaseSHA, "--head", bundle.HeadSHA, "--dry-run", "--gate"})
	root.SetOut(&output)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); !errors.Is(err, report.ErrGate) {
		t.Fatalf("dry-run gate: %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "1 blocking finding") || !strings.Contains(text, "LEFT") || !strings.Contains(text, "deleted check allowed an invalid input") {
		t.Fatalf("incomplete preview: %s", text)
	}
}

func TestFeedbackGitHubPublishesBeforeGate(t *testing.T) {
	input := filepath.Join("..", "..", "..", "..", "docs", "schema", "testdata", "diffpal-left.json")
	bundle, err := report.ReadBundle(input)
	if err != nil {
		t.Fatal(err)
	}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/graphql":
			_, _ = writer.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/reviews"):
			_, _ = writer.Write([]byte(`[]`))
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/reviews"):
			posts++
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("GITHUB_TOKEN", "secret-token")
	root := NewRoot(nil, "test")
	root.SetArgs([]string{"feedback", "github", "--in", input, "--repo", "owner/repo", "--pr-number", "7",
		"--base", bundle.BaseSHA, "--head", bundle.HeadSHA, "--gate"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); !errors.Is(err, report.ErrGate) || posts != 1 {
		t.Fatalf("publication did not precede gate: posts=%d err=%v", posts, err)
	}
}

func TestFeedbackGitHubRejectsInvalidReportBeforeNetwork(t *testing.T) {
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"version":"v5","findings":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("GITHUB_TOKEN", "token")
	root := NewRoot(nil, "test")
	root.SetArgs([]string{"feedback", "github", "--in", invalid, "--repo", "owner/repo", "--pr-number", "7", "--base", "a", "--head", "b"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); !errors.Is(err, report.ErrInvalidReport) || requests != 0 {
		t.Fatalf("invalid input reached network: requests=%d err=%v", requests, err)
	}
}

func TestFeedbackGitHubRejectsTokenInStoredBundle(t *testing.T) {
	source := filepath.Join("..", "..", "..", "..", "docs", "schema", "testdata", "lintpal-right.json")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("Changed code may violate go/errors.md."), []byte("contains secret-token-value"), 1)
	input := filepath.Join(t.TempDir(), "findings.json")
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	bundle, err := report.ReadBundle(input)
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("GITHUB_TOKEN", "secret-token-value")
	root := NewRoot(nil, "test")
	root.SetArgs([]string{"feedback", "github", "--in", input, "--repo", "owner/repo", "--pr-number", "7", "--base", bundle.BaseSHA, "--head", bundle.HeadSHA})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); !errors.Is(err, report.ErrExport) || requests != 0 {
		t.Fatalf("secret-bearing bundle reached network: requests=%d err=%v", requests, err)
	}
}
