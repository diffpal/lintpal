package packs

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

// RepositoryRoot resolves the current worktree root without reading source.
func RepositoryRoot(ctx context.Context, cwd string) (string, error) {
	command := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
	command.Stderr = io.Discard
	data, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrSource
	}
	root := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if root == "" {
		return "", ErrSource
	}
	return root, nil
}
