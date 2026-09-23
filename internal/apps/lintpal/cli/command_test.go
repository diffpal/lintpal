package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestAuxiliaryCommandsDoNotRunLint(t *testing.T) {
	dir := t.TempDir()
	command := exec.Command("git", "init", "-q", dir)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	t.Chdir(dir)
	t.Setenv("TYPESAFE_API_KEY", "secret-sentinel")
	called := false
	lint := func(context.Context, Options, io.Writer, io.Writer) error { called = true; return nil }
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--help"}, "doctor"},
		{[]string{"version"}, "lintpal test-version"},
		{[]string{"completion", "bash"}, "__start_lintpal"},
		{[]string{"doctor"}, "credential: present"},
	} {
		root := NewRoot(lint, "test-version")
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(test.args)
		if err := root.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if !strings.Contains(out.String(), test.want) || strings.Contains(out.String(), "secret-sentinel") {
			t.Fatalf("%v output: %q", test.args, out.String())
		}
	}
	if called {
		t.Fatal("auxiliary command invoked lint")
	}
	if err := os.Unsetenv("TYPESAFE_API_KEY"); err != nil {
		t.Fatal(err)
	}
	root := NewRoot(lint, "test-version")
	root.SetArgs([]string{"doctor"})
	if err := root.ExecuteContext(t.Context()); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("missing credential: %v", err)
	}
}

func TestLintProviderSelection(t *testing.T) {
	for _, provider := range []string{"jev", "openrouter", "custom"} {
		var selected string
		root := NewRoot(func(_ context.Context, o Options, _, _ io.Writer) error { selected = o.Provider; return nil }, "dev")
		args := []string{"lint", "--base", "a", "--head", "b", "--provider", provider}
		if provider == "custom" {
			args = append(args, "--base-url", "http://127.0.0.1:1")
		}
		root.SetArgs(args)
		if err := root.ExecuteContext(t.Context()); err != nil || selected != provider {
			t.Fatalf("provider %s: selected %q, err %v", provider, selected, err)
		}
	}
}
