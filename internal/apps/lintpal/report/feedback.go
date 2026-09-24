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
	Version      string    `json:"version"`
	ReviewID     string    `json:"review_id"`
	BaseSHA      string    `json:"base_sha"`
	HeadSHA      string    `json:"head_sha"`
	MergeBaseSHA string    `json:"merge_base_sha"`
	Findings     []Finding `json:"findings"`
	Skips        []Skip    `json:"skips"`
	Stats        Stats     `json:"stats"`
}

type Finding struct {
	ID          string `json:"id"`
	ReviewID    string `json:"review_id"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Path        string `json:"path"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	ChangedSpan struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Side      string `json:"side"`
	} `json:"changed_span"`
	Evidence struct {
		Kind   string `json:"kind"`
		RuleID string `json:"rule_id"`
		Anchor string `json:"anchor"`
	} `json:"evidence"`
	Decision struct {
		Kind  string  `json:"kind"`
		Value float64 `json:"value"`
	} `json:"decision"`
	Blocking   bool   `json:"blocking"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	WorkItemID string `json:"work_item_id"`
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

// RenderGitHubResult builds the deterministic top-level GitHub feedback body.
// unanchoredIDs contains validated findings that the GitHub transport could not
// attach to a diff line. Unknown or duplicate IDs are rejected.
func RenderGitHubResult(bundle Bundle, unanchoredIDs []string) ([]byte, error) {
	unanchored := make(map[string]struct{}, len(unanchoredIDs))
	known := make(map[string]struct{}, len(bundle.Findings))
	for _, finding := range bundle.Findings {
		known[finding.ID] = struct{}{}
	}
	for _, id := range unanchoredIDs {
		if _, ok := known[id]; !ok {
			return nil, ErrInvalidReport
		}
		if _, exists := unanchored[id]; exists {
			return nil, ErrInvalidReport
		}
		unanchored[id] = struct{}{}
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "# LintPal findings\n\n- Base: %s\n- Head: %s\n\n", markdownText(bundle.BaseSHA), markdownText(bundle.HeadSHA))
	out.WriteString("## Gate status\n\n")
	fmt.Fprintf(&out, "%s\n\n", blockingStatus(BlockingCount(bundle)))
	out.WriteString("## Publication\n\n")
	fmt.Fprintf(&out, "- Findings: %d\n- Inline: %d\n- Not attached inline: %d\n",
		len(bundle.Findings), len(bundle.Findings)-len(unanchored), len(unanchored))
	if len(unanchored) == 0 {
		return out.Bytes(), nil
	}
	out.WriteString("\n### Not attached inline\n\n")
	for _, finding := range bundle.Findings {
		if _, ok := unanchored[finding.ID]; !ok {
			continue
		}
		fmt.Fprintf(&out, "- %s — %s (%s:%d-%d, %s)\n",
			markdownText(finding.ID), markdownText(finding.Title), markdownText(finding.ChangedSpan.Path),
			finding.ChangedSpan.StartLine, finding.ChangedSpan.EndLine, markdownText(finding.ChangedSpan.Side))
	}
	return out.Bytes(), nil
}

// RenderGitHubFinding builds one deterministic inline body from validated
// finding fields. It does not add semantic analysis or remediation text.
func RenderGitHubFinding(finding Finding) []byte {
	var out bytes.Buffer
	status := "nonblocking"
	if finding.Blocking {
		status = "blocking"
	}
	fmt.Fprintf(&out, "**%s** (%s): %s\n\n%s\n", markdownText(finding.Severity), status,
		markdownText(finding.Title), markdownText(finding.Message))
	if finding.Evidence.Kind == "rule" {
		fmt.Fprintf(&out, "\nRule: %s\n", markdownText(finding.Evidence.RuleID))
	}
	return out.Bytes()
}

func BlockingCount(bundle Bundle) int {
	count := 0
	for _, finding := range bundle.Findings {
		if finding.Blocking {
			count++
		}
	}
	return count
}

func blockingStatus(count int) string {
	switch count {
	case 0:
		return "No blocking findings"
	case 1:
		return "1 blocking finding"
	default:
		return fmt.Sprintf("%d blocking findings", count)
	}
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
