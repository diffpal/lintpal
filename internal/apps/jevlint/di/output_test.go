package di

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/diffpal/jevlint/internal/apps/jevlint/cli"
	"github.com/diffpal/jevlint/internal/apps/jevlint/report"
)

func TestWriteArtifactReplacesOnlyWithCompletePayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeArtifact(path, []byte("complete")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "complete" {
		t.Fatalf("artifact: %q %v", data, err)
	}
	if err := writeArtifact(dir, []byte("bad")); !errors.Is(err, cli.ErrInvalidOptions) {
		t.Fatalf("directory accepted: %v", err)
	}
	if err := writeArtifact(filepath.Join(dir, "missing", "report.json"), []byte("bad")); !errors.Is(err, report.ErrExport) {
		t.Fatalf("missing parent: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary artifact retained: %v %v", entries, err)
	}
}

func TestSelectedCredentialCannotEnterReport(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "secret-sentinel")
	t.Setenv("OPENROUTER_API_KEY", "router-sentinel")
	t.Setenv("JEVLINT_TOKEN", "custom-sentinel")
	for _, test := range []struct {
		options cli.Options
		want    string
	}{
		{cli.Options{Provider: "jev"}, "secret-sentinel"},
		{cli.Options{Provider: "openrouter"}, "router-sentinel"},
		{cli.Options{Provider: "custom", AuthTokenEnv: "JEVLINT_TOKEN"}, "custom-sentinel"},
	} {
		credential := selectedCredential(test.options)
		if credential != test.want {
			t.Fatalf("wrong token source for %s", test.options.Provider)
		}
		artifact := report.Report{Diagnostics: []report.Diagnostic{{Path: "safe.go", Title: "contains " + credential}}, Skips: []report.Skip{}}
		if !containsCredential(artifact, credential) {
			t.Fatal("secret-bearing title accepted")
		}
		artifact.Diagnostics[0].Title = "safe"
		artifact.Skips = []report.Skip{{OldPath: "src/" + credential, NewPath: "safe.go"}}
		if !containsCredential(artifact, credential) {
			t.Fatal("secret-bearing skip path accepted")
		}
		artifact.Skips = nil
		if containsCredential(artifact, credential) {
			t.Fatal("safe report rejected")
		}
	}
}
