// Package jev defines the typed decision boundary used by lintpal.
package jev

import "context"

// Provider evaluates typed questions against one state.
type Provider interface {
	Evaluate(context.Context, Request) (Response, error)
}

// Request is independent of the provider's wire format.
type Request struct {
	Model     string
	State     any
	Questions map[string]Question
}

// Question is one of the supported Jev decision primitives.
type Question interface {
	isQuestion()
}

type NoulQuestion struct {
	Instructions string
	Criteria     *NoulCriteria
}

func (NoulQuestion) isQuestion() {}

// NoulCriteria optionally describes the two ends of a yes/no answer.
type NoulCriteria struct {
	True  string
	False string
}

type ChoiceQuestion struct {
	Instructions string
	Criteria     map[string]string
}

func (ChoiceQuestion) isQuestion() {}

type ScoreQuestion struct {
	Instructions string
	Criteria     []string
}

func (ScoreQuestion) isQuestion() {}

// Response carries decisions and optional usage metadata.
type Response struct {
	Model   string
	Answers map[string]Answer
	Usage   Usage
}

type Answer interface {
	isAnswer()
}

type NoulAnswer struct {
	Probability float64
}

func (NoulAnswer) isAnswer() {}

type ChoiceAnswer struct {
	Choice        string
	Probabilities map[string]float64
	Confidence    float64
}

func (ChoiceAnswer) isAnswer() {}

type ScoreAnswer struct {
	Score         float64
	Legend        map[string]string
	Probabilities map[string]float64
	Confidence    float64
}

func (ScoreAnswer) isAnswer() {}

type Usage struct {
	InputTokens  int
	OutputTokens int
	CostUSD      *float64
}
