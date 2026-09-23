package rules

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sort"

	"github.com/diffpal/lintpal/internal/apps/lintpal/contextplan"
	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

var ErrInvalidDecision = errors.New("invalid rule decision")

// Decision carries fixed rule metadata, immutable Git coordinates, and one
// validated numeric signal. Value is a probability for Noul/Choice and a
// normalized score for Score; Kind names that distinction.
type Decision struct {
	RuleID     string
	WorkItemID string
	Path       string
	Side       git.Side
	StartLine  int
	EndLine    int
	Severity   Severity
	Title      string
	Message    string
	Kind       string
	Value      float64
	Confidence *float64
}

// Decide validates the complete provider response before emitting any decision.
// selections may include other batches; only exact question IDs in batch apply.
func Decide(ctx context.Context, batch contextplan.Batch, selections []Selection, response jev.Response) ([]Decision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := jev.ValidateResponse(batch.Request, response); err != nil {
		return nil, ErrInvalidDecision
	}
	if len(batch.Request.Questions) == 0 || len(batch.ItemByQuestion) != len(batch.Request.Questions) {
		return nil, ErrInvalidDecision
	}
	byID := make(map[string]Selection, len(selections))
	for _, selection := range selections {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if selection.QuestionID == "" {
			return nil, ErrInvalidDecision
		}
		if _, exists := byID[selection.QuestionID]; exists {
			return nil, ErrInvalidDecision
		}
		byID[selection.QuestionID] = selection
	}
	selected := make([]Selection, 0, len(batch.Request.Questions))
	for id, question := range batch.Request.Questions {
		selection, ok := byID[id]
		if !ok || !validSelectionItem(selection.Item) || batch.ItemByQuestion[id] != selection.Item.ID ||
			!containsGroup(batch.GroupIDs, selection.GroupID) || selection.QuestionID != selection.Item.ID+"/"+selection.Rule.ID ||
			math.IsNaN(selection.Rule.Threshold) || math.IsInf(selection.Rule.Threshold, 0) ||
			selection.Rule.Threshold < 0 || selection.Rule.Threshold > 1 {
			return nil, ErrInvalidDecision
		}
		bindings, err := Questions(ctx, []Selection{selection})
		if err != nil || len(bindings) != 1 || !reflect.DeepEqual(bindings[0].Question, question) {
			return nil, ErrInvalidDecision
		}
		selected = append(selected, selection)
	}
	sort.Slice(selected, func(i, j int) bool {
		if itemLess(selected[i].Item, selected[j].Item) {
			return true
		}
		if itemLess(selected[j].Item, selected[i].Item) {
			return false
		}
		return selected[i].Rule.ID < selected[j].Rule.ID
	})
	decisions := make([]Decision, 0, len(selected))
	for _, selection := range selected {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		answer := response.Answers[selection.QuestionID]
		value, kind, confidence, trigger := evaluate(selection.Rule, answer)
		if !trigger {
			continue
		}
		item, rule := selection.Item, selection.Rule
		decisions = append(decisions, Decision{RuleID: rule.ID, WorkItemID: item.ID,
			Path: item.Path, Side: item.Side, StartLine: item.StartLine, EndLine: item.EndLine,
			Severity: rule.Severity, Title: rule.Title, Message: rule.Message,
			Kind: kind, Value: value, Confidence: confidence})
	}
	return decisions, nil
}

func containsGroup(groups []string, groupID string) bool {
	for _, id := range groups {
		if id == groupID {
			return true
		}
	}
	return false
}

func evaluate(rule Rule, answer jev.Answer) (float64, string, *float64, bool) {
	switch rule.Type {
	case "noul":
		var a jev.NoulAnswer
		switch v := answer.(type) {
		case jev.NoulAnswer:
			a = v
		case *jev.NoulAnswer:
			a = *v
		}
		return a.Probability, "noul_probability", nil, a.Probability >= rule.Threshold
	case "choice":
		var a jev.ChoiceAnswer
		switch v := answer.(type) {
		case jev.ChoiceAnswer:
			a = v
		case *jev.ChoiceAnswer:
			a = *v
		}
		probability := a.Probabilities[a.Choice]
		for _, trigger := range rule.TriggerChoices {
			if a.Choice == trigger && probability >= rule.Threshold {
				confidence := a.Confidence
				return probability, "selected_probability", &confidence, true
			}
		}
		return probability, "selected_probability", nil, false
	case "score":
		var a jev.ScoreAnswer
		switch v := answer.(type) {
		case jev.ScoreAnswer:
			a = v
		case *jev.ScoreAnswer:
			a = *v
		}
		value := a.Score / float64(len(rule.ScoreCriteria)-1)
		confidence := a.Confidence
		return value, "normalized_score", &confidence, value >= rule.Threshold
	default:
		return 0, "", nil, false
	}
}
