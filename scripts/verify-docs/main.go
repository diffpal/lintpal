// Command verify-docs checks local Markdown links and rule examples offline.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

var markdownLink = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

func main() {
	if err := verify("."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("documentation links and rule examples verified")
}

func verify(root string) error {
	readme := filepath.Join(root, "README.md")
	if err := checkLinks(readme); err != nil {
		return err
	}
	if err := checkLinks(filepath.Join(root, "CONTRIBUTING.md")); err != nil {
		return err
	}
	docs := filepath.Join(root, "docs")
	if err := filepath.WalkDir(docs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		return checkLinks(path)
	}); err != nil {
		return err
	}
	examples := filepath.Join(root, "examples", "rules")
	packDirs, err := os.ReadDir(examples)
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range packDirs {
		if !entry.IsDir() {
			continue
		}
		count++
		packDir := filepath.Join(examples, entry.Name())
		if _, err := rules.LoadDirectory(context.Background(), packDir); err != nil {
			return fmt.Errorf("%s: %w", packDir, err)
		}
	}
	if count == 0 {
		return errors.New("no rule examples found in examples/rules")
	}
	return nil
}

func checkLinks(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, match := range markdownLink.FindAllSubmatch(content, -1) {
		ref := string(match[1])
		if strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "//") {
			continue
		}
		parsed, err := url.Parse(ref)
		if err != nil {
			return fmt.Errorf("%s: invalid link %q: %w", path, ref, err)
		}
		if parsed.Scheme != "" || parsed.Host != "" {
			continue
		}
		if parsed.Path == "" || filepath.IsAbs(parsed.Path) {
			return fmt.Errorf("%s: invalid local link %q", path, ref)
		}
		target := filepath.Join(filepath.Dir(path), filepath.FromSlash(parsed.Path))
		if _, err := os.Stat(target); err != nil {
			return fmt.Errorf("%s: broken link %q: %w", path, ref, err)
		}
	}
	return nil
}
