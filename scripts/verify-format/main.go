// Command verify-format checks Go formatting without a platform-specific shell.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := verify("."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Go files formatted")
}

func verify(root string) error {
	var unformatted []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".beads" || entry.Name() == ".artifacts" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		formatted, err := format.Source(original)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if !bytes.Equal(original, formatted) {
			unformatted = append(unformatted, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(unformatted) != 0 {
		return fmt.Errorf("unformatted Go files: %s", strings.Join(unformatted, ", "))
	}
	return nil
}
