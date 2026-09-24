package di

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/app"
	"github.com/diffpal/lintpal/internal/apps/lintpal/cli"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	adkruntime "github.com/diffpal/lintpal/internal/apps/lintpal/runtime/adk"
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
	artifact = report.WithGate(artifact, options.FailOn)
	writeStarted = time.Now()
	diagnosticCount = len(artifact.Diagnostics)
	if containsCredential(artifact, credential) {
		return fmt.Errorf("%w: unsafe report", report.ErrExport)
	}
	var preview bytes.Buffer
	switch options.Format {
	case report.JSON:
		err = report.WriteJSON(&preview, artifact)
	case report.Markdown:
		err = report.WriteMarkdown(&preview, artifact)
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
	if !options.Gate {
		if stdout == nil {
			return report.ErrExport
		}
		written, writeErr := stdout.Write(preview.Bytes())
		if writeErr != nil || written != preview.Len() {
			return fmt.Errorf("%w: write stdout", report.ErrExport)
		}
		return nil
	}
	return report.WriteAndGate(stdout, artifact, options.Format, options.FailOn)
}

// selectedCredential uses the exact run credential passed to the provider.
// Legacy callers without a bound credential use the trusted endpoint's env key.
func selectedCredential(options cli.Options) string {
	if options.CredentialResolved {
		return options.Credential
	}
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
	if report.Validate(artifact) == nil {
		payload, err := json.Marshal(artifact)
		return err != nil || bytes.Contains(payload, []byte(credential))
	}
	// Partial reports appear in focused guard tests; scan their source fields too.
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
	return report.WriteArtifact(path, payload)
}
