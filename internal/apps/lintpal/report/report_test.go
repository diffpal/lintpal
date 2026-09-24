package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

func TestNewAnchorsAndSort(t *testing.T) {
	item := git.WorkItem{ID: strings.Repeat("1", 64), Path: "a.go", NewPath: "a.go", Side: git.Right, StartLine: 2, EndLine: 3, Hunk: 1}
	result := git.Result{Revisions: git.Revisions{Base: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), MergeBase: strings.Repeat("a", 40)}, Items: []git.WorkItem{item}}
	decision := rules.Decision{RuleID: "r.md", WorkItemID: item.ID, Path: item.Path, Side: item.Side, StartLine: 2, EndLine: 3, Severity: rules.High, Title: "Title", Message: "Message", Kind: "noul_probability", Value: .8}
	report, err := New(result, []rules.Decision{decision}, "systemone", "model", Stats{WorkItems: 1})
	if err != nil || report.SchemaVersion != SchemaVersion || report.Stats.Diagnostics != 1 {
		t.Fatalf("report: %+v, %v", report, err)
	}
	decision.StartLine = 4
	if _, err := New(result, []rules.Decision{decision}, "systemone", "model", Stats{WorkItems: 1}); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("forged anchor: %v", err)
	}
	decision.StartLine = 2
	if _, err := New(result, []rules.Decision{decision, decision}, "systemone", "model", Stats{WorkItems: 1}); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("duplicate: %v", err)
	}
	report.SchemaVersion = "unknown"
	if !errors.Is(Validate(report), ErrInvalidReport) {
		t.Fatal("invalid version accepted")
	}
}
