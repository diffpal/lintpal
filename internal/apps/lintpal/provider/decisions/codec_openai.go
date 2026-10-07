package decisions

import (
	"encoding/json"
	"sort"
	"strconv"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

// GPT-6 Luna Standard short-context rates published by OpenAI on 2026-10-06.
const (
	openAILunaInputUSDPerMillion      = 0.10
	openAILunaCachedUSDPerMillion     = 0.01
	openAILunaCacheWriteUSDPerMillion = 0.125
	openAILunaOutputUSDPerMillion     = 0.50
)

type openAICodec struct{}

type openAIRequestWire struct {
	Input     string               `json:"input"`
	Model     string               `json:"model"`
	Questions []openAIQuestionWire `json:"questions"`
}

type openAIQuestionWire struct {
	Type         string             `json:"type"`
	Name         string             `json:"name"`
	Instructions string             `json:"instructions"`
	Choices      []openAIChoiceWire `json:"choices,omitempty"`
	Levels       []openAILevelWire  `json:"levels,omitempty"`
}

type openAIChoiceWire struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type openAILevelWire struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

func (openAICodec) encode(request jev.Request) ([]byte, error) {
	input, ok := request.State.(string)
	if !ok {
		return nil, jev.ErrInvalidRequest
	}
	ids := make([]string, 0, len(request.Questions))
	for id := range request.Questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	questions := make([]openAIQuestionWire, 0, len(ids))
	for _, id := range ids {
		question, ok := openAIQuestion(id, request.Questions[id])
		if !ok {
			return nil, jev.ErrInvalidRequest
		}
		questions = append(questions, question)
	}
	body, err := json.Marshal(openAIRequestWire{Input: input, Model: request.Model, Questions: questions})
	if err != nil {
		return nil, jev.ErrInvalidRequest
	}
	return body, nil
}

func openAIQuestion(id string, question jev.Question) (openAIQuestionWire, bool) {
	switch q := question.(type) {
	case jev.NoulQuestion:
		return openAINoulQuestion(id, q), true
	case *jev.NoulQuestion:
		if q == nil {
			return openAIQuestionWire{}, false
		}
		return openAINoulQuestion(id, *q), true
	case jev.ChoiceQuestion:
		return openAIChoiceQuestion(id, q), true
	case *jev.ChoiceQuestion:
		if q == nil {
			return openAIQuestionWire{}, false
		}
		return openAIChoiceQuestion(id, *q), true
	case jev.ScoreQuestion:
		return openAIScoreQuestion(id, q), true
	case *jev.ScoreQuestion:
		if q == nil {
			return openAIQuestionWire{}, false
		}
		return openAIScoreQuestion(id, *q), true
	default:
		return openAIQuestionWire{}, false
	}
}

func openAINoulQuestion(id string, q jev.NoulQuestion) openAIQuestionWire {
	instructions := q.Instructions
	if q.Criteria != nil {
		instructions += "\n\nAnswer criteria:\n- true: " + q.Criteria.True + "\n- false: " + q.Criteria.False
	}
	return openAIQuestionWire{Type: "predicate", Name: id, Instructions: instructions}
}

func openAIChoiceQuestion(id string, q jev.ChoiceQuestion) openAIQuestionWire {
	values := make([]string, 0, len(q.Criteria))
	for value := range q.Criteria {
		values = append(values, value)
	}
	sort.Strings(values)
	choices := make([]openAIChoiceWire, 0, len(values))
	for _, value := range values {
		choices = append(choices, openAIChoiceWire{Value: value, Description: q.Criteria[value]})
	}
	return openAIQuestionWire{Type: "choice", Name: id, Instructions: q.Instructions, Choices: choices}
}

func openAIScoreQuestion(id string, q jev.ScoreQuestion) openAIQuestionWire {
	levels := make([]openAILevelWire, 0, len(q.Criteria))
	for index, description := range q.Criteria {
		levels = append(levels, openAILevelWire{Label: strconv.Itoa(index), Description: description})
	}
	return openAIQuestionWire{Type: "score", Name: id, Instructions: q.Instructions, Levels: levels}
}

type openAIResponseWire struct {
	Model   string            `json:"model"`
	Answers []json.RawMessage `json:"answers"`
	Usage   *openAIUsageWire  `json:"usage"`
}

type openAIUsageWire struct {
	InputTokens       *int                    `json:"input_tokens"`
	InputTokenDetails *openAIInputDetailsWire `json:"input_tokens_details"`
	OutputTokens      *int                    `json:"output_tokens"`
	Cost              *float64                `json:"cost"`
}

type openAIInputDetailsWire struct {
	CachedTokens     *int `json:"cached_tokens"`
	CacheWriteTokens *int `json:"cache_write_tokens"`
}

type openAIAnswerHeader struct {
	Type string  `json:"type"`
	Name *string `json:"name"`
}

type openAIPredicateAnswerWire struct {
	Name        *string  `json:"name"`
	Probability *float64 `json:"probability"`
}

type openAIChoiceAnswerWire struct {
	Name          *string                       `json:"name"`
	Choice        *string                       `json:"choice"`
	Confidence    *float64                      `json:"confidence"`
	Probabilities []openAIChoiceProbabilityWire `json:"probabilities"`
}

type openAIChoiceProbabilityWire struct {
	Value       string   `json:"value"`
	Probability *float64 `json:"probability"`
}

type openAIScoreAnswerWire struct {
	Name          *string                      `json:"name"`
	Score         *float64                     `json:"score"`
	Confidence    *float64                     `json:"confidence"`
	Probabilities []openAIScoreProbabilityWire `json:"probabilities"`
}

type openAIScoreProbabilityWire struct {
	Label       string   `json:"label"`
	Value       *int     `json:"value"`
	Probability *float64 `json:"probability"`
}

func (openAICodec) decode(data []byte, request jev.Request) (jev.Response, error) {
	var wire openAIResponseWire
	if err := json.Unmarshal(data, &wire); err != nil || wire.Usage == nil || wire.Usage.InputTokens == nil ||
		wire.Usage.OutputTokens == nil || wire.Usage.InputTokenDetails == nil ||
		wire.Usage.InputTokenDetails.CachedTokens == nil || wire.Usage.InputTokenDetails.CacheWriteTokens == nil {
		return jev.Response{}, ErrProtocol
	}
	inputTokens := *wire.Usage.InputTokens
	outputTokens := *wire.Usage.OutputTokens
	cachedTokens := *wire.Usage.InputTokenDetails.CachedTokens
	cacheWriteTokens := *wire.Usage.InputTokenDetails.CacheWriteTokens
	if inputTokens < 0 || outputTokens < 0 || cachedTokens < 0 || cacheWriteTokens < 0 ||
		cachedTokens > inputTokens || cacheWriteTokens > inputTokens-cachedTokens {
		return jev.Response{}, ErrProtocol
	}
	response := jev.Response{
		Model:   wire.Model,
		Answers: make(map[string]jev.Answer, len(wire.Answers)),
		Usage: jev.Usage{InputTokens: inputTokens, OutputTokens: outputTokens,
			CostUSD: openAICost(request.Model, inputTokens, outputTokens, cachedTokens, cacheWriteTokens, wire.Usage.Cost), RequestCount: 1},
	}
	for _, raw := range wire.Answers {
		var header openAIAnswerHeader
		if err := json.Unmarshal(raw, &header); err != nil || header.Name == nil || *header.Name == "" {
			return jev.Response{}, ErrProtocol
		}
		id := *header.Name
		question, ok := request.Questions[id]
		if !ok {
			return jev.Response{}, ErrProtocol
		}
		if _, duplicate := response.Answers[id]; duplicate {
			return jev.Response{}, ErrProtocol
		}
		answer, ok := decodeOpenAIAnswer(raw, header.Type, question)
		if !ok {
			return jev.Response{}, ErrProtocol
		}
		response.Answers[id] = answer
	}
	if err := jev.ValidateResponse(request, response); err != nil {
		return jev.Response{}, ErrProtocol
	}
	return response, nil
}

func decodeOpenAIAnswer(raw []byte, answerType string, question jev.Question) (jev.Answer, bool) {
	switch answerType {
	case "predicate":
		if !isNoulQuestion(question) {
			return nil, false
		}
		var wire openAIPredicateAnswerWire
		if err := json.Unmarshal(raw, &wire); err != nil || wire.Probability == nil {
			return nil, false
		}
		return jev.NoulAnswer{Probability: *wire.Probability}, true
	case "choice":
		criteria, ok := choiceCriteria(question)
		if !ok {
			return nil, false
		}
		var wire openAIChoiceAnswerWire
		if err := json.Unmarshal(raw, &wire); err != nil || wire.Choice == nil || wire.Confidence == nil || wire.Probabilities == nil {
			return nil, false
		}
		probabilities := make(map[string]float64, len(wire.Probabilities))
		for _, item := range wire.Probabilities {
			if item.Probability == nil {
				return nil, false
			}
			if _, exists := probabilities[item.Value]; exists {
				return nil, false
			}
			if _, exists := criteria[item.Value]; !exists {
				return nil, false
			}
			probabilities[item.Value] = *item.Probability
		}
		return jev.ChoiceAnswer{Choice: *wire.Choice, Confidence: *wire.Confidence, Probabilities: probabilities}, true
	case "score":
		criteria, ok := scoreCriteria(question)
		if !ok {
			return nil, false
		}
		var wire openAIScoreAnswerWire
		if err := json.Unmarshal(raw, &wire); err != nil || wire.Score == nil || wire.Confidence == nil || wire.Probabilities == nil {
			return nil, false
		}
		legend := make(map[string]string, len(criteria))
		probabilities := make(map[string]float64, len(wire.Probabilities))
		for index, description := range criteria {
			legend[strconv.Itoa(index)] = description
		}
		for _, item := range wire.Probabilities {
			if item.Value == nil || item.Probability == nil || *item.Value < 0 || *item.Value >= len(criteria) {
				return nil, false
			}
			key := strconv.Itoa(*item.Value)
			if item.Label != key {
				return nil, false
			}
			if _, exists := probabilities[key]; exists {
				return nil, false
			}
			probabilities[key] = *item.Probability
		}
		return jev.ScoreAnswer{Score: *wire.Score, Confidence: *wire.Confidence, Legend: legend, Probabilities: probabilities}, true
	default:
		return nil, false
	}
}

func isNoulQuestion(question jev.Question) bool {
	switch q := question.(type) {
	case jev.NoulQuestion:
		return true
	case *jev.NoulQuestion:
		return q != nil
	default:
		return false
	}
}

func choiceCriteria(question jev.Question) (map[string]string, bool) {
	switch q := question.(type) {
	case jev.ChoiceQuestion:
		return q.Criteria, true
	case *jev.ChoiceQuestion:
		if q != nil {
			return q.Criteria, true
		}
	}
	return nil, false
}

func scoreCriteria(question jev.Question) ([]string, bool) {
	switch q := question.(type) {
	case jev.ScoreQuestion:
		return q.Criteria, true
	case *jev.ScoreQuestion:
		if q != nil {
			return q.Criteria, true
		}
	}
	return nil, false
}

func openAICost(model string, inputTokens, outputTokens, cachedTokens, cacheWriteTokens int, reported *float64) *float64 {
	if reported != nil || model != "gpt-6-luna" {
		return reported
	}
	ordinaryTokens := inputTokens - cachedTokens - cacheWriteTokens
	cost := (float64(ordinaryTokens)*openAILunaInputUSDPerMillion +
		float64(cachedTokens)*openAILunaCachedUSDPerMillion +
		float64(cacheWriteTokens)*openAILunaCacheWriteUSDPerMillion +
		float64(outputTokens)*openAILunaOutputUSDPerMillion) / 1_000_000
	return &cost
}
