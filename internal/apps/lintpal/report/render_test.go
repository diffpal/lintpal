package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

type shortWriter struct{ seen []byte }

func (w *shortWriter) Write(data []byte) (int, error) {
	w.seen = append(w.seen, data[:1]...)
	return 1, nil
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWriteAndGate(t *testing.T) {
	item := git.WorkItem{ID: strings.Repeat("1", 64), Path: "odd\n.go", NewPath: "odd\n.go", Side: git.Right, StartLine: 1, EndLine: 1, Hunk: 1}
	result := git.Result{Revisions: git.Revisions{Base: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), MergeBase: strings.Repeat("a", 40)}, Items: []git.WorkItem{item}}
	decision := rules.Decision{RuleID: "r.md", WorkItemID: item.ID, Path: item.Path, Side: item.Side, StartLine: 1, EndLine: 1, Severity: rules.High, Title: "title\nnext", Message: "message", Kind: "noul_probability", Value: .9}
	artifact, err := New(result, []rules.Decision{decision}, "systemone", "model", Stats{WorkItems: 1})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteAndGate(&out, artifact, JSON, High); !errors.Is(err, ErrGate) {
		t.Fatalf("gate: %v", err)
	}
	var wire struct {
		Version  string `json:"version"`
		ReviewID string `json:"review_id"`
		Findings []struct {
			ID          string `json:"id"`
			ReviewID    string `json:"review_id"`
			ChangedSpan struct {
				Side string `json:"side"`
			} `json:"changed_span"`
			Evidence struct {
				Kind   string `json:"kind"`
				RuleID string `json:"rule_id"`
			} `json:"evidence"`
			Decision struct {
				Value float64 `json:"value"`
			} `json:"decision"`
			Blocking bool `json:"blocking"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &wire); err != nil || wire.Version != "v5" || len(wire.Findings) != 1 ||
		wire.ReviewID == "" || wire.Findings[0].ReviewID != wire.ReviewID || wire.Findings[0].ID == "" ||
		wire.Findings[0].ChangedSpan.Side != "RIGHT" || wire.Findings[0].Evidence.Kind != "rule" ||
		wire.Findings[0].Evidence.RuleID != "r.md" || wire.Findings[0].Decision.Value != .9 || !wire.Findings[0].Blocking ||
		bytes.Contains(out.Bytes(), []byte(`"diagnostics": [`)) {
		t.Fatalf("unexpected v5 JSON: %s, %v", out.String(), err)
	}
	out.Reset()
	if err := WriteAndGate(&out, artifact, Markdown, Critical); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "odd\n.go") || strings.Contains(out.String(), "title\nnext") {
		t.Fatalf("unsafe markdown lines: %q", out.String())
	}
	if !strings.Contains(out.String(), "odd .go") {
		t.Fatalf("missing escaped path: %q", out.String())
	}
	if !strings.Contains(out.String(), "# LintPal findings") || !strings.Contains(out.String(), "Rule: r.md") || !strings.Contains(out.String(), "(RIGHT)") {
		t.Fatalf("markdown missing finding fields:\n%s", out.String())
	}
	var short shortWriter
	if err := WriteAndGate(&short, artifact, JSON, Low); !errors.Is(err, ErrExport) {
		t.Fatalf("short write: %v", err)
	}
	if err := WriteAndGate(failedWriter{}, artifact, JSON, Low); !errors.Is(err, ErrExport) {
		t.Fatalf("write failure: %v", err)
	}
	if err := WriteAndGate(&out, artifact, JSON, None); err != nil {
		t.Fatalf("none gate: %v", err)
	}
	for _, test := range []struct {
		threshold Threshold
		gate      bool
	}{
		{Low, true}, {Medium, true}, {High, true}, {Critical, false}, {None, false},
	} {
		gate, err := MeetsGate(artifact, test.threshold)
		if err != nil || gate != test.gate {
			t.Fatalf("threshold %s: %v, %v", test.threshold, gate, err)
		}
	}
}
