package report

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/rules"
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
	decision := rules.Decision{RuleID: "r", WorkItemID: item.ID, Path: item.Path, Side: item.Side, StartLine: 1, EndLine: 1, Severity: rules.High, Title: "title\nnext", Message: "message", Kind: "noul_probability", Value: .9}
	artifact, err := New(result, []rules.Decision{decision}, "systemone", "model", Stats{WorkItems: 1})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteAndGate(&out, artifact, JSON, High); !errors.Is(err, ErrGate) {
		t.Fatalf("gate: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"schema_version": "jevlint.report.v1"`)) || !bytes.Contains(out.Bytes(), []byte(`"rule_id": "r"`)) {
		t.Fatalf("incomplete JSON: %s", out.String())
	}
	wantJSON := fmt.Sprintf(`{
  "schema_version": "jevlint.report.v1",
  "base_sha": "%s",
  "head_sha": "%s",
  "merge_base_sha": "%s",
  "diagnostics": [
    {
      "rule_id": "r",
      "work_item_id": "%s",
      "severity": "high",
      "path": "odd\n.go",
      "side": "RIGHT",
      "start_line": 1,
      "end_line": 1,
      "title": "title\nnext",
      "message": "message",
      "provider": "systemone",
      "model": "model",
      "evidence": {
        "kind": "noul_probability",
        "value": 0.9
      }
    }
  ],
  "skips": [],
  "stats": {
    "work_items": 1,
    "skipped": 0,
    "groups": 0,
    "batches": 0,
    "questions": 0,
    "diagnostics": 1,
    "input_tokens": 0,
    "output_tokens": 0
  }
}
`, strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("a", 40), strings.Repeat("1", 64))
	if out.String() != wantJSON {
		t.Fatalf("JSON golden mismatch:\n%s", out.String())
	}
	out.Reset()
	if err := WriteAndGate(&out, artifact, Human, Critical); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "odd\n.go") || strings.Contains(out.String(), "title\nnext") {
		t.Fatalf("unsafe human lines: %q", out.String())
	}
	if !strings.Contains(out.String(), `"odd\n.go"`) {
		t.Fatalf("missing escaped path: %q", out.String())
	}
	wantHuman := fmt.Sprintf("jevlint jevlint.report.v1 base=%s head=%s merge_base=%s\nhigh RIGHT \"odd\\n.go\":1-1 rule=\"r\" title=\"title\\nnext\" message=\"message\" evidence=noul_probability:0.9 provider=\"systemone\" model=\"model\"\nstats work_items=1 skipped=0 groups=0 batches=0 questions=0 diagnostics=1 input_tokens=0 output_tokens=0\n", strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("a", 40))
	if out.String() != wantHuman {
		t.Fatalf("human golden mismatch:\n%s", out.String())
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
