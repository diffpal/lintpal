package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/diffpal/lintpal/internal/apps/lintpal/provider/systemone"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var provider, baseURL, tokenEnv string
	var envFile string
	var noEnvFile bool
	command := &cobra.Command{Use: "doctor", Short: "Check local lintpal prerequisites", Args: func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return ErrInvalidOptions
		}
		return nil
	}}
	flags := command.Flags()
	flags.StringVar(&provider, "provider", "", "Provider to check: jev, openrouter, or custom")
	flags.StringVar(&baseURL, "base-url", "", "Trusted custom provider base URL")
	flags.StringVar(&tokenEnv, "auth-token-env", "", "Trusted custom token environment name")
	flags.StringVar(&envFile, "env-file", "", "Load settings from this .env file")
	flags.BoolVar(&noEnvFile, "no-env-file", false, "Do not load a .env file")
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		if flags.Changed("env-file") && (envFile == "" || noEnvFile) {
			return ErrInvalidOptions
		}
		dir, err := os.Getwd()
		if err != nil {
			return ErrInvalidOptions
		}
		lookup, err := LoadEnv(cmd.Context(), dir, envFile, noEnvFile)
		if err != nil {
			return err
		}
		choose := func(name, value, env, fallback string) string {
			if flags.Changed(name) {
				return value
			}
			if candidate, ok := lookup(env); ok {
				return candidate
			}
			return fallback
		}
		selected := choose("provider", provider, "LINTPAL_PROVIDER", "jev")
		url := choose("base-url", baseURL, "LINTPAL_BASE_URL", "")
		name := choose("auth-token-env", tokenEnv, "LINTPAL_AUTH_TOKEN_ENV", "")
		switch selected {
		case "jev":
			if url != "" || name != "" {
				return ErrInvalidOptions
			}
			name = "TYPESAFE_API_KEY"
		case "openrouter":
			if url != "" || name != "" {
				return ErrInvalidOptions
			}
			name = "OPENROUTER_API_KEY"
		case "custom":
			if name == "" {
				name = "LINTPAL_TOKEN"
			}
			if _, err := systemone.TrustedCustom(url, name); err != nil {
				return ErrInvalidOptions
			}
		default:
			return ErrInvalidOptions
		}
		return doctorWithLookup(cmd.Context(), cmd.OutOrStdout(), selected, name, lookup)
	}
	return command
}

// Doctor checks local Git and credential presence. It never makes HTTP calls
// or prints token values, source, endpoint URLs, or repository content.
func Doctor(ctx context.Context, output io.Writer, provider, tokenEnv string) error {
	return doctorWithLookup(ctx, output, provider, tokenEnv, os.LookupEnv)
}

func doctorWithLookup(ctx context.Context, output io.Writer, provider, tokenEnv string, lookup LookupEnv) error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrInvalidOptions
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrInvalidOptions
	}
	credential, _ := lookup(tokenEnv)
	present := credential != ""
	status := "missing"
	if present {
		status = "present"
	}
	if _, err := fmt.Fprintf(output, "git: ok\nrepository: ok\nprovider: %s\ncredential: %s\n", provider, status); err != nil {
		return fmt.Errorf("%w: doctor output", ErrInvalidOptions)
	}
	if !present {
		return ErrInvalidOptions
	}
	return nil
}
