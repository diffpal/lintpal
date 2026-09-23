// Package report defines the validated, deterministic jevlint.report.v1 artifact.
package report

import (
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/rules"
)

const SchemaVersion = "jevlint.report.v1"

var ErrInvalidReport = errors.New("invalid lint report")

type Evidence struct {
	Kind       string   `json:"kind"`
	Value      float64  `json:"value"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type Diagnostic struct {
	RuleID     string         `json:"rule_id"`
	WorkItemID string         `json:"work_item_id"`
	Severity   rules.Severity `json:"severity"`
	Path       string         `json:"path"`
	Side       git.Side       `json:"side"`
	StartLine  int            `json:"start_line"`
	EndLine    int            `json:"end_line"`
	Title      string         `json:"title"`
	Message    string         `json:"message"`
	Provider   string         `json:"provider"`
	Model      string         `json:"model"`
	Evidence   Evidence       `json:"evidence"`
}

type Skip struct {
	OldPath string         `json:"old_path,omitempty"`
	NewPath string         `json:"new_path,omitempty"`
	Reason  git.SkipReason `json:"reason"`
}

type Stats struct {
	WorkItems    int `json:"work_items"`
	Skipped      int `json:"skipped"`
	Groups       int `json:"groups"`
	Batches      int `json:"batches"`
	Questions    int `json:"questions"`
	Diagnostics  int `json:"diagnostics"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Report struct {
	SchemaVersion string       `json:"schema_version"`
	BaseSHA       string       `json:"base_sha"`
	HeadSHA       string       `json:"head_sha"`
	MergeBaseSHA  string       `json:"merge_base_sha"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Skips         []Skip       `json:"skips"`
	Stats         Stats        `json:"stats"`
}

// New checks every decision against the original changed-line work items.
func New(result git.Result, decisions []rules.Decision, provider, model string, stats Stats) (Report, error) {
	if !commitID(result.Revisions.Base) || !commitID(result.Revisions.Head) ||
		!commitID(result.Revisions.MergeBase) || !safeName(provider) || !safeName(model) ||
		stats.WorkItems != len(result.Items) || stats.Skipped != len(result.Skips) ||
		stats.Groups < 0 || stats.Batches < 0 || stats.Questions < 0 ||
		stats.InputTokens < 0 || stats.OutputTokens < 0 {
		return Report{}, ErrInvalidReport
	}
	items := make(map[string]git.WorkItem, len(result.Items))
	for _, item := range result.Items {
		if item.ID == "" || item.Path == "" || item.StartLine < 1 || item.EndLine < item.StartLine ||
			(item.Side != git.Left && item.Side != git.Right) {
			return Report{}, ErrInvalidReport
		}
		if _, exists := items[item.ID]; exists {
			return Report{}, ErrInvalidReport
		}
		items[item.ID] = item
	}
	report := Report{SchemaVersion: SchemaVersion, BaseSHA: result.Revisions.Base,
		HeadSHA: result.Revisions.Head, MergeBaseSHA: result.Revisions.MergeBase,
		Diagnostics: make([]Diagnostic, 0, len(decisions)), Skips: make([]Skip, 0, len(result.Skips))}
	seen := make(map[string]bool, len(decisions))
	for _, decision := range decisions {
		item, ok := items[decision.WorkItemID]
		key := decision.WorkItemID + "/" + decision.RuleID
		if !ok || seen[key] || decision.Path != item.Path || decision.Side != item.Side ||
			decision.StartLine != item.StartLine || decision.EndLine != item.EndLine ||
			strings.TrimSpace(decision.RuleID) == "" || strings.TrimSpace(decision.Title) == "" ||
			strings.TrimSpace(decision.Message) == "" || !validSeverity(decision.Severity) ||
			!validEvidence(decision.Kind, decision.Value, decision.Confidence) {
			return Report{}, ErrInvalidReport
		}
		seen[key] = true
		var confidence *float64
		if decision.Confidence != nil {
			copy := *decision.Confidence
			confidence = &copy
		}
		report.Diagnostics = append(report.Diagnostics, Diagnostic{RuleID: decision.RuleID,
			WorkItemID: item.ID, Severity: decision.Severity, Path: item.Path, Side: item.Side,
			StartLine: item.StartLine, EndLine: item.EndLine, Title: decision.Title,
			Message: decision.Message, Provider: provider, Model: model,
			Evidence: Evidence{Kind: decision.Kind, Value: decision.Value, Confidence: confidence}})
	}
	for _, skip := range result.Skips {
		if (skip.Reason != git.SkipBinary && skip.Reason != git.SkipNonRegular && skip.Reason != git.SkipNoLines) ||
			(skip.OldPath == "" && skip.NewPath == "") {
			return Report{}, ErrInvalidReport
		}
		report.Skips = append(report.Skips, Skip{OldPath: skip.OldPath, NewPath: skip.NewPath, Reason: skip.Reason})
	}
	sort.Slice(report.Diagnostics, func(i, j int) bool {
		return diagnosticLess(report.Diagnostics[i], report.Diagnostics[j])
	})
	sort.Slice(report.Skips, func(i, j int) bool {
		return skipLess(report.Skips[i], report.Skips[j])
	})
	stats.Diagnostics = len(report.Diagnostics)
	report.Stats = stats
	if err := Validate(report); err != nil {
		return Report{}, err
	}
	return report, nil
}

// Validate checks the public artifact before it is rendered or gated.
// Exact changed-line provenance is checked by New, which has the Git work items.
func Validate(report Report) error {
	if report.SchemaVersion != SchemaVersion || !commitID(report.BaseSHA) ||
		!commitID(report.HeadSHA) || !commitID(report.MergeBaseSHA) ||
		report.Diagnostics == nil || report.Skips == nil ||
		report.Stats.WorkItems < 0 || report.Stats.Skipped != len(report.Skips) ||
		report.Stats.Groups < 0 || report.Stats.Batches < 0 || report.Stats.Questions < 0 ||
		report.Stats.Diagnostics != len(report.Diagnostics) ||
		report.Stats.InputTokens < 0 || report.Stats.OutputTokens < 0 {
		return ErrInvalidReport
	}
	seen := make(map[string]bool, len(report.Diagnostics))
	for i, d := range report.Diagnostics {
		key := d.WorkItemID + "/" + d.RuleID
		if seen[key] || d.RuleID == "" || d.WorkItemID == "" || d.Path == "" ||
			(d.Side != git.Left && d.Side != git.Right) || d.StartLine < 1 || d.EndLine < d.StartLine ||
			strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Message) == "" ||
			!safeName(d.Provider) || !safeName(d.Model) || !validSeverity(d.Severity) ||
			!validEvidence(d.Evidence.Kind, d.Evidence.Value, d.Evidence.Confidence) {
			return ErrInvalidReport
		}
		if i > 0 && diagnosticLess(d, report.Diagnostics[i-1]) {
			return ErrInvalidReport
		}
		seen[key] = true
	}
	for i, skip := range report.Skips {
		if skip.OldPath == "" && skip.NewPath == "" ||
			(skip.Reason != git.SkipBinary && skip.Reason != git.SkipNonRegular && skip.Reason != git.SkipNoLines) {
			return ErrInvalidReport
		}
		if i > 0 && skipLess(skip, report.Skips[i-1]) {
			return ErrInvalidReport
		}
	}
	return nil
}

func diagnosticLess(a, b Diagnostic) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Side != b.Side {
		return a.Side < b.Side
	}
	if a.StartLine != b.StartLine {
		return a.StartLine < b.StartLine
	}
	if a.EndLine != b.EndLine {
		return a.EndLine < b.EndLine
	}
	if a.RuleID != b.RuleID {
		return a.RuleID < b.RuleID
	}
	return a.WorkItemID < b.WorkItemID
}

func skipLess(a, b Skip) bool {
	if a.OldPath != b.OldPath {
		return a.OldPath < b.OldPath
	}
	if a.NewPath != b.NewPath {
		return a.NewPath < b.NewPath
	}
	return a.Reason < b.Reason
}

func validSeverity(value rules.Severity) bool {
	return value == rules.Low || value == rules.Medium || value == rules.High || value == rules.Critical
}

func validEvidence(kind string, value float64, confidence *float64) bool {
	if kind != "noul_probability" && kind != "selected_probability" && kind != "normalized_score" {
		return false
	}
	if !finite(value) || value < 0 || value > 1 {
		return false
	}
	return confidence == nil || finite(*confidence) && *confidence >= 0 && *confidence <= 1
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func commitID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func safeName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || strings.ContainsRune("._~/-:@", char) {
			continue
		}
		return false
	}
	return true
}
