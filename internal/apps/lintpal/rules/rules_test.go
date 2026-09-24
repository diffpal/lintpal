package rules

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadReturnsIndependentRules(t *testing.T) {
	pack, err := Load(strings.NewReader(minimalPack))
	if err != nil || len(pack.Rules()) != 1 {
		t.Fatalf("load: %v, %+v", err, pack)
	}
	copyRules := pack.Rules()
	copyRules[0].Title = "tampered"
	if pack.Rules()[0].Title == "tampered" {
		t.Fatal("pack leaked mutable rule")
	}
}

func TestRuleValidationMatrix(t *testing.T) {
	for name, change := range map[string]func(string) string{
		"duplicate ID": func(s string) string {
			return s + "  - id: demo.test\n    type: noul\n    instructions: Other?\n    threshold: 0.9\n    severity: high\n    title: Other\n    message: Other.\n"
		},
		"bad severity":      func(s string) string { return strings.Replace(s, "severity: high", "severity: severe", 1) },
		"missing threshold": func(s string) string { return strings.Replace(s, "threshold: 0.9", "threshold:", 1) },
		"bad threshold":     func(s string) string { return strings.Replace(s, "threshold: 0.9", "threshold: 1.1", 1) },
		"invalid path":      func(s string) string { return s + "    paths: ['[bad']\n" },
		"command":           func(s string) string { return s + "    command: ./bad.sh\n" },
		"noul trigger":      func(s string) string { return s + "    trigger_choices: [x]\n" },
	} {
		body := change(minimalPack)
		pack, err := Load(strings.NewReader(body))
		if err == nil || len(pack.Rules()) != 0 {
			t.Fatalf("%s: accepted invalid pack", name)
		}
	}
	choice := `schema: lintpal.rules.v1
rules:
  - id: demo.choice
    type: choice
    instructions: Which?
    criteria: {safe: Safe, risky: Risky}
    trigger_choices: [risky]
    threshold: 0.8
    severity: high
    title: Risk
    message: Risky choice.
`
	pack, err := Load(strings.NewReader(choice))
	if err != nil || len(pack.Rules()) != 1 || len(pack.Rules()[0].ChoiceCriteria) != 2 {
		t.Fatalf("choice: %v", err)
	}
	pack, err = Load(strings.NewReader(strings.Replace(choice, "trigger_choices: [risky]", "trigger_choices: [missing]", 1)))
	if !errors.Is(err, ErrInvalidRule) || len(pack.Rules()) != 0 {
		t.Fatalf("bad choice: %v", err)
	}
	score := strings.Replace(choice, "type: choice", "type: score", 1)
	score = strings.Replace(score, "criteria: {safe: Safe, risky: Risky}", "criteria: [Low, High]", 1)
	score = strings.Replace(score, "    trigger_choices: [risky]\n", "", 1)
	pack, err = Load(strings.NewReader(score))
	if err != nil || len(pack.Rules()[0].ScoreCriteria) != 2 {
		t.Fatalf("score: %v", err)
	}
}
