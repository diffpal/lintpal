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
