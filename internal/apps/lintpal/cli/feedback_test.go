package cli

import (
	"bytes"
	"errors"
	"io"
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
