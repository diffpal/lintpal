package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/packs"
	"github.com/spf13/cobra"
)

func newPackCommand() *cobra.Command {
	command := &cobra.Command{Use: "pack", Short: "Manage project rule packs"}
	command.AddCommand(&cobra.Command{
		Use: "import NAME SOURCE", Short: "Import a local or pinned GitHub rule pack",
		Args: packArgs(2, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := packRoot(cmd)
			if err != nil {
				return err
			}
			entry, err := importPack(cmd, root, args[0], args[1], false)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "imported pack %s sha256:%s\n", entry.Name, entry.SHA256)
			return err
		},
	})
	command.AddCommand(&cobra.Command{
		Use: "update NAME [SOURCE]", Short: "Update an installed rule pack",
		Args: packArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := packRoot(cmd)
			if err != nil {
				return err
			}
			previous, err := packs.Get(root, args[0])
			if err != nil {
				return err
			}
			source := previous.Source
			if previous.SourceKind == "github" {
				source += "@" + previous.RequestedRef
			}
			if len(args) == 2 {
				source = args[1]
			}
			entry, err := importPack(cmd, root, args[0], source, true)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated pack %s sha256:%s\n", entry.Name, entry.SHA256)
			return err
		},
	})
	command.AddCommand(&cobra.Command{
		Use: "verify [NAME]", Short: "Verify installed packs against the lockfile",
		Args: packArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := packRoot(cmd)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			entries, err := packs.Verify(cmd.Context(), root, name)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "verified %d pack(s)\n", len(entries))
			return err
		},
	})
	return command
}

func packArgs(min, max int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < min || len(args) > max {
			return ErrInvalidOptions
		}
		return nil
	}
}

func packRoot(cmd *cobra.Command) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", ErrInvalidOptions
	}
	return packs.RepositoryRoot(cmd.Context(), cwd)
}

func importPack(cmd *cobra.Command, root, name, source string, update bool) (packs.Entry, error) {
	if strings.HasPrefix(source, "github:") {
		return packs.ImportGitHub(cmd.Context(), root, name, source, update)
	}
	return packs.ImportLocal(cmd.Context(), root, name, source, update)
}
