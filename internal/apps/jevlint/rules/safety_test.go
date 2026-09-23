package rules

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/diffpal/jevlint/internal/apps/jevlint/contextplan"
	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
)

func TestLoadAndSelectionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	pack, err := LoadContext(ctx, strings.NewReader(minimalPack))
	if !errors.Is(err, context.Canceled) || len(pack.Rules()) != 0 {
		t.Fatalf("load cancel: %v", err)
	}
	selected, err := Select(ctx, BuiltIn(), sampleGroups())
	if !errors.Is(err, context.Canceled) || len(selected) != 0 {
		t.Fatalf("select cancel: %v", err)
	}
	bindings, err := Questions(ctx, nil)
	if !errors.Is(err, context.Canceled) || len(bindings) != 0 {
		t.Fatalf("questions cancel: %v", err)
	}
}

func TestBuiltInRepresentativeSafeAndBuggyAnswers(t *testing.T) {
	item := git.WorkItem{ID: strings.Repeat("d", 64), NewPath: "pkg/command.go", Path: "pkg/command.go",
		Side: git.Right, Hunk: 1, StartLine: 8, EndLine: 8}
	groups := []contextplan.Group{{ID: "source-group", Items: []git.WorkItem{item}}}
	selected, err := Select(t.Context(), BuiltIn(), groups)
	if err != nil || len(selected) != 2 {
		t.Fatalf("built-in selection: %v, %+v", err, selected)
	}
	bindings, err := Questions(t.Context(), selected)
	if err != nil {
		t.Fatal(err)
	}
	batch := contextplan.Batch{GroupIDs: []string{"source-group"},
		Request:        jev.Request{Model: "jev-1.13.0", State: "committed Go source", Questions: map[string]jev.Question{}},
		ItemByQuestion: map[string]string{}}
	for _, binding := range bindings {
		batch.Request.Questions[binding.QuestionID] = binding.Question
		batch.ItemByQuestion[binding.QuestionID] = binding.WorkItemID
	}
	safe := jev.Response{Model: "jev-1.13.0", Answers: map[string]jev.Answer{}}
	buggy := jev.Response{Model: "jev-1.13.0", Answers: map[string]jev.Answer{}}
	for _, selection := range selected {
		safe.Answers[selection.QuestionID] = jev.NoulAnswer{Probability: 0.1}
		probability := 0.1
		if selection.Rule.ID == "security.shell-injection" {
			probability = 0.96
		}
		buggy.Answers[selection.QuestionID] = jev.NoulAnswer{Probability: probability}
	}
	decisions, err := Decide(t.Context(), batch, selected, safe)
	if err != nil || len(decisions) != 0 {
		t.Fatalf("safe: %v, %+v", err, decisions)
	}
	decisions, err = Decide(t.Context(), batch, selected, buggy)
	if err != nil || len(decisions) != 1 || decisions[0].RuleID != "security.shell-injection" ||
		decisions[0].Severity != High || decisions[0].Path != item.Path || decisions[0].Value != 0.96 {
		t.Fatalf("buggy: %v, %+v", err, decisions)
	}
}

func TestUnsafeRuleFieldsAndLimits(t *testing.T) {
	for _, extra := range []string{"    provider_url: https://evil.invalid\n", "    token_env: SECRET\n", "    command: ./run.sh\n", "    template: '{{ .Exec }}'\n"} {
		pack, err := Load(strings.NewReader(minimalPack + extra))
		if !errors.Is(err, ErrInvalidPack) || len(pack.Rules()) != 0 {
			t.Fatalf("unsafe field %q: %v", extra, err)
		}
	}
	pack, err := Load(strings.NewReader(strings.Replace(minimalPack, "instructions: Is this a problem?",
		"instructions: "+strings.Repeat("x", maxInstructions+1), 1)))
	if !errors.Is(err, ErrInvalidRule) || len(pack.Rules()) != 0 {
		t.Fatalf("overlong: %v", err)
	}
}
