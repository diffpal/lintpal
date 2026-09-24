// Command self-review builds lintpal and reviews a committed comparison.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "self-review: cannot find working directory")
		os.Exit(2)
	}
	root, err := repositoryRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "self-review: run from a Git worktree")
		os.Exit(2)
	}
	os.Exit(run(root, root, os.Getenv, os.Stdout, os.Stderr))
}

func repositoryRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func run(sourceRoot, reviewRoot string, getenv func(string) string, stdout, stderr io.Writer) int {
	base, head := getenv("BASE"), getenv("HEAD")
	if base == "" || head == "" {
		_, _ = fmt.Fprintln(stderr, "self-review: set BASE and HEAD to committed Git revisions")
		return 2
	}
	artifactDir := filepath.Join(reviewRoot, ".artifacts", "lintpal")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		_, _ = fmt.Fprintln(stderr, "self-review: cannot create artifact directory")
		return 4
	}
	report := filepath.Join(artifactDir, "self-review.json")
	if err := os.Remove(report); err != nil && !os.IsNotExist(err) {
		_, _ = fmt.Fprintln(stderr, "self-review: cannot replace previous report")
		return 4
	}
	binary := filepath.Join(artifactDir, "lintpal-self-review")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if err := os.Remove(binary); err != nil && !os.IsNotExist(err) {
		_, _ = fmt.Fprintln(stderr, "self-review: cannot replace previous binary")
		return 4
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/lintpal")
	build.Dir = sourceRoot
	build.Stdout, build.Stderr = stdout, stderr
	if err := build.Run(); err != nil {
		_, _ = fmt.Fprintln(stderr, "self-review: build failed")
		return processExit(err)
	}
	args := []string{"lint", "--base", base, "--head", head, "--format", "json", "--out", report}
	for _, option := range []struct{ env, flag string }{
		{"PROVIDER", "--provider"}, {"RULES", "--rules"}, {"FAIL_ON", "--fail-on"},
	} {
		if value := getenv(option.env); value != "" {
			args = append(args, option.flag, value)
		}
	}
	command := exec.Command(binary, args...)
	command.Dir = reviewRoot
	command.Stdout, command.Stderr = stdout, stderr
	return processExit(command.Run())
}

func processExit(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 5
}
