package jev

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func sampleRequest() Request {
	return Request{
		Model: "jev-1.13",
		State: map[string]any{"change": "bounded source"},
		Questions: map[string]Question{
			"n": NoulQuestion{Instructions: "Is there a problem?"},
			"c": ChoiceQuestion{Instructions: "Which kind?", Criteria: map[string]string{"safe": "safe", "risk": "risky"}},
			"s": ScoreQuestion{Instructions: "How severe?", Criteria: []string{"low", "medium", "high"}},
		},
	}
}

func sampleResponse() Response {
	return Response{
		Model: "jev-1.13.0",
		Answers: map[string]Answer{
			"n": NoulAnswer{Probability: 0.7},
			"c": ChoiceAnswer{Choice: "risk", Probabilities: map[string]float64{"safe": 0.2, "risk": 0.8}, Confidence: 0.6},
			"s": ScoreAnswer{Score: 1.7, Legend: map[string]string{"0": "low", "1": "medium", "2": "high"}, Probabilities: map[string]float64{"0": 0, "1": 0.3, "2": 0.7}, Confidence: 0.7},
		},
		Usage: Usage{InputTokens: 10, OutputTokens: 3},
	}
}

func TestValidateTypedContract(t *testing.T) {
	request := sampleRequest()
	if err := ValidateRequest(request); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResponse(request, sampleResponse()); err != nil {
		t.Fatal(err)
	}
}

func TestRejectInvalidRequestWithoutEcho(t *testing.T) {
	for _, mutate := range []func(*Request){
		func(r *Request) { r.Model = "" },
		func(r *Request) { r.State = 42 },
		func(r *Request) { r.Questions = nil },
		func(r *Request) {
			r.Questions["c"] = ChoiceQuestion{Instructions: "secret question", Criteria: map[string]string{"only": "one"}}
		},
		func(r *Request) {
			r.Questions["s"] = ScoreQuestion{Instructions: "secret question", Criteria: []string{"one"}}
		},
		func(r *Request) { r.Questions["n"] = (*NoulQuestion)(nil) },
	} {
		request := sampleRequest()
		mutate(&request)
		err := ValidateRequest(request)
		if !errors.Is(err, ErrInvalidRequest) || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "bounded source") {
			t.Fatalf("invalid request error = %v", err)
		}
	}
}

func TestRejectInvalidAnswers(t *testing.T) {
	request := sampleRequest()
	for _, mutate := range []func(*Response){
		func(r *Response) { delete(r.Answers, "n") },
		func(r *Response) { r.Answers["other"] = NoulAnswer{Probability: 0.5} },
		func(r *Response) { r.Answers["n"] = NoulAnswer{Probability: math.NaN()} },
		func(r *Response) { r.Answers["c"] = NoulAnswer{Probability: 0.4} },
		func(r *Response) {
			r.Answers["c"] = ChoiceAnswer{Choice: "risk", Probabilities: map[string]float64{"risk": 0.8}, Confidence: 0.8}
		},
		func(r *Response) { r.Answers["s"] = ScoreAnswer{Score: 9, Confidence: 0.7} },
		func(r *Response) {
			r.Answers["s"] = ScoreAnswer{Score: 0.1, Legend: map[string]string{"0": "low", "1": "medium", "2": "high"}, Probabilities: map[string]float64{"0": 0, "1": 0.3, "2": 0.7}, Confidence: 0.7}
		},
		func(r *Response) { r.Usage.InputTokens = -1 },
	} {
		response := sampleResponse()
		mutate(&response)
		if err := ValidateResponse(request, response); !errors.Is(err, ErrInvalidResponse) {
			t.Fatalf("invalid response accepted: %+v, error = %v", response, err)
		}
	}
}
