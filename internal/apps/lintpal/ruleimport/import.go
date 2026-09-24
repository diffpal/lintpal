// Package ruleimport copies validated Markdown mandates into a repository.
package ruleimport

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rulesource"
)

var ErrConflict = errors.New("rule import conflict")
var ErrStorage = errors.New("rule import storage failed")

type Result struct {
	Created     []string
	Overwritten []string
	Unchanged   []string
	Commit      string
}

// Import reads SOURCE and installs its Markdown rules under the worktree rule root.
func Import(ctx context.Context, repositoryRoot, source, prefix string, force bool) (Result, error) {
	if source == "" {
		return Result{}, rulesource.ErrSource
	}
	var incoming []rules.Mandate
	var commit string
	var err error
	if strings.HasPrefix(source, "github:") {
		incoming, commit, err = rulesource.ReadGitHubMarkdown(ctx, source)
	} else {
		path, pathErr := filepath.Abs(source)
		if pathErr != nil {
			return Result{}, rulesource.ErrSource
		}
		incoming, err = rules.ReadMandates(ctx, path)
	}
	if err != nil {
		return Result{}, err
	}
	if len(incoming) == 0 {
		return Result{}, rules.ErrInvalidPack
	}
	for i := range incoming {
		if prefix != "" {
			incoming[i].ID = prefix + "/" + incoming[i].ID
		}
	}
	if _, err := rules.CompileMandates(incoming); err != nil {
		return Result{}, err
	}
	base := filepath.Join(repositoryRoot, ".lintpal")
	root := filepath.Join(base, "rules")
	if err := safeDirectory(repositoryRoot, false); err != nil {
		return Result{}, err
	}
	if err := safeDirectory(base, true); err != nil {
		return Result{}, err
	}
	if err := safeDirectory(root, true); err != nil {
		return Result{}, err
	}
	current := map[string]rules.Mandate{}
	rootExists := false
	if info, err := os.Lstat(root); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return Result{}, ErrStorage
		}
		rootExists = true
		mandates, err := rules.ReadMandates(ctx, root)
		if err != nil {
			return Result{}, err
		}
		for _, mandate := range mandates {
			current[mandate.ID] = mandate
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, ErrStorage
	}
	result := Result{Commit: commit}
	for _, mandate := range incoming {
		if previous, exists := current[mandate.ID]; exists {
			if !force {
				return Result{}, ErrConflict
			}
			if previous.Body == mandate.Body {
				result.Unchanged = append(result.Unchanged, mandate.ID)
			} else {
				result.Overwritten = append(result.Overwritten, mandate.ID)
			}
		} else {
			result.Created = append(result.Created, mandate.ID)
		}
		current[mandate.ID] = mandate
	}
	combined := make([]rules.Mandate, 0, len(current))
	for _, mandate := range current {
		combined = append(combined, mandate)
	}
	if _, err := rules.CompileMandates(combined); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := os.Mkdir(base, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return Result{}, ErrStorage
	}
	if err := safeDirectory(base, false); err != nil {
		return Result{}, err
	}
	stage, err := os.MkdirTemp(base, ".rules-stage-")
	if err != nil {
		return Result{}, ErrStorage
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if rootExists {
		if err := copyTree(ctx, root, stage); err != nil {
			return Result{}, err
		}
	}
	for _, mandate := range incoming {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		target := filepath.Join(stage, filepath.FromSlash(mandate.ID))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return Result{}, ErrStorage
		}
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrStorage
		}
		if err := os.WriteFile(target, []byte(mandate.Body), 0600); err != nil {
			return Result{}, ErrStorage
		}
	}
	if _, err := rules.LoadDirectory(ctx, stage); err != nil {
		return Result{}, err
	}
	if err := swapRuleTree(ctx, root, stage, rootExists, os.Rename); err != nil {
		return Result{}, err
	}
	sort.Strings(result.Created)
	sort.Strings(result.Overwritten)
	sort.Strings(result.Unchanged)
	return result, nil
}

func safeDirectory(path string, mayBeMissing bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && mayBeMissing {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrStorage
	}
	return nil
}

func copyTree(ctx context.Context, source, target string) error {
	return filepath.WalkDir(source, func(file string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil || entry.Type()&os.ModeSymlink != 0 {
			return ErrStorage
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ErrStorage
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			if err := os.Mkdir(dest, 0700); err != nil {
				return ErrStorage
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return ErrStorage
		}
		input, err := os.Open(file)
		if err != nil {
			return ErrStorage
		}
		output, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			_ = input.Close()
			return ErrStorage
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		inputErr := input.Close()
		if copyErr != nil || closeErr != nil || inputErr != nil {
			return ErrStorage
		}
		info, err := entry.Info()
		if err != nil || os.Chmod(dest, info.Mode().Perm()) != nil {
			return ErrStorage
		}
		return nil
	})
}

func swapRuleTree(ctx context.Context, root, stage string, exists bool, rename func(string, string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !exists {
		if err := rename(stage, root); err != nil {
			return ErrStorage
		}
		return nil
	}
	base := filepath.Dir(root)
	backup, err := os.MkdirTemp(base, ".rules-backup-")
	if err != nil {
		return ErrStorage
	}
	removeBackup := true
	defer func() {
		if removeBackup {
			_ = os.RemoveAll(backup)
		}
	}()
	old := filepath.Join(backup, "rules")
	if err := rename(root, old); err != nil {
		return ErrStorage
	}
	if err := ctx.Err(); err != nil {
		if rename(old, root) != nil {
			removeBackup = false
			return ErrStorage
		}
		return err
	}
	if err := rename(stage, root); err != nil {
		if rename(old, root) != nil {
			removeBackup = false
			return ErrStorage
		}
		return ErrStorage
	}
	return nil
}
