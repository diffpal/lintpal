package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/githubfeedback"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	"github.com/spf13/cobra"
)

func newFeedbackCommand() *cobra.Command {
	feedback := &cobra.Command{Use: "feedback", Short: "Render stored lint findings", Args: cobra.NoArgs}
	feedback.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	var input, output string
	var gate bool
	markdown := &cobra.Command{Use: "markdown", Short: "Render a findings v5 report as Markdown", Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 0 {
			return ErrInvalidOptions
		}
		return nil
	}}
	markdown.Flags().StringVar(&input, "in", "", "Findings v5 JSON report to read")
	markdown.Flags().StringVar(&output, "out", "", "Write Markdown atomically to this path instead of stdout")
	markdown.Flags().BoolVar(&gate, "gate", false, "Exit 10 after output if a stored finding is blocking")
	markdown.RunE = func(cmd *cobra.Command, _ []string) error {
		if input == "" || input == "-" || len(input) > 4096 || strings.ContainsRune(input, 0) ||
			output == "-" || len(output) > 4096 || strings.ContainsRune(output, 0) {
			return ErrInvalidOptions
		}
		if output != "" {
			if filepath.Clean(input) == filepath.Clean(output) {
				return ErrInvalidOptions
			}
			if existing, err := os.Stat(output); err == nil && existing.IsDir() {
				return ErrInvalidOptions
			}
		}
		bundle, err := report.ReadBundle(input)
		if err != nil {
			return err
		}
		payload := report.RenderMarkdown(bundle)
		if output != "" {
			if err := report.WriteArtifact(output, payload); err != nil {
				return err
			}
		} else {
			written, err := cmd.OutOrStdout().Write(payload)
			if err != nil || written != len(payload) {
				return fmt.Errorf("%w: write stdout", report.ErrExport)
			}
		}
		if gate && report.BundleBlocks(bundle) {
			return report.ErrGate
		}
		return nil
	}
	feedback.AddCommand(markdown)
	feedback.AddCommand(newGitHubFeedbackCommand())
	return feedback
}

func newGitHubFeedbackCommand() *cobra.Command {
	var input, repoName, baseSHA, headSHA, channel, tokenEnv string
	var prNumber int
	var dryRun, gate bool
	command := &cobra.Command{Use: "github", Short: "Publish stored findings to a GitHub pull request", Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 0 {
			return ErrInvalidOptions
		}
		return nil
	}}
	flags := command.Flags()
	flags.StringVar(&input, "in", "", "Findings v5 JSON report to read")
	flags.StringVar(&repoName, "repo", "", "GitHub repository as owner/name")
	flags.IntVar(&prNumber, "pr-number", 0, "GitHub pull request number")
	flags.StringVar(&baseSHA, "base", "", "Expected pull request base SHA")
	flags.StringVar(&headSHA, "head", "", "Expected pull request head SHA")
	flags.StringVar(&channel, "review-channel", githubfeedback.DefaultChannel, "Stable GitHub publication channel")
	flags.StringVar(&tokenEnv, "auth-token-env", "GITHUB_TOKEN", "Environment variable containing the GitHub token")
	flags.BoolVar(&dryRun, "dry-run", false, "Render intended GitHub feedback without API writes")
	flags.BoolVar(&gate, "gate", false, "Exit 10 after output or publication if a stored finding is blocking")
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		if input == "" || input == "-" || len(input) > 4096 || strings.ContainsRune(input, 0) || !validEnvName(tokenEnv) {
			return ErrInvalidOptions
		}
		bundle, err := report.ReadBundle(input)
		if err != nil {
			return err
		}
		identity, err := githubfeedback.NewIdentity(channel)
		if err != nil {
			return ErrInvalidOptions
		}
		reviewCtx, err := githubfeedback.ResolveContext(githubfeedback.ContextOptions{
			Repo: repoName, PRNumber: prNumber, BaseSHA: baseSHA, HeadSHA: headSHA,
		})
		if err != nil || reviewCtx.BaseSHA != bundle.BaseSHA || reviewCtx.HeadSHA != bundle.HeadSHA {
			return githubfeedback.ErrInvalidContext
		}
		if reviewCtx.UnsafeFork {
			_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), "GitHub publication skipped for fork pull request")
			if writeErr != nil {
				return fmt.Errorf("%w: write stdout", report.ErrExport)
			}
			if gate && report.BundleBlocks(bundle) {
				return report.ErrGate
			}
			return nil
		}
		var active map[string]string
		var publisher *githubfeedback.Publisher
		var token string
		if !dryRun {
			token = os.Getenv(tokenEnv)
			if strings.TrimSpace(token) == "" {
				return githubfeedback.ErrInvalidContext
			}
			if bundleContains(bundle, token) {
				return report.ErrExport
			}
			apiBase := strings.TrimSpace(os.Getenv("GITHUB_API_URL"))
			if apiBase == "" {
				apiBase = "https://api.github.com"
			}
			publisher, err = githubfeedback.NewPublisher(apiBase, http.DefaultClient)
			if err != nil {
				return err
			}
			active, err = publisher.ActiveFindings(cmd.Context(), token, reviewCtx, identity)
			if err != nil {
				return err
			}
		}
		plan := githubfeedback.PlanComments(bundle, active)
		result, err := report.RenderGitHubResult(bundle, plan.UnanchoredIDs)
		if err != nil {
			return err
		}
		if dryRun {
			if err := writeGitHubPreview(cmd, result, plan); err != nil {
				return err
			}
		} else {
			if err := publisher.Publish(cmd.Context(), token, reviewCtx, identity, string(result), plan); err != nil {
				return err
			}
		}
		if gate && report.BundleBlocks(bundle) {
			return report.ErrGate
		}
		return nil
	}
	return command
}

func bundleContains(bundle report.Bundle, value string) bool {
	if value == "" {
		return false
	}
	payload, err := json.Marshal(bundle)
	return err != nil || strings.Contains(string(payload), value)
}

func writeGitHubPreview(command *cobra.Command, result []byte, plan githubfeedback.Plan) error {
	writer := command.OutOrStdout()
	if _, err := writer.Write(result); err != nil {
		return fmt.Errorf("%w: write stdout", report.ErrExport)
	}
	for _, comment := range plan.Comments {
		if _, err := fmt.Fprintf(writer, "\n---\n\n%s:%d-%d (%s)\n\n%s", comment.Path, comment.StartLine, comment.EndLine, comment.Side, comment.Body); err != nil {
			return fmt.Errorf("%w: write stdout", report.ErrExport)
		}
	}
	return nil
}

func validEnvName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, char := range value {
		if char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char == '_' || index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}
