package decisions

import (
	"encoding/json"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

type sharedCodec struct{}

func (sharedCodec) encode(request jev.Request) ([]byte, error) {
	questions := make(map[string]any, len(request.Questions))
	for id, question := range request.Questions {
		questions[id] = encodeQuestion(question)
	}
	body, err := json.Marshal(struct {
		State     any            `json:"state"`
		Model     string         `json:"model"`
		Questions map[string]any `json:"questions"`
	}{State: request.State, Model: request.Model, Questions: questions})
	if err != nil {
		return nil, jev.ErrInvalidRequest
	}
	return body, nil
}

func (sharedCodec) decode(data []byte, request jev.Request) (jev.Response, error) {
	return decodeResponse(data, request)
}

func encodeQuestion(question jev.Question) any {
	switch q := question.(type) {
	case jev.NoulQuestion:
		return noulWire(q)
	case *jev.NoulQuestion:
		return noulWire(*q)
	case jev.ChoiceQuestion:
		return map[string]any{"type": "choice", "instructions": q.Instructions, "criteria": q.Criteria}
	case *jev.ChoiceQuestion:
		return map[string]any{"type": "choice", "instructions": q.Instructions, "criteria": q.Criteria}
	case jev.ScoreQuestion:
		return map[string]any{"type": "score", "instructions": q.Instructions, "criteria": q.Criteria}
	case *jev.ScoreQuestion:
		return map[string]any{"type": "score", "instructions": q.Instructions, "criteria": q.Criteria}
	default:
		return nil
	}
}

func noulWire(q jev.NoulQuestion) map[string]any {
	wire := map[string]any{"type": "noul", "instructions": q.Instructions}
	if q.Criteria != nil {
		wire["criteria"] = map[string]string{"true": q.Criteria.True, "false": q.Criteria.False}
	}
	return wire
}

type responseWire struct {
	Model   string                     `json:"model"`
	Answers map[string]json.RawMessage `json:"answers"`
	Usage   *usageWire                 `json:"usage"`
}

type usageWire struct {
	InputTokens  *int     `json:"input_tokens"`
	OutputTokens *int     `json:"output_tokens"`
	Cost         *float64 `json:"cost"`
}

type answerWire struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul"`
	Choice        *string            `json:"choice"`
	Score         *float64           `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
	Legend        map[string]string  `json:"legend"`
	Confidence    *float64           `json:"confidence"`
}

func decodeResponse(data []byte, request jev.Request) (jev.Response, error) {
	var wire responseWire
	if err := json.Unmarshal(data, &wire); err != nil || wire.Usage == nil || wire.Usage.InputTokens == nil || wire.Usage.OutputTokens == nil {
		return jev.Response{}, ErrProtocol
	}
	response := jev.Response{
		Model:   wire.Model,
		Answers: make(map[string]jev.Answer, len(wire.Answers)),
		Usage:   jev.Usage{InputTokens: *wire.Usage.InputTokens, OutputTokens: *wire.Usage.OutputTokens, CostUSD: wire.Usage.Cost, RequestCount: 1},
	}
	for id, raw := range wire.Answers {
		var answer answerWire
		if err := json.Unmarshal(raw, &answer); err != nil {
			return jev.Response{}, ErrProtocol
		}
		switch answer.Type {
		case "noul":
			if answer.Noul == nil {
				return jev.Response{}, ErrProtocol
			}
			response.Answers[id] = jev.NoulAnswer{Probability: *answer.Noul}
		case "choice":
			if answer.Choice == nil || answer.Confidence == nil || answer.Probabilities == nil {
				return jev.Response{}, ErrProtocol
			}
			response.Answers[id] = jev.ChoiceAnswer{Choice: *answer.Choice, Probabilities: answer.Probabilities, Confidence: *answer.Confidence}
		case "score":
			if answer.Score == nil || answer.Confidence == nil || answer.Probabilities == nil || answer.Legend == nil {
				return jev.Response{}, ErrProtocol
			}
			response.Answers[id] = jev.ScoreAnswer{Score: *answer.Score, Legend: answer.Legend, Probabilities: answer.Probabilities, Confidence: *answer.Confidence}
		default:
			return jev.Response{}, ErrProtocol
		}
	}
	if err := jev.ValidateResponse(request, response); err != nil {
		return jev.Response{}, ErrProtocol
	}
	return response, nil
}
