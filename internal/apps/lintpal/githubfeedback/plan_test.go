package githubfeedback

import (
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
)

func TestPlanCommentsPreservesLocationAndDeduplicates(t *testing.T) {
	left := report.Finding{ID: "left-id", Severity: "high", Title: "Left", Message: "fixed", Blocking: true}
	left.ChangedSpan.Path, left.ChangedSpan.StartLine, left.ChangedSpan.EndLine, left.ChangedSpan.Side = "old.go", 3, 5, "LEFT"
	left.Evidence.Kind, left.Evidence.RuleID = "rule", "go/errors.md"
	right := left
	right.ID = "right-id"
	right.ChangedSpan.Path, right.ChangedSpan.StartLine, right.ChangedSpan.EndLine, right.ChangedSpan.Side = "new.go", 8, 8, "RIGHT"
	invalid := left
	invalid.ID, invalid.ChangedSpan.Path = "invalid-id", ""
	rightBody := string(report.RenderGitHubFinding(right))
	rightDigest := commentDigest(right.ID, rightBody, right.ChangedSpan.Path, right.ChangedSpan.StartLine, right.ChangedSpan.EndLine, right.ChangedSpan.Side)
	plan := PlanComments(report.Bundle{Findings: []report.Finding{left, right, invalid}}, map[string]string{"right-id": rightDigest})
	if len(plan.Comments) != 1 || plan.Comments[0].Side != "LEFT" || plan.Comments[0].StartLine != 3 || plan.Comments[0].EndLine != 5 ||
		len(plan.SkippedIDs) != 1 || plan.SkippedIDs[0] != "right-id" || len(plan.UnanchoredIDs) != 1 || plan.UnanchoredIDs[0] != "invalid-id" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if !strings.Contains(plan.Comments[0].Body, "go/errors") {
		t.Fatalf("missing stored rule body: %s", plan.Comments[0].Body)
	}
}

func TestIdentityValidationAndMarkers(t *testing.T) {
	identity, err := NewIdentity(" Team.One ")
	if err != nil || identity.Channel != "team.one" {
		t.Fatalf("unexpected identity: %+v %v", identity, err)
	}
	if !strings.Contains(identity.ResultMarker("abc"), "head_sha:abc") || !strings.Contains(identity.FindingMarker("finding-1", "digest"), "id:finding-1 digest:digest") {
		t.Fatal("missing marker fields")
	}
	if _, err := NewIdentity("-bad"); err != ErrInvalidChannel {
		t.Fatalf("invalid channel accepted: %v", err)
	}
}

func TestPlanCommentsRepublishesChangedBodyAtSameFindingID(t *testing.T) {
	finding := report.Finding{ID: "stable-id", Severity: "high", Title: "Changed", Message: "new fixed message"}
	finding.ChangedSpan.Path, finding.ChangedSpan.StartLine, finding.ChangedSpan.EndLine, finding.ChangedSpan.Side = "same.go", 7, 7, "RIGHT"
	plan := PlanComments(report.Bundle{Findings: []report.Finding{finding}}, map[string]string{"stable-id": "old-digest"})
	if len(plan.Comments) != 1 || len(plan.SkippedIDs) != 0 || plan.Comments[0].Digest == "old-digest" {
		t.Fatalf("changed finding was not republished: %+v", plan)
	}
}
