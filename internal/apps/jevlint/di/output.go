package di

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/diffpal/jevlint/internal/apps/jevlint/app"
	"github.com/diffpal/jevlint/internal/apps/jevlint/cli"
	"github.com/diffpal/jevlint/internal/apps/jevlint/report"
	adkruntime "github.com/diffpal/jevlint/internal/apps/jevlint/runtime/adk"
)

// ExecuteLint writes a complete optional artifact, then a complete stdout
// report, before applying the severity gate.
func ExecuteLint(ctx context.Context, dir string, options cli.Options, stdout, stderr io.Writer) (runErr error) {
	runtime := adkruntime.New()
	credential := selectedCredential(options)
	var writeStarted time.Time
	var diagnosticCount int
	defer func() {
		if !writeStarted.IsZero() {
			status := app.StatusOK
			if runErr != nil {
				status = app.StatusError
			}
			runtime.Observe(app.StageWrite, status, diagnosticCount, time.Since(writeStarted))
		}
		if options.Metrics && stderr != nil {
			var metrics bytes.Buffer
			for _, metric := range runtime.Snapshot() {
				_, _ = fmt.Fprintf(&metrics, "metric stage=%s status=%s count=%d duration_ms=%d\n",
					metric.Stage, metric.Status, metric.Count, metric.Duration.Milliseconds())
			}
			if credential == "" || !bytes.Contains(metrics.Bytes(), []byte(credential)) {
				_, _ = stderr.Write(metrics.Bytes())
			}
		}
	}()
	artifact, err := lintWithRuntime(ctx, dir, options, runtime)
	if err != nil {
		return err
	}
	writeStarted = time.Now()
	diagnosticCount = len(artifact.Diagnostics)
	if containsCredential(artifact, credential) {
		return fmt.Errorf("%w: unsafe report", report.ErrExport)
	}
	var preview bytes.Buffer
	switch options.Format {
	case report.JSON:
		err = report.WriteJSON(&preview, artifact)
	case report.Human:
		err = report.WriteHuman(&preview, artifact)
	default:
		return cli.ErrInvalidOptions
	}
	if err != nil {
		return err
	}
	if credential != "" && bytes.Contains(preview.Bytes(), []byte(credential)) {
		return fmt.Errorf("%w: unsafe report", report.ErrExport)
	}
	if options.Out != "" {
		output := preview.Bytes()
		if options.Format != report.JSON {
			var jsonOutput bytes.Buffer
			if err := report.WriteJSON(&jsonOutput, artifact); err != nil {
				return err
			}
			output = jsonOutput.Bytes()
		}
		if credential != "" && bytes.Contains(output, []byte(credential)) {
			return fmt.Errorf("%w: unsafe report", report.ErrExport)
		}
		if err := writeArtifact(options.Out, output); err != nil {
			return err
		}
	}
	return report.WriteAndGate(stdout, artifact, options.Format, options.FailOn)
}

// selectedCredential reads only the token source fixed by the preset or the
// custom process option. Repository rule content cannot name this variable.
func selectedCredential(options cli.Options) string {
	switch options.Provider {
	case "jev":
		return os.Getenv("TYPESAFE_API_KEY")
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY")
	case "custom":
		return os.Getenv(options.AuthTokenEnv)
	default:
		return ""
	}
}

func containsCredential(artifact report.Report, credential string) bool {
	if credential == "" {
		return false
	}
	has := func(value string) bool { return strings.Contains(value, credential) }
	if has(artifact.SchemaVersion) || has(artifact.BaseSHA) || has(artifact.HeadSHA) || has(artifact.MergeBaseSHA) {
		return true
	}
	for _, diagnostic := range artifact.Diagnostics {
		if has(diagnostic.RuleID) || has(diagnostic.WorkItemID) || has(string(diagnostic.Severity)) ||
			has(diagnostic.Path) || has(string(diagnostic.Side)) || has(diagnostic.Title) || has(diagnostic.Message) ||
			has(diagnostic.Provider) || has(diagnostic.Model) || has(diagnostic.Evidence.Kind) {
			return true
		}
	}
	for _, skip := range artifact.Skips {
		if has(skip.OldPath) || has(skip.NewPath) || has(string(skip.Reason)) {
			return true
		}
	}
	return false
}

func writeArtifact(path string, payload []byte) error {
	if existing, err := os.Stat(path); err == nil && existing.IsDir() {
		return cli.ErrInvalidOptions
	}
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".jevlint-report-*")
	if err != nil {
		return fmt.Errorf("%w: create artifact", report.ErrExport)
	}
	defer os.Remove(file.Name())
	var written int
	written, err = file.Write(payload)
	if err != nil || written != len(payload) {
		file.Close()
		return fmt.Errorf("%w: write artifact", report.ErrExport)
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("%w: sync artifact", report.ErrExport)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("%w: close artifact", report.ErrExport)
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("%w: rename artifact", report.ErrExport)
	}
	return nil
}
