package rules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/contextplan"
	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

const typedPack = `schema: lintpal.rules.v1
rules:
  - id: demo.noul
    type: noul
    instructions: Is this risky?
    threshold: 0.8
    severity: high
    title: Risk
    message: Risk found.
    paths: ['*.go']
    sides: [RIGHT]
  - id: demo.choice
    type: choice
    instructions: Which class?
    criteria: {safe: Safe, risky: Risky}
    trigger_choices: [risky]
    threshold: 0.8
    severity: medium
    title: Class
    message: Risky class.
    paths: ['src/*.go']
  - id: demo.score
    type: score
    instructions: How severe?
    criteria: [Low, High]
    threshold: 0.8
    severity: low
    title: Score
    message: High score.
`

func sampleGroups() []contextplan.Group {
	left := git.WorkItem{ID: strings.Repeat("a", 64), Path: "src/old.go", OldPath: "src/old.go", Side: git.Left, Hunk: 1, StartLine: 2, EndLine: 2}
	right := git.WorkItem{ID: strings.Repeat("b", 64), Path: "src/new.go", NewPath: "src/new.go", Side: git.Right, Hunk: 1, StartLine: 3, EndLine: 3}
	doc := git.WorkItem{ID: strings.Repeat("c", 64), Path: "doc.txt", NewPath: "doc.txt", Side: git.Right, Hunk: 1, StartLine: 1, EndLine: 1}
	return []contextplan.Group{{ID: "left", Items: []git.WorkItem{left}}, {ID: "right", Items: []git.WorkItem{right}}, {ID: "doc", Items: []git.WorkItem{doc}}}
}

func TestSelectAndQuestions(t *testing.T) {
	pack, err := Load(strings.NewReader(typedPack))
	if err != nil {
		t.Fatal(err)
	}
	groups := sampleGroups()
	selected, err := Select(t.Context(), pack, groups)
	if err != nil || len(selected) != 6 {
		t.Fatalf("selected: %v, %+v", err, selected)
	}
	for _, selection := range selected {
		if selection.Item.Side == git.Left && selection.Rule.ID == "demo.noul" {
			t.Fatal("noul matched LEFT")
		}
		if selection.Item.Path == "doc.txt" && selection.Rule.ID != "demo.score" {
			t.Fatal("path mismatch")
		}
	}
	bindings, err := Questions(t.Context(), selected)
	if err != nil || len(bindings) != 6 {
		t.Fatalf("bindings: %v, %+v", err, bindings)
	}
	seenTypes := map[string]bool{}
	for _, binding := range bindings {
		if len(binding.QuestionID) > 128 || !strings.HasPrefix(binding.QuestionID, binding.WorkItemID+"/") {
			t.Fatalf("bad ID: %s", binding.QuestionID)
		}
		switch binding.Question.(type) {
		case jev.NoulQuestion:
			seenTypes["noul"] = true
		case jev.ChoiceQuestion:
			seenTypes["choice"] = true
		case jev.ScoreQuestion:
			seenTypes["score"] = true
		default:
			t.Fatalf("bad type %T", binding.Question)
		}
	}
	if len(seenTypes) != 3 {
		t.Fatalf("types: %+v", seenTypes)
	}
	groups[0], groups[2] = groups[2], groups[0]
	again, err := Select(t.Context(), pack, groups)
	if err != nil || !reflect.DeepEqual(selected, again) {
		t.Fatalf("unstable selection: %v", err)
	}
	selected = append(selected, selected[0])
	if got, err := Questions(t.Context(), selected); err == nil || len(got) != 0 {
		t.Fatal("duplicate question accepted")
	}
}
