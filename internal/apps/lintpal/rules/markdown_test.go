package rules

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/contextplan"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

func TestLoadDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "go"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "errors.md"), []byte("Handle returned errors.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}
	pack, err := LoadDirectory(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	rules := pack.Rules()
	if len(rules) != 1 || rules[0].ID != "go/errors.md" || rules[0].Type != "noul" ||
		rules[0].Threshold != .95 || rules[0].Severity != Medium ||
		!strings.Contains(rules[0].Instructions, "Handle returned errors.") {
		t.Fatalf("unexpected Markdown rule: %+v", rules)
	}
	rules[0].Instructions = "tampered"
	if pack.Rules()[0].Instructions == "tampered" {
		t.Fatal("pack leaked mutable rule")
	}
}

func TestLoadDirectoryRejectsUnsafeInput(t *testing.T) {
	tests := map[string][]byte{
		"empty":      {},
		"whitespace": []byte("  \n"),
		"invalid":    {0xff},
		"nul":        []byte("bad\x00rule"),
		"oversized":  []byte(strings.Repeat("x", maxMarkdownFileBytes+1)),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "rule.md"), body, 0600); err != nil {
				t.Fatal(err)
			}
			pack, err := LoadDirectory(t.Context(), root)
			if err == nil || len(pack.Rules()) != 0 {
				t.Fatalf("accepted unsafe rule: %v", err)
			}
		})
	}
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDirectory(t.Context(), root); !errors.Is(err, ErrInvalidPack) {
		t.Fatalf("symlink: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := LoadDirectory(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestCompileMandatesRejectsBadIDsAndDuplicates(t *testing.T) {
	for _, id := range []string{"../escape.md", "/absolute.md", "not-markdown.txt", "a\\b.md", strings.Repeat("x", 61) + ".md"} {
		if _, err := CompileMandates([]Mandate{{ID: id, Body: "Do this."}}); err == nil {
			t.Fatalf("accepted ID %q", id)
		}
	}
	if _, err := CompileMandates([]Mandate{{ID: "a.md", Body: "One"}, {ID: "a.md", Body: "Two"}}); err == nil {
		t.Fatal("accepted duplicate")
	}
}

func TestMarkdownPolicyAndGlobalSelection(t *testing.T) {
	pack, err := CompileMandates([]Mandate{{ID: "review/requirement.md", Body: "Changed code must handle errors."}})
	if err != nil {
		t.Fatal(err)
	}
	pack, err = WithPolicy(pack, 0.8, Critical)
	if err != nil || pack.Rules()[0].Threshold != 0.8 || pack.Rules()[0].Severity != Critical {
		t.Fatalf("policy: %+v, %v", pack.Rules(), err)
	}
	if _, err := WithPolicy(pack, -0.1, Medium); err == nil {
		t.Fatal("accepted bad policy")
	}
	groups := sampleGroups()
	selected, err := SelectWithPaths(t.Context(), pack, groups, []string{"*.go"}, []string{"old.go"})
	if err != nil || len(selected) != 1 || selected[0].Item.Path != "src/new.go" ||
		selected[0].Rule.ID != "review/requirement.md" {
		t.Fatalf("selection: %+v, %v", selected, err)
	}
	selected, err = SelectWithPaths(t.Context(), pack, groups, nil, nil)
	if err != nil || len(selected) != 3 {
		t.Fatalf("all paths: %+v, %v", selected, err)
	}
}

func TestMarkdownFrontmatterAndOverrides(t *testing.T) {
	pack, err := CompileMandates([]Mandate{
		{ID: "go/errors.md", Body: "---\nseverity: high\nthreshold: 0.8\ntitle: Handle errors\n---\nChanged code must handle errors.\n"},
		{ID: "go/cleanup.md", Body: "Changed code must clean up resources.\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := pack.Rules()
	if got[0].Severity != Medium || got[0].Threshold != .95 ||
		got[1].Severity != High || got[1].Threshold != .8 || got[1].Title != "Handle errors" ||
		strings.Contains(got[1].Instructions, "severity: high") ||
		!strings.Contains(got[1].Instructions, "Changed code must handle errors.") {
		t.Fatalf("frontmatter not separated: %+v", got)
	}
	severity := Critical
	overridden, err := WithOverrides(pack, nil, &severity)
	if err != nil || overridden.Rules()[1].Severity != Critical || overridden.Rules()[1].Threshold != .8 {
		t.Fatalf("severity override: %+v, %v", overridden.Rules(), err)
	}
	threshold := .6
	overridden, err = WithOverrides(pack, &threshold, nil)
	if err != nil || overridden.Rules()[0].Threshold != .6 || overridden.Rules()[1].Threshold != .6 || overridden.Rules()[1].Severity != High {
		t.Fatalf("threshold override: %+v, %v", overridden.Rules(), err)
	}
}

func TestMarkdownFrontmatterRejectsInvalidMetadata(t *testing.T) {
	for name, body := range map[string]string{
		"unknown":       "---\nsides: [LEFT]\n---\nRequirement.",
		"duplicate":     "---\nseverity: low\nseverity: high\n---\nRequirement.",
		"bad severity":  "---\nseverity: fatal\n---\nRequirement.",
		"bad threshold": "---\nthreshold: 1.2\n---\nRequirement.",
		"bad title":     "---\ntitle: ''\n---\nRequirement.",
		"numeric title": "---\ntitle: 123\n---\nRequirement.",
		"empty body":    "---\nseverity: high\n---\n",
		"no close":      "---\nseverity: high\nRequirement.",
		"alias":         "---\ntitle: &name Some title\nseverity: high\n---\nRequirement.",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := CompileMandates([]Mandate{{ID: "rule.md", Body: body}}); !errors.Is(err, ErrInvalidRule) {
				t.Fatalf("accepted invalid frontmatter: %v", err)
			}
		})
	}
}

func TestMarkdownDecisionUsesFixedReportFields(t *testing.T) {
	pack, err := CompileMandates([]Mandate{{ID: "review/requirement.md", Body: "Changed code must handle errors."}})
	if err != nil {
		t.Fatal(err)
	}
	pack, err = WithPolicy(pack, 0.8, Critical)
	if err != nil {
		t.Fatal(err)
	}
	selections, err := SelectWithPaths(t.Context(), pack, sampleGroups()[:1], nil, nil)
	if err != nil || len(selections) != 1 {
		t.Fatalf("select: %+v, %v", selections, err)
	}
	bindings, err := Questions(t.Context(), selections)
	if err != nil {
		t.Fatal(err)
	}
	batch := contextplan.Batch{GroupIDs: []string{selections[0].GroupID},
		Request: jev.Request{Model: "test", State: "committed code", Questions: map[string]jev.Question{
			bindings[0].QuestionID: bindings[0].Question}},
		ItemByQuestion: map[string]string{bindings[0].QuestionID: bindings[0].WorkItemID}}
	response := jev.Response{Model: "test", Answers: map[string]jev.Answer{
		bindings[0].QuestionID: jev.NoulAnswer{Probability: 0.8}}}
	decisions, err := Decide(t.Context(), batch, selections, response)
	if err != nil || len(decisions) != 1 || decisions[0].RuleID != "review/requirement.md" ||
		decisions[0].Severity != Critical || decisions[0].Title != "Possible rule violation" ||
		decisions[0].Message != "Changed code may violate review/requirement.md." ||
		decisions[0].StartLine != selections[0].Item.StartLine {
		t.Fatalf("fixed decision: %+v, %v", decisions, err)
	}
	response.Answers[bindings[0].QuestionID] = jev.NoulAnswer{Probability: 0.799}
	decisions, err = Decide(t.Context(), batch, selections, response)
	if err != nil || len(decisions) != 0 {
		t.Fatalf("below threshold: %+v, %v", decisions, err)
	}
}
