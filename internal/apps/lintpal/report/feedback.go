package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/diffpal/lintpal/docs/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const MaxBundleBytes = 64 << 20

type Bundle struct {
	Version  string    `json:"version"`
	ReviewID string    `json:"review_id"`
	BaseSHA  string    `json:"base_sha"`
	HeadSHA  string    `json:"head_sha"`
	Findings []Finding `json:"findings"`
}

type Finding struct {
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Path        string `json:"path"`
	ChangedSpan struct {
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Side      string `json:"side"`
	} `json:"changed_span"`
	Evidence struct {
		Kind   string `json:"kind"`
		RuleID string `json:"rule_id"`
		Anchor string `json:"anchor"`
	} `json:"evidence"`
	Blocking bool `json:"blocking"`
}

var (
	bundleSchemaOnce sync.Once
	bundleSchema     *jsonschema.Schema
	bundleSchemaErr  error
)

func compiledBundleSchema() (*jsonschema.Schema, error) {
	bundleSchemaOnce.Do(func() {
		var document any
		if err := json.Unmarshal(schema.FindingsV5, &document); err != nil {
			bundleSchemaErr = err
			return
		}
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource("v5.schema.json", document); err != nil {
			bundleSchemaErr = err
			return
		}
		bundleSchema, bundleSchemaErr = compiler.Compile("v5.schema.json")
	})
	return bundleSchema, bundleSchemaErr
}

// ParseBundle validates a shared findings v5 artifact before using its fields.
func ParseBundle(data []byte) (Bundle, error) {
	if len(data) == 0 || len(data) > MaxBundleBytes || !utf8.Valid(data) || !json.Valid(data) {
		return Bundle{}, ErrInvalidReport
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return Bundle{}, ErrInvalidReport
	}
	compiled, err := compiledBundleSchema()
	if err != nil || compiled.Validate(document) != nil {
		return Bundle{}, ErrInvalidReport
	}
	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return Bundle{}, ErrInvalidReport
	}
	return bundle, nil
}

// ReadBundle bounds a stored report before parsing it. Errors contain no report text.
func ReadBundle(path string) (Bundle, error) {
	file, err := os.Open(path)
	if err != nil {
		return Bundle{}, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, MaxBundleBytes+1))
	if err != nil {
		return Bundle{}, ErrInvalidReport
	}
	return ParseBundle(data)
}

// BundleFromReport converts an internal report through its public v5 shape.
func BundleFromReport(artifact Report) (Bundle, error) {
	data, err := json.Marshal(artifact)
	if err != nil {
		return Bundle{}, ErrInvalidReport
	}
	return ParseBundle(data)
}

func WriteMarkdown(writer io.Writer, artifact Report) error {
	bundle, err := BundleFromReport(artifact)
	if err != nil {
		return err
	}
	return writeComplete(writer, RenderMarkdown(bundle))
}

// RenderMarkdown builds complete feedback from a validated findings bundle.
func RenderMarkdown(bundle Bundle) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "# LintPal findings\n\n- Base: %s\n- Head: %s\n\n", markdownText(bundle.BaseSHA), markdownText(bundle.HeadSHA))
	if len(bundle.Findings) == 0 {
		out.WriteString("No findings.\n")
		return out.Bytes()
	}
	fmt.Fprintf(&out, "%d finding(s).\n\n", len(bundle.Findings))
	for _, finding := range bundle.Findings {
		status := "nonblocking"
		if finding.Blocking {
			status = "blocking"
		}
		fmt.Fprintf(&out, "- **%s** (%s): %s\n", markdownText(finding.Severity), status, markdownText(finding.Title))
		fmt.Fprintf(&out, "  - Location: %s:%d-%d (%s)\n", markdownText(finding.Path), finding.ChangedSpan.StartLine, finding.ChangedSpan.EndLine, markdownText(finding.ChangedSpan.Side))
		if finding.Evidence.Kind == "rule" {
			fmt.Fprintf(&out, "  - Rule: %s\n", markdownText(finding.Evidence.RuleID))
		} else {
			fmt.Fprintf(&out, "  - Evidence: %s\n", markdownText(finding.Evidence.Anchor))
		}
		fmt.Fprintf(&out, "  - Message: %s\n", markdownText(finding.Message))
	}
	return out.Bytes()
}

func markdownText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = html.EscapeString(value)
	return strings.NewReplacer(
		"\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]",
		"(", "\\(", ")", "\\)", "#", "\\#", "+", "\\+", "-", "\\-", "!", "\\!", "|", "\\|",
	).Replace(value)
}

func BundleBlocks(bundle Bundle) bool {
	for _, finding := range bundle.Findings {
		if finding.Blocking {
			return true
		}
	}
	return false
}
