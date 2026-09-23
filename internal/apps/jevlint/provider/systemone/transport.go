package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
)

var ErrMissingCredential = errors.New("System One credential is required")
var ErrTransport = errors.New("System One transport failed")
var ErrProtocol = errors.New("invalid System One response")
var ErrBodyLimit = errors.New("System One body limit exceeded")

const maxRequestBytes = 1 << 20
const maxResponseBytes = 2 << 20
const maxAttempts = 3
const attemptTimeout = 15 * time.Second
const maxRetryDelay = 2 * time.Second

// HTTPError exposes only a status code, never a provider response body.
type HTTPError struct{ Status int }

func (e HTTPError) Error() string { return fmt.Sprintf("System One HTTP status %d", e.Status) }

// Provider implements the native Jev decision port over one System One ABI.
type Provider struct {
	endpoint Endpoint
	target   *url.URL
	client   *http.Client
	timeout  time.Duration
}

var _ jev.Provider = (*Provider)(nil)

func New(endpoint Endpoint, client *http.Client) (*Provider, error) {
	target, err := endpoint.target()
	if err != nil {
		return nil, err
	}
	return &Provider{endpoint: endpoint, target: target, client: secureClient(client), timeout: attemptTimeout}, nil
}

func (p *Provider) Evaluate(ctx context.Context, request jev.Request) (jev.Response, error) {
	if err := jev.ValidateRequest(request); err != nil {
		return jev.Response{}, err
	}
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
		return jev.Response{}, jev.ErrInvalidRequest
	}
	if len(body) > maxRequestBytes {
		return jev.Response{}, ErrBodyLimit
	}
	token := p.endpoint.token()
	if p.endpoint.tokenEnv != "" && token == "" {
		return jev.Response{}, ErrMissingCredential
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return jev.Response{}, err
		}
		response, retry, delay, err := p.doOnce(ctx, body, token, request)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if !retry || attempt == maxAttempts-1 {
			return jev.Response{}, err
		}
		if delay < 0 {
			delay = retryBackoff(attempt)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return jev.Response{}, ctx.Err()
		case <-timer.C:
		}
	}
	return jev.Response{}, lastErr
}

func (p *Provider) doOnce(ctx context.Context, body []byte, token string, request jev.Request) (jev.Response, bool, time.Duration, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, p.target.String(), bytes.NewReader(body))
	if err != nil {
		return jev.Response{}, false, 0, ErrTransport
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+token)
	}
	httpResponse, err := p.client.Do(httpRequest)
	if httpResponse != nil {
		defer httpResponse.Body.Close()
	}
	if ctx.Err() != nil {
		return jev.Response{}, false, 0, ctx.Err()
	}
	if err != nil {
		return jev.Response{}, true, -1, ErrTransport
	}
	if httpResponse.StatusCode != http.StatusOK {
		status := httpResponse.StatusCode
		retry := status == http.StatusTooManyRequests || status == 529 || status >= 500 && status <= 599
		return jev.Response{}, retry, retryAfter(httpResponse.Header.Get("Retry-After"), time.Now()), HTTPError{Status: status}
	}
	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxResponseBytes+1))
	if ctx.Err() != nil {
		return jev.Response{}, false, 0, ctx.Err()
	}
	if err != nil {
		return jev.Response{}, true, -1, ErrTransport
	}
	if len(responseBody) > maxResponseBytes {
		return jev.Response{}, false, 0, ErrBodyLimit
	}
	response, err := decodeResponse(responseBody, request)
	if err != nil {
		return jev.Response{}, false, 0, err
	}
	return response, false, 0, nil
}

func retryBackoff(attempt int) time.Duration {
	delay := time.Duration(100*(1<<attempt)) * time.Millisecond
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return -1
		}
		if seconds > int(maxRetryDelay/time.Second) {
			return maxRetryDelay
		}
		return capRetryDelay(time.Duration(seconds) * time.Second)
	}
	if date, err := http.ParseTime(value); err == nil {
		return capRetryDelay(date.Sub(now))
	}
	return -1
}

func capRetryDelay(delay time.Duration) time.Duration {
	if delay < 0 {
		return 0
	}
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
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
		Usage:   jev.Usage{InputTokens: *wire.Usage.InputTokens, OutputTokens: *wire.Usage.OutputTokens, CostUSD: wire.Usage.Cost},
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
