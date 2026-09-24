package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvDiscoveryAndPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("LINTPAL_PROVIDER=openrouter\nTYPESAFE_API_KEY='file-secret'\n# comment\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lookup, err := LoadEnv(t.Context(), filepath.Join(dir, "sub"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := lookup("LINTPAL_PROVIDER"); got != "openrouter" {
		t.Fatalf("discovered provider = %q", got)
	}
	t.Setenv("LINTPAL_PROVIDER", "jev")
	if got, _ := lookup("LINTPAL_PROVIDER"); got != "jev" {
		t.Fatalf("process precedence = %q", got)
	}
	if got, _ := lookup("TYPESAFE_API_KEY"); got != "file-secret" {
		t.Fatal("file credential unavailable")
	}
	disabled, err := LoadEnv(t.Context(), dir, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := disabled("TYPESAFE_API_KEY"); ok {
		t.Fatal("disabled discovery loaded file credential")
	}
}

func TestLoadEnvInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	empty, err := LoadEnv(t.Context(), dir, "", false)
	if err != nil {
		t.Fatalf("absent default file: %v", err)
	}
	if _, ok := empty("LINTPAL_FILE_ONLY_TEST"); ok {
		t.Fatal("absent default file returned a value")
	}
	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{"duplicate", "A=1\nA=2\n", ErrInvalidEnvFile},
		{"invalid-key", "BAD-KEY=value\n", ErrInvalidEnvFile},
		{"invalid-quote", "A='value\n", ErrInvalidEnvFile},
		{"embedded-newline", "A=\"one\\ntwo\"\n", ErrInvalidEnvFile},
		{"invalid-utf8", string([]byte{0xff}), ErrInvalidEnvFile},
		{"limit", "A=" + strings.Repeat("x", maxEnvFileBytes), ErrEnvFileLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadEnv(t.Context(), dir, path, false)
			if !errors.Is(err, tc.want) || strings.Contains(err.Error(), "value") {
				t.Fatalf("error = %v, want safe %v", err, tc.want)
			}
		})
	}
	_, err = LoadEnv(t.Context(), dir, "missing", false)
	if !errors.Is(err, ErrInvalidEnvFile) {
		t.Fatalf("missing explicit file: %v", err)
	}
	_, err = LoadEnv(t.Context(), dir, "missing", true)
	if !errors.Is(err, ErrInvalidEnvFile) {
		t.Fatalf("conflicting selection: %v", err)
	}
	path := filepath.Join(dir, "linked")
	if err := os.Symlink(filepath.Join(dir, "duplicate"), path); err != nil {
		t.Fatal(err)
	}
	_, err = LoadEnv(t.Context(), dir, path, false)
	if !errors.Is(err, ErrInvalidEnvFile) {
		t.Fatalf("symlink: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = LoadEnv(ctx, dir, "", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
}

func TestLayeredLookupCopiesFileValues(t *testing.T) {
	file := map[string]string{"A": "file", "B": "file"}
	lookup := LayeredLookup(func(key string) (string, bool) {
		if key == "A" {
			return "", true
		}
		return "", false
	}, file)
	file["B"] = "modified"
	if got, ok := lookup("A"); !ok || got != "" {
		t.Fatalf("empty process value = %q, %v", got, ok)
	}
	if got, ok := lookup("B"); !ok || got != "file" {
		t.Fatalf("copied file value = %q, %v", got, ok)
	}
}
