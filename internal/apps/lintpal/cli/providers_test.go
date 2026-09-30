package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func TestCustomPathPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, flag, env, want    string
		flagSet, envSet, invalid bool
	}{
		{name: "default", want: "/v1/systemone"},
		{name: "env", env: "/decisions", envSet: true, want: "/decisions"},
		{name: "flag", flag: "/v1/decisions", flagSet: true, env: "/ignored", envSet: true, want: "/v1/decisions"},
		{name: "empty flag", flagSet: true, env: "/decisions", envSet: true, invalid: true},
		{name: "empty env", envSet: true, invalid: true},
		{name: "relative", flag: "decisions", flagSet: true, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := RawOptions{Uncommitted: true, Provider: "custom", BaseURL: "https://example.test/api", APIPath: tc.flag,
				Changed: map[string]bool{"provider": true, "base-url": true, "api-path": tc.flagSet}}
			lookup := func(name string) (string, bool) {
				if name == "LINTPAL_API_PATH" {
					return tc.env, tc.envSet
				}
				return "", false
			}
			options, err := Resolve(raw, lookup)
			if tc.invalid {
				if !errors.Is(err, ErrInvalidOptions) {
					t.Fatalf("invalid path accepted: %v", err)
				}
			} else if err != nil || options.APIPath != tc.want {
				t.Fatalf("path = %q, error = %v", options.APIPath, err)
			}
		})
	}
}

func TestOpenAIModelAndCredentialSelection(t *testing.T) {
	raw := RawOptions{Uncommitted: true, Provider: "openai", Changed: map[string]bool{"provider": true}}
	if _, err := Resolve(raw, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("missing model accepted: %v", err)
	}
	for _, flagModel := range []bool{false, true} {
		lookup := func(name string) (string, bool) {
			switch name {
			case "TYPESAFE_API_KEY", "OPENROUTER_API_KEY", "LINTPAL_TOKEN":
				t.Fatalf("read unselected credential %s", name)
			case "OPENAI_API_KEY":
				return "openai-sentinel", true
			case "LINTPAL_MODEL":
				return "env-model", true
			}
			return "", false
		}
		want := "env-model"
		if flagModel {
			raw.Model = "flag-model"
			raw.Changed["model"] = true
			want = raw.Model
		}
		options, err := Resolve(raw, lookup)
		if err != nil || options.Model != want || options.Credential != "openai-sentinel" || !options.CredentialResolved {
			t.Fatalf("model/credential selection failed: error = %v", err)
		}
	}
}

func TestProviderConfigurationInLintAndDoctor(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("TYPESAFE_API_KEY", "native-sentinel")
	t.Setenv("OPENROUTER_API_KEY", "router-sentinel")
	t.Setenv("OPENAI_API_KEY", "openai-sentinel")
	t.Setenv("LINTPAL_TOKEN", "custom-sentinel")
	for _, tc := range []struct {
		args    []string
		invalid bool
	}{
		{args: []string{"--provider", "openai"}},
		{args: []string{"--provider", "custom", "--base-url", "https://example.test/api", "--api-path", "/v1/decisions"}},
		{args: []string{"--provider", "custom", "--base-url", "https://example.test", "--api-path", ""}, invalid: true},
		{args: []string{"--provider", "custom", "--base-url", "https://example.test", "--api-path", "//other"}, invalid: true},
		{args: []string{"--provider", "custom", "--base-url", "https://example.test", "--auth-token-env", "OPENAI_API_KEY"}, invalid: true},
		{args: []string{"--provider", "jev", "--api-path", "/v1/decisions"}, invalid: true},
		{args: []string{"--provider", "openrouter", "--api-path", ""}, invalid: true},
		{args: []string{"--provider", "openai", "--base-url", "https://other.test"}, invalid: true},
		{args: []string{"--provider", "openai", "--auth-token-env", "OTHER_KEY"}, invalid: true},
	} {
		for _, command := range []string{"lint", "doctor"} {
			called := false
			root := NewRoot(func(_ context.Context, options Options, _, _ io.Writer) error {
				called = true
				if options.Provider == "custom" && options.APIPath != "/v1/decisions" {
					t.Errorf("wrong custom path %q", options.APIPath)
				}
				return nil
			}, "test")
			args := append([]string{command, "--no-env-file"}, tc.args...)
			if command == "lint" {
				args = append(args, "--uncommitted", "--model", "test-model")
			}
			var output bytes.Buffer
			root.SetArgs(args)
			root.SetOut(&output)
			root.SetErr(&output)
			err := root.ExecuteContext(t.Context())
			if tc.invalid {
				if !errors.Is(err, ErrInvalidOptions) || called {
					t.Fatalf("%v: invalid config ran lint or succeeded: %v", args, err)
				}
			} else if err != nil || (command == "lint" && !called) {
				t.Fatalf("%v: %v", args, err)
			}
			if strings.Contains(output.String(), "sentinel") {
				t.Fatalf("%v leaked credential", args)
			}
		}
	}
}
