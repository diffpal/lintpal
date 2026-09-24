package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndRenderSharedFindings(t *testing.T) {
	for _, name := range []string{"lintpal-right.json", "diffpal-left.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "schema", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		bundle, err := ParseBundle(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		output := string(RenderMarkdown(bundle))
		if !strings.Contains(output, "# LintPal findings") || !strings.Contains(output, bundle.Findings[0].ChangedSpan.Side) || !strings.Contains(output, bundle.Findings[0].Message) {
			t.Fatalf("missing shared finding content for %s: %s", name, output)
		}
		if BundleBlocks(bundle) != bundle.Findings[0].Blocking {
			t.Fatalf("wrong gate for %s", name)
		}
		finding := bundle.Findings[0]
		if finding.ID == "" || finding.ReviewID == "" || finding.Category == "" ||
			finding.ChangedSpan.Path == "" || finding.Provider == "" {
			t.Fatalf("missing publication projection for %s: %+v", name, finding)
		}
	}
}

func TestParseBundleRejectsInvalidAndEscapesText(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "schema", "testdata", "lintpal-right.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	findings := document["findings"].([]any)
	finding := findings[0].(map[string]any)
	delete(finding, "blocking")
	missingBlocking, _ := json.Marshal(document)
	if _, err := ParseBundle(missingBlocking); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("missing blocking accepted: %v", err)
	}
	finding["blocking"] = true
	finding["title"] = "unsafe\n# heading [link](https://example.com)"
	finding["path"] = "a`*b.go"
	finding["message"] = "<script>alert(1)</script>"
	unsafeInput, _ := json.Marshal(document)
	bundle, err := ParseBundle(unsafeInput)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderMarkdown(bundle)
	if bytes.Contains(output, []byte("\n# heading")) || bytes.Contains(output, []byte("<script>")) ||
		!bytes.Contains(output, []byte("\\[link\\]")) || !bytes.Contains(output, []byte("a\\`\\*b.go")) || !BundleBlocks(bundle) {
		t.Fatalf("unsafe Markdown: %s", output)
	}
	if _, err := ParseBundle(bytes.Repeat([]byte(" "), MaxBundleBytes+1)); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("oversized bundle accepted: %v", err)
	}
}

func TestEmptyBundleRendersNoFindings(t *testing.T) {
	bundle := Bundle{BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40), Findings: []Finding{}}
	if got := string(RenderMarkdown(bundle)); !strings.Contains(got, "No findings.") || BundleBlocks(bundle) {
		t.Fatalf("empty feedback: %s", got)
	}
}

func TestRenderGitHubResultBlockingStatus(t *testing.T) {
	base := Finding{ID: "finding-1", Title: "Rule finding"}
	base.ChangedSpan.Path = "internal/example.go"
	base.ChangedSpan.StartLine = 12
	base.ChangedSpan.EndLine = 12
	base.ChangedSpan.Side = "RIGHT"

	tests := []struct {
		name     string
		findings []Finding
		want     string
	}{
		{name: "zero", findings: []Finding{base}, want: "No blocking findings"},
		{name: "one", findings: []Finding{func() Finding { finding := base; finding.Blocking = true; return finding }()}, want: "1 blocking finding"},
		{name: "many", findings: func() []Finding {
			first := base
			first.Blocking = true
			second := base
			second.ID = "finding-2"
			second.Blocking = true
			return []Finding{first, second}
		}(), want: "2 blocking findings"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := RenderGitHubResult(Bundle{BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40), Findings: test.findings}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(output), test.want) {
				t.Fatalf("missing %q in:\n%s", test.want, output)
			}
		})
	}
}

func TestRenderGitHubResultIdentifiesUnanchoredFindings(t *testing.T) {
	finding := Finding{ID: "finding-1", Title: "unsafe [title](https://example.com)"}
	finding.ChangedSpan.Path = "a`*b.go"
	finding.ChangedSpan.StartLine = 4
	finding.ChangedSpan.EndLine = 6
	finding.ChangedSpan.Side = "LEFT"
	bundle := Bundle{BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40), Findings: []Finding{finding}}
	output, err := RenderGitHubResult(bundle, []string{finding.ID})
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	if !strings.Contains(text, "Not attached inline: 1") || !strings.Contains(text, "finding\\-1") ||
		!strings.Contains(text, "unsafe \\[title\\]") || strings.Contains(text, "](https://example.com)") {
		t.Fatalf("unexpected GitHub result:\n%s", text)
	}
	if _, err := RenderGitHubResult(bundle, []string{"unknown"}); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("unknown ID accepted: %v", err)
	}
	if _, err := RenderGitHubResult(bundle, []string{finding.ID, finding.ID}); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("duplicate ID accepted: %v", err)
	}
}

func TestRenderGitHubFindingUsesOnlyStoredFields(t *testing.T) {
	finding := Finding{Severity: "high", Title: "Check <title>", Message: "fixed *message*", Blocking: true}
	finding.Evidence.Kind = "rule"
	finding.Evidence.RuleID = "go/errors`unsafe.md"
	output := string(RenderGitHubFinding(finding))
	for _, want := range []string{"**high** (blocking)", "Check &lt;title&gt;", "fixed \\*message\\*", "go/errors"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in:\n%s", want, output)
		}
	}
	if strings.Contains(output, "suggest") || strings.Contains(output, "impact") {
		t.Fatalf("invented prose in:\n%s", output)
	}
}
