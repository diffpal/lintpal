package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	"github.com/diffpal/lintpal/internal/apps/lintpal/ruleimport"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rulesource"
	"github.com/spf13/cobra"
)

func newRuleCommand() *cobra.Command {
	command := &cobra.Command{Use: "rule", Short: "Inspect repository rules"}
	var prefix string
	var force bool
	importCommand := &cobra.Command{Use: "import SOURCE", Short: "Import Markdown rules into this repository", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := repositoryRoot(cmd)
			if err != nil {
				return err
			}
			result, err := ruleimport.Import(cmd.Context(), root, args[0], prefix, force)
			if err != nil {
				return err
			}
			for _, id := range result.Created {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", id); err != nil {
					return fmt.Errorf("%w: rule import output", report.ErrExport)
				}
			}
			for _, id := range result.Overwritten {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "overwritten %s\n", id); err != nil {
					return fmt.Errorf("%w: rule import output", report.ErrExport)
				}
			}
			for _, id := range result.Unchanged {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "unchanged %s\n", id); err != nil {
					return fmt.Errorf("%w: rule import output", report.ErrExport)
				}
			}
			if result.Commit != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "source commit %s\n", result.Commit); err != nil {
					return fmt.Errorf("%w: rule import output", report.ErrExport)
				}
			}
			return nil
		}}
	importCommand.Flags().StringVar(&prefix, "prefix", "", "Prefix imported rule IDs")
	importCommand.Flags().BoolVar(&force, "force", false, "Replace colliding rule files")
	command.AddCommand(importCommand)
	command.AddCommand(&cobra.Command{Use: "list", Short: "List repository rule IDs", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalog, err := loadRuleCatalog(cmd)
			if err != nil {
				return err
			}
			for _, rule := range catalog.Rules() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), rule.ID); err != nil {
					return fmt.Errorf("%w: rule list output", report.ErrExport)
				}
			}
			return nil
		}})
	command.AddCommand(&cobra.Command{Use: "view ID", Short: "Show a rule and its effective policy", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := loadRuleCatalog(cmd)
			if err != nil {
				return err
			}
			for _, rule := range catalog.Rules() {
				if rule.ID != args[0] {
					continue
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\nTitle: %s\nSeverity: %s\nThreshold: %g\n\n%s\n",
					rule.ID, rule.Title, rule.Severity, rule.Threshold, strings.TrimSpace(rule.Body))
				if err != nil {
					return fmt.Errorf("%w: rule view output", report.ErrExport)
				}
				return nil
			}
			return ErrInvalidOptions
		}})
	command.AddCommand(&cobra.Command{Use: "validate", Short: "Validate repository rules", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalog, err := loadRuleCatalog(cmd)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "validated %d rule(s)\n", len(catalog.Rules())); err != nil {
				return fmt.Errorf("%w: rule validation output", report.ErrExport)
			}
			return nil
		}})
	return command
}

func ruleDirectory(cmd *cobra.Command) (string, error) {
	root, err := repositoryRoot(cmd)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".lintpal", "rules"), nil
}

func repositoryRoot(cmd *cobra.Command) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", ErrInvalidOptions
	}
	return rulesource.RepositoryRoot(cmd.Context(), cwd)
}

func loadRuleCatalog(cmd *cobra.Command) (rules.Pack, error) {
	directory, err := ruleDirectory(cmd)
	if err != nil {
		return rules.Pack{}, err
	}
	return rules.LoadDirectory(cmd.Context(), directory)
}
