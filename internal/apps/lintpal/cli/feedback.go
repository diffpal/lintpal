package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	return feedback
}
