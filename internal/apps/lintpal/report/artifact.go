package report

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteArtifact replaces a report file only after the complete payload is synced.
func WriteArtifact(path string, payload []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".lintpal-report-*")
	if err != nil {
		return fmt.Errorf("%w: create artifact", ErrExport)
	}
	defer func() { _ = os.Remove(file.Name()) }()
	written, err := file.Write(payload)
	if err != nil || written != len(payload) {
		_ = file.Close()
		return fmt.Errorf("%w: write artifact", ErrExport)
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("%w: sync artifact", ErrExport)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("%w: close artifact", ErrExport)
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("%w: rename artifact", ErrExport)
	}
	return nil
}
