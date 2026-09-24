package githubfeedback

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
)

var ErrInvalidFinding = errors.New("invalid GitHub inline finding")

type Comment struct {
	FindingID string
	Body      string
	Path      string
	StartLine int
	EndLine   int
	Side      string
	Digest    string
}

type Plan struct {
	Comments      []Comment
	SkippedIDs    []string
	UnanchoredIDs []string
}

func PlanComments(bundle report.Bundle, activeFindings map[string]string) Plan {
	plan := Plan{Comments: make([]Comment, 0, len(bundle.Findings))}
	for _, finding := range bundle.Findings {
		span := finding.ChangedSpan
		if finding.ID == "" || span.Path == "" || span.StartLine < 1 || span.EndLine < span.StartLine ||
			(span.Side != "LEFT" && span.Side != "RIGHT") {
			plan.UnanchoredIDs = append(plan.UnanchoredIDs, finding.ID)
			continue
		}
		body := string(report.RenderGitHubFinding(finding))
		digest := commentDigest(finding.ID, body, span.Path, span.StartLine, span.EndLine, span.Side)
		if activeFindings[finding.ID] == digest {
			plan.SkippedIDs = append(plan.SkippedIDs, finding.ID)
			continue
		}
		plan.Comments = append(plan.Comments, Comment{FindingID: finding.ID, Body: body,
			Path: span.Path, StartLine: span.StartLine, EndLine: span.EndLine, Side: span.Side, Digest: digest})
	}
	return plan
}

func commentDigest(id, body, path string, startLine, endLine int, side string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%s%d:%s%d:%s:%d:%d:%s", len(id), id, len(body), body, len(path), path, startLine, endLine, side))))
}
