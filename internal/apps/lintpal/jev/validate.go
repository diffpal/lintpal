package jev

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
)

var ErrInvalidRequest = errors.New("invalid Jev request")
var ErrInvalidResponse = errors.New("invalid Jev response")

// ValidateRequest checks the native contract without exposing state or question text in errors.
func ValidateRequest(request Request) error {
	if strings.TrimSpace(request.Model) == "" || len(request.Questions) == 0 || request.State == nil {
		return ErrInvalidRequest
	}
	state, err := json.Marshal(request.State)
	if err != nil || len(state) == 0 || (state[0] != '"' && state[0] != '{' && state[0] != '[') {
		return ErrInvalidRequest
	}
	for id, question := range request.Questions {
		if strings.TrimSpace(id) == "" || len(id) > 128 || !validQuestion(question) {
			return ErrInvalidRequest
		}
	}
	return nil
}

func validQuestion(question Question) bool {
	switch q := question.(type) {
	case NoulQuestion:
		return strings.TrimSpace(q.Instructions) != ""
	case *NoulQuestion:
		return q != nil && validQuestion(*q)
	case ChoiceQuestion:
		if strings.TrimSpace(q.Instructions) == "" || len(q.Criteria) < 2 || len(q.Criteria) > 255 {
			return false
		}
		for option := range q.Criteria {
			if strings.TrimSpace(option) == "" {
				return false
			}
		}
		return true
	case *ChoiceQuestion:
		return q != nil && validQuestion(*q)
	case ScoreQuestion:
		if strings.TrimSpace(q.Instructions) == "" || len(q.Criteria) < 2 || len(q.Criteria) > 10 {
			return false
		}
		for _, level := range q.Criteria {
			if strings.TrimSpace(level) == "" {
				return false
			}
		}
		return true
	case *ScoreQuestion:
		return q != nil && validQuestion(*q)
	default:
		return false
	}
}

// ValidateResponse rejects partial or inconsistent decisions before policy uses them.
func ValidateResponse(request Request, response Response) error {
	if strings.TrimSpace(response.Model) == "" || len(response.Answers) != len(request.Questions) ||
		response.Usage.InputTokens < 0 || response.Usage.OutputTokens < 0 ||
		(response.Usage.CostUSD != nil && (!finite(*response.Usage.CostUSD) || *response.Usage.CostUSD < 0)) {
		return ErrInvalidResponse
	}
	for id, question := range request.Questions {
		answer, ok := response.Answers[id]
		if !ok || !validAnswer(question, answer) {
			return ErrInvalidResponse
		}
	}
	return nil
}

func validAnswer(question Question, answer Answer) bool {
	switch q := question.(type) {
	case *NoulQuestion:
		if q == nil {
			return false
		}
		return validAnswer(*q, answer)
	case NoulQuestion:
		a, ok := answer.(NoulAnswer)
		if !ok {
			if p, pointerOK := answer.(*NoulAnswer); pointerOK && p != nil {
				a = *p
			} else {
				return false
			}
		}
		return unit(a.Probability)
	case *ChoiceQuestion:
		if q == nil {
			return false
		}
		return validAnswer(*q, answer)
	case ChoiceQuestion:
		a, ok := answer.(ChoiceAnswer)
		if !ok {
			if p, pointerOK := answer.(*ChoiceAnswer); pointerOK && p != nil {
				a = *p
			} else {
				return false
			}
		}
		if _, ok := q.Criteria[a.Choice]; !ok || !unit(a.Confidence) || !distribution(a.Probabilities, q.Criteria) {
			return false
		}
		for _, probability := range a.Probabilities {
			if probability > a.Probabilities[a.Choice]+1e-6 {
				return false
			}
		}
		return true
	case *ScoreQuestion:
		if q == nil {
			return false
		}
		return validAnswer(*q, answer)
	case ScoreQuestion:
		a, ok := answer.(ScoreAnswer)
		if !ok {
			if p, pointerOK := answer.(*ScoreAnswer); pointerOK && p != nil {
				a = *p
			} else {
				return false
			}
		}
		if !finite(a.Score) || a.Score < 0 || a.Score > float64(len(q.Criteria)-1) || !unit(a.Confidence) ||
			len(a.Legend) != len(q.Criteria) || len(a.Probabilities) != len(q.Criteria) {
			return false
		}
		sum := 0.0
		weighted := 0.0
		for index, level := range q.Criteria {
			key := strconv.Itoa(index)
			probability, ok := a.Probabilities[key]
			if !ok || a.Legend[key] != level || !unit(probability) {
				return false
			}
			sum += probability
			weighted += float64(index) * probability
		}
		return math.Abs(sum-1) <= 1e-6 && math.Abs(a.Score-weighted) <= 0.02
	default:
		return false
	}
}

func distribution(values map[string]float64, criteria map[string]string) bool {
	if len(values) != len(criteria) {
		return false
	}
	sum := 0.0
	for key := range criteria {
		value, ok := values[key]
		if !ok || !unit(value) {
			return false
		}
		sum += value
	}
	return math.Abs(sum-1) <= 1e-6
}

func unit(value float64) bool   { return finite(value) && value >= 0 && value <= 1 }
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
