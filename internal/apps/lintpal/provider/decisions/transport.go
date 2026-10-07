package decisions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

var ErrMissingCredential = errors.New("decisions credential is required")
var ErrTransport = errors.New("decisions transport failed")
var ErrProtocol = errors.New("invalid Decisions response")
var ErrBodyLimit = errors.New("decisions body limit exceeded")

const maxRequestBytes = 1 << 20
const maxResponseBytes = 2 << 20
const maxAttempts = 3
const attemptTimeout = 15 * time.Second
const maxRetryDelay = 2 * time.Second

// HTTPError exposes only a status code, never a provider response body.
type HTTPError struct{ Status int }

func (e HTTPError) Error() string { return fmt.Sprintf("Decisions HTTP status %d", e.Status) }

// Provider implements the typed decision port over the shared Decisions ABI.
type Provider struct {
	endpoint Endpoint
	target   *url.URL
	client   *http.Client
	codec    codec
	timeout  time.Duration
	token    *string
}

var _ jev.Provider = (*Provider)(nil)
var _ jev.RequestSizer = (*Provider)(nil)

func New(endpoint Endpoint, client *http.Client) (*Provider, error) {
	target, err := endpoint.target()
	if err != nil {
		return nil, err
	}
	selectedCodec, err := endpoint.codec()
	if err != nil {
		return nil, err
	}
	return &Provider{endpoint: endpoint, target: target, client: secureClient(client), codec: selectedCodec, timeout: attemptTimeout}, nil
}

// NewWithToken binds one trusted credential for this run without changing the
// process environment. An empty token still fails for credentialed endpoints.
func NewWithToken(endpoint Endpoint, client *http.Client, token string) (*Provider, error) {
	provider, err := New(endpoint, client)
	if err != nil {
		return nil, err
	}
	provider.token = &token
	return provider, nil
}

// RequestSize returns the exact encoded request size for this provider codec.
func (p *Provider) RequestSize(request jev.Request) (int, error) {
	if p == nil || p.codec == nil {
		return 0, jev.ErrInvalidRequest
	}
	if err := jev.ValidateRequest(request); err != nil {
		return 0, err
	}
	body, err := p.codec.encode(request)
	if err != nil {
		return 0, err
	}
	return len(body), nil
}

func (p *Provider) Evaluate(ctx context.Context, request jev.Request) (jev.Response, error) {
	if err := jev.ValidateRequest(request); err != nil {
		return jev.Response{}, err
	}
	body, err := p.codec.encode(request)
	if err != nil {
		return jev.Response{}, err
	}
	if len(body) > maxRequestBytes {
		return jev.Response{}, ErrBodyLimit
	}
	var token string
	if p.token != nil {
		token = *p.token
	} else {
		token = p.endpoint.token()
	}
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
			response.Usage.RequestCount = attempt + 1
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
		defer func() { _ = httpResponse.Body.Close() }()
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
	response, err := p.codec.decode(responseBody, request)
	if err != nil {
		return jev.Response{}, false, 0, err
	}
	response.Usage.CostUSD = p.endpoint.costUSD(response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.CostUSD)
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
