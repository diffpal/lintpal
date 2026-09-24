package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// LintFunc receives parsed process options and the command's output streams.
type LintFunc func(context.Context, Options, io.Writer, io.Writer) error

// NewRoot constructs the executable command tree without opening files or
// constructing a provider. Later commands can be added at this boundary.
func NewRoot(lint LintFunc, version string) *cobra.Command {
	root := &cobra.Command{Use: "lintpal", Short: "Lint committed Git changes with typed decisions", SilenceUsage: true, SilenceErrors: true}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return ErrInvalidOptions
		}
		return cmd.Help()
	}
	root.SetFlagErrorFunc(func(*cobra.Command, error) error { return ErrInvalidOptions })
	root.Args = func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return ErrInvalidOptions
		}
		return nil
	}
	var raw RawOptions
	var envFile string
	var noEnvFile bool
	command := &cobra.Command{Use: "lint", Short: "Lint a committed Git diff", Args: func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return ErrInvalidOptions
		}
		return nil
	}}
	flags := command.Flags()
	flags.StringVar(&raw.Base, "base", "", "Base commit or revision")
	flags.StringVar(&raw.Head, "head", "", "Head commit or revision")
	flags.StringVar(&raw.Provider, "provider", "", "Jev provider: jev, openrouter, or custom")
	flags.StringVar(&raw.Model, "model", "", "System One model name or alias")
	flags.StringVar(&raw.Rules, "rules", "", "Declarative rule pack path")
	flags.StringVar(&raw.Format, "format", "", "Stdout format: human or json")
	flags.StringVar(&raw.Out, "out", "", "Additional JSON artifact path")
	flags.StringVar(&raw.FailOn, "fail-on", "", "Gate threshold: low, medium, high, critical, or none")
	flags.StringVar(&raw.Timeout, "timeout", "", "Whole-run timeout")
	flags.StringVar(&raw.MaxConcurrency, "max-concurrency", "", "Maximum provider concurrency")
	flags.StringVar(&raw.BaseURL, "base-url", "", "Trusted custom provider base URL")
	flags.StringVar(&raw.AuthTokenEnv, "auth-token-env", "", "Trusted custom provider token environment name")
	flags.BoolVar(&raw.Metrics, "metrics", false, "Print local run metrics to stderr")
	flags.StringVar(&envFile, "env-file", "", "Load settings from this .env file")
	flags.BoolVar(&noEnvFile, "no-env-file", false, "Do not load a .env file")
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		if lint == nil {
			return ErrInvalidOptions
		}
		if flags.Changed("env-file") && (envFile == "" || noEnvFile) {
			return ErrInvalidOptions
		}
		raw.Changed = make(map[string]bool)
		flags.Visit(func(flag *pflag.Flag) { raw.Changed[flag.Name] = true })
		dir, err := os.Getwd()
		if err != nil {
			return ErrInvalidOptions
		}
		lookup, err := LoadEnv(cmd.Context(), dir, envFile, noEnvFile)
		if err != nil {
			return err
		}
		options, err := Resolve(raw, lookup)
		if err != nil {
			return err
		}
		return lint(cmd.Context(), options, cmd.OutOrStdout(), cmd.ErrOrStderr())
	}
	root.AddCommand(command)
	root.AddCommand(newDoctorCommand())
	root.AddCommand(newPackCommand())
	if version == "" {
		version = "dev"
	}
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print the lintpal version", Args: func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return ErrInvalidOptions
		}
		return nil
	}, RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "lintpal %s\n", version)
		return err
	}})
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newCompletionCommand(root))
	return root
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	command := &cobra.Command{Use: "completion <bash|zsh|fish|powershell>", Short: "Generate shell completion", Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidOptions
		}
		return nil
	}}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
		case "zsh":
			return root.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return root.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return ErrInvalidOptions
		}
	}
	return command
}
