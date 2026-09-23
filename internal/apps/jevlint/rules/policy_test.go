package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/diffpal/jevlint/internal/apps/jevlint/contextplan"
	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
)

func policyFixture(t *testing.T) (contextplan.Batch, []Selection) {
	t.Helper()
	pack, err := Load(strings.NewReader(typedPack))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := Select(t.Context(), pack, sampleGroups())
	if err != nil {
		t.Fatal(err)
	}
	var right []Selection
	for _, selection := range selected {
		if selection.GroupID == "right" {
			right = append(right, selection)
		}
	}
	bindings, err := Questions(t.Context(), right)
	if err != nil {
		t.Fatal(err)
	}
	batch := contextplan.Batch{GroupIDs: []string{"right"}, Request: jev.Request{
		Model: "jev-1.13.0", State: "source", Questions: make(map[string]jev.Question)},
		ItemByQuestion: make(map[string]string)}
	for _, binding := range bindings {
		batch.Request.Questions[binding.QuestionID] = binding.Question
		batch.ItemByQuestion[binding.QuestionID] = binding.WorkItemID
	}
	return batch, right
}

func responseAt(selections []Selection, value float64) jev.Response {
	answers := make(map[string]jev.Answer)
	for _, selection := range selections {
		switch selection.Rule.Type {
		case "noul":
			answers[selection.QuestionID] = jev.NoulAnswer{Probability: value}
		case "choice":
			answers[selection.QuestionID] = jev.ChoiceAnswer{Choice: "risky",
				Probabilities: map[string]float64{"safe": 1 - value, "risky": value}, Confidence: value}
		case "score":
			answers[selection.QuestionID] = jev.ScoreAnswer{Score: value,
				Legend:        map[string]string{"0": "Low", "1": "High"},
				Probabilities: map[string]float64{"0": 1 - value, "1": value}, Confidence: value}
		}
	}
	return jev.Response{Model: "jev-1.13.0", Answers: answers}
}

func TestDecisionThresholdsAndFixedMetadata(t *testing.T) {
	batch, selected := policyFixture(t)
	for _, tc := range []struct {
		value float64
		want  int
	}{{0.79, 0}, {0.8, 3}, {0.81, 3}} {
		decisions, err := Decide(t.Context(), batch, selected, responseAt(selected, tc.value))
		if err != nil || len(decisions) != tc.want {
			t.Fatalf("value %.2f: %v, %+v", tc.value, err, decisions)
		}
		for _, decision := range decisions {
			if decision.Path != "src/new.go" || decision.StartLine != 3 || decision.EndLine != 3 ||
				decision.Title == "" || decision.Message == "" || decision.Kind == "" || decision.Value != tc.value {
				t.Fatalf("bad decision: %+v", decision)
			}
		}
	}
	response := responseAt(selected, 0.8)
	for _, selection := range selected {
		if selection.Rule.Type == "choice" {
			response.Answers[selection.QuestionID] = jev.ChoiceAnswer{Choice: "safe",
				Probabilities: map[string]float64{"safe": 0.8, "risky": 0.2}, Confidence: 0.8}
		}
	}
	decisions, err := Decide(t.Context(), batch, selected, response)
	if err != nil || len(decisions) != 2 {
		t.Fatalf("nontrigger choice: %v, %+v", err, decisions)
	}
}

func TestDecisionRejectsPartialOrMismatchedResponse(t *testing.T) {
	batch, selected := policyFixture(t)
	response := responseAt(selected, 0.8)
	delete(response.Answers, selected[0].QuestionID)
	got, err := Decide(t.Context(), batch, selected, response)
	if !errors.Is(err, ErrInvalidDecision) || len(got) != 0 {
		t.Fatalf("missing: %v", err)
	}
	response = responseAt(selected, 0.8)
	response.Answers["extra"] = jev.NoulAnswer{Probability: 0.9}
	got, err = Decide(t.Context(), batch, selected, response)
	if !errors.Is(err, ErrInvalidDecision) || len(got) != 0 {
		t.Fatalf("extra: %v", err)
	}
	response = responseAt(selected, 0.8)
	batch.ItemByQuestion[selected[0].QuestionID] = strings.Repeat("f", 64)
	got, err = Decide(t.Context(), batch, selected, response)
	if !errors.Is(err, ErrInvalidDecision) || len(got) != 0 {
		t.Fatalf("mismatch: %v", err)
	}
}
