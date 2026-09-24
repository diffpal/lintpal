package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

// The internal diagnostic keeps the validated rule decision and Git work item.
// MarshalJSON writes the common v5 findings contract used by both CLIs.
type wireBundle struct {
	Version      string        `json:"version"`
	ReviewID     string        `json:"review_id"`
	BaseSHA      string        `json:"base_sha"`
	HeadSHA      string        `json:"head_sha"`
	MergeBaseSHA string        `json:"merge_base_sha"`
	Findings     []wireFinding `json:"findings"`
	Skips        []Skip        `json:"skips"`
	Stats        Stats         `json:"stats"`
}

type wireSpan struct {
	Path      string   `json:"path"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
	Side      git.Side `json:"side"`
}

type wireEvidence struct {
	Kind   string `json:"kind"`
	RuleID string `json:"rule_id"`
}

type wireDecision struct {
	Kind  string  `json:"kind"`
	Value float64 `json:"value"`
}

type wireFinding struct {
	ID          string         `json:"id"`
	ReviewID    string         `json:"review_id"`
	Category    string         `json:"category"`
	Severity    rules.Severity `json:"severity"`
	Path        string         `json:"path"`
	StartLine   int            `json:"start_line"`
	EndLine     int            `json:"end_line"`
	ChangedSpan wireSpan       `json:"changed_span"`
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	Evidence    wireEvidence   `json:"evidence"`
	Decision    wireDecision   `json:"decision"`
	Blocking    bool           `json:"blocking"`
	Provider    string         `json:"provider"`
	Model       string         `json:"model,omitempty"`
	WorkItemID  string         `json:"work_item_id,omitempty"`
}

func (artifact Report) MarshalJSON() ([]byte, error) {
	if err := Validate(artifact); err != nil {
		return nil, err
	}
	reviewID := stableDigest("lintpal", artifact.BaseSHA, artifact.HeadSHA, artifact.MergeBaseSHA, artifact.identitySalt)
	bundle := wireBundle{Version: SchemaVersion, ReviewID: reviewID, BaseSHA: artifact.BaseSHA,
		HeadSHA: artifact.HeadSHA, MergeBaseSHA: artifact.MergeBaseSHA,
		Findings: make([]wireFinding, 0, len(artifact.Diagnostics)), Skips: artifact.Skips, Stats: artifact.Stats}
	for _, d := range artifact.Diagnostics {
		bundle.Findings = append(bundle.Findings, wireFinding{
			ID: stableDigest(reviewID, d.WorkItemID, d.RuleID, d.Path, string(d.Side),
				fmt.Sprint(d.StartLine), fmt.Sprint(d.EndLine)),
			ReviewID: reviewID, Category: "rule_violation", Severity: d.Severity,
			Path: d.Path, StartLine: d.StartLine, EndLine: d.EndLine,
			ChangedSpan: wireSpan{Path: d.Path, StartLine: d.StartLine, EndLine: d.EndLine, Side: d.Side},
			Title:       d.Title, Message: d.Message,
			Evidence: wireEvidence{Kind: "rule", RuleID: d.RuleID},
			Decision: wireDecision{Kind: d.Evidence.Kind, Value: d.Evidence.Value},
			Blocking: meetsSeverity(d.Severity, artifact.gate), Provider: d.Provider,
			Model: d.Model, WorkItemID: d.WorkItemID,
		})
	}
	return json.Marshal(bundle)
}

func stableDigest(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = fmt.Fprintf(h, "%d:%s", len(part), part)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func meetsSeverity(severity rules.Severity, threshold Threshold) bool {
	if threshold == None || threshold == "" {
		return false
	}
	minimum, err := rank(threshold)
	if err != nil {
		return false
	}
	level, _ := rank(Threshold(severity))
	return level >= minimum
}
