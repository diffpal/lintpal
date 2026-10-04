package report

import (
	"errors"
	"math"
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

func TestValidateReviewMetrics(t *testing.T) {
	result := git.Result{Revisions: git.Revisions{
		Base: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), MergeBase: strings.Repeat("a", 40),
	}}
	report, err := New(result, nil, "systemone", "model", Stats{})
	if err != nil {
		t.Fatal(err)
	}
	zero, positive, negative, nan, infinity := 0.0, 0.125, -0.1, math.NaN(), math.Inf(1)
	for _, tc := range []struct {
		name    string
		metrics *ReviewMetrics
		valid   bool
	}{
		{"legacy", nil, true},
		{"complete", &ReviewMetrics{RequestCount: 2, ReviewDurationMS: 12, CostUSD: &positive}, true},
		{"unknown-cost", &ReviewMetrics{RequestCount: 1}, true},
		{"zero-requests", &ReviewMetrics{CostUSD: &zero}, true},
		{"negative-requests", &ReviewMetrics{RequestCount: -1}, false},
		{"negative-duration", &ReviewMetrics{RequestCount: 1, ReviewDurationMS: -1}, false},
		{"negative-cost", &ReviewMetrics{RequestCount: 1, CostUSD: &negative}, false},
		{"nan-cost", &ReviewMetrics{RequestCount: 1, CostUSD: &nan}, false},
		{"infinite-cost", &ReviewMetrics{RequestCount: 1, CostUSD: &infinity}, false},
		{"zero-requests-unknown-cost", &ReviewMetrics{}, false},
		{"zero-requests-positive-cost", &ReviewMetrics{CostUSD: &positive}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := report
			candidate.Stats.Review = tc.metrics
			if got := Validate(candidate) == nil; got != tc.valid {
				t.Fatalf("valid = %t, want %t", got, tc.valid)
			}
		})
	}
}
