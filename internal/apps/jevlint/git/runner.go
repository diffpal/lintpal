// Package git reads committed changes without consulting the working tree.
package git

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

var ErrLimit = errors.New("git input limit exceeded")
var ErrInvalidRevision = errors.New("invalid commit revision")
var ErrAmbiguousBase = errors.New("comparison has no unique merge base")

const defaultOutputBytes = 8 << 20

type commandRunner struct {
	dir string
}

type cappedWriter struct {
	limit int
	buf   []byte
	full  bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if len(p) > w.limit-len(w.buf) {
		w.full = true
		return 0, ErrLimit
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (r commandRunner) run(ctx context.Context, limit int, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("git output limit: %w", ErrLimit)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	cmd.Env = cleanGitEnv(os.Environ())
	stdout := &cappedWriter{limit: limit}
	cmd.Stdout = stdout
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if stdout.full {
		return nil, ErrLimit
	}
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w", args[0], err)
	}
	return stdout.buf, nil
}

func cleanGitEnv(env []string) []string {
	clean := make([]string, 0, len(env)+4)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			continue
		}
		clean = append(clean, entry)
	}
	return append(clean,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_OPTIONAL_LOCKS=0",
	)
}

// Revisions contains only verified immutable commit IDs.
type Revisions struct {
	Base      string
	Head      string
	MergeBase string
}

func (r commandRunner) resolve(ctx context.Context, base, head string) (Revisions, error) {
	baseID, err := r.resolveCommit(ctx, base)
	if err != nil {
		return Revisions{}, fmt.Errorf("base: %w", err)
	}
	headID, err := r.resolveCommit(ctx, head)
	if err != nil {
		return Revisions{}, fmt.Errorf("head: %w", err)
	}
	out, err := r.run(ctx, 256, "merge-base", "--all", baseID, headID)
	if err != nil {
		if ctx.Err() != nil {
			return Revisions{}, ctx.Err()
		}
		if errors.Is(err, ErrLimit) {
			return Revisions{}, err
		}
		return Revisions{}, fmt.Errorf("merge base: %w", ErrAmbiguousBase)
	}
	ids := strings.Fields(string(out))
	if len(ids) != 1 || !validObjectID(ids[0]) {
		return Revisions{}, ErrAmbiguousBase
	}
	return Revisions{Base: baseID, Head: headID, MergeBase: ids[0]}, nil
}

func (r commandRunner) resolveCommit(ctx context.Context, rev string) (string, error) {
	if rev == "" {
		return "", ErrInvalidRevision
	}
	out, err := r.run(ctx, 128, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrInvalidRevision
	}
	id := strings.TrimSpace(string(out))
	if !validObjectID(id) {
		return "", ErrInvalidRevision
	}
	return id, nil
}

func validObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
