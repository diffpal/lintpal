// Command verify-packs checks installed packs when this project has a lockfile.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diffpal/lintpal/internal/apps/lintpal/packs"
)

func main() {
	count, err := verify(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("verified %d pack(s)\n", count)
}

func verify(root string) (int, error) {
	_, err := os.Lstat(filepath.Join(root, ".lintpal", "packs.lock.json"))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	entries, err := packs.VerifyMarkdown(context.Background(), root, "")
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}
