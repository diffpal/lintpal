package decisions

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestPresetAndCustomConformance(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "native-secret")
	t.Setenv("OPENROUTER_API_KEY", "router-secret")
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	for _, tc := range []struct {
		name     string
		endpoint Endpoint
		url      string
		token    string
	}{
		{"typesafe", TypeSafe(), "https://api.typesafe.ai/v1/systemone", "native-secret"},
		{"openrouter", OpenRouter(), "https://openrouter.ai/api/alpha/decisions", "router-secret"},
		{"openai", OpenAI(), "https://api.openai.com/v1/decisions", "openai-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Method != http.MethodPost || request.URL.String() != tc.url || request.Header.Get("Authorization") != "Bearer "+tc.token {
					t.Errorf("unexpected preset request URL/headers: %s %s", request.Method, request.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(validResponse))}, nil
			})}
			provider, err := New(tc.endpoint, client)
			if err != nil {
				t.Fatal(err)
			}
			response, err := provider.Evaluate(t.Context(), typedRequest())
			if err != nil || calls != 1 || len(response.Answers) != 3 {
				t.Fatalf("calls = %d, response = %+v, error = %v", calls, response, err)
			}
		})
	}

	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()
	endpoint, err := TrustedCustom(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := New(endpoint, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Evaluate(t.Context(), typedRequest()); err != nil || receivedAuth != "" {
		t.Fatalf("custom inherited preset credential: auth present = %v, error = %v", receivedAuth != "", err)
	}
}

func TestCredentialNeverFollowsRedirect(t *testing.T) {
	forwarded := false
	destination := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		forwarded = r.Header.Get("Authorization") != ""
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer custom-secret" {
			t.Error("source did not receive configured token")
		}
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	t.Setenv("LINTPAL_TOKEN", "custom-secret")
	endpoint, err := TrustedCustom(source.URL, "LINTPAL_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := New(endpoint, source.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Evaluate(t.Context(), typedRequest())
	var status HTTPError
	if !errors.As(err, &status) || status.Status != http.StatusTemporaryRedirect || forwarded {
		t.Fatalf("redirect error = %v, forwarded = %v", err, forwarded)
	}
}

func TestBoundTokenOverridesProcessEnvironment(t *testing.T) {
	t.Setenv("LINTPAL_TOKEN", "process-secret")
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()
	endpoint, err := TrustedCustom(server.URL, "LINTPAL_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewWithToken(endpoint, server.Client(), "file-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Evaluate(t.Context(), typedRequest()); err != nil {
		t.Fatal(err)
	}
	if receivedAuth != "Bearer file-secret" || os.Getenv("LINTPAL_TOKEN") != "process-secret" {
		t.Fatalf("bound token not used or process environment changed: auth matched = %v", receivedAuth == "Bearer file-secret")
	}
}

func TestWireRejectsNumericAndEnvelopeErrors(t *testing.T) {
	request := typedRequest()
	for _, body := range []string{
		`{"model":"jev","answers":{"n":{"type":"noul","noul":1.2},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		`{"model":"jev","answers":{"n":{"type":"noul","noul":0.5},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.2},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		`{"model":"jev","answers":{"n":{"type":"noul","noul":0.5},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":2.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		`{"model":"jev","answers":{"n":{"type":"noul","noul":0.5},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}},"usage":{"input_tokens":-1,"output_tokens":1}}`,
		`{"model":"jev","answers":{"n":{"type":"noul","noul":0.5},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}}}`,
	} {
		if _, err := decodeResponse([]byte(body), request); !errors.Is(err, ErrProtocol) {
			t.Fatalf("invalid wire accepted: error = %v", err)
		}
	}
}

func TestProviderDoesNotExposeADKModel(t *testing.T) {
	var _ jev.Provider = (*Provider)(nil)
}

// Mirrors the documented Decisions envelope, including optional service metadata.
const openRouterResponse = `{"id":"gen-decision-fixture","provider":"TypeSafe","model":"typesafe/jev-1.13-20260917","answers":{"n":{"type":"noul","noul":0.7},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}},"usage":{"input_tokens":12,"output_tokens":3,"cost":0.00002}}`

func TestOpenRouterDecisionsContract(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "unselected-native-secret")
	request := typedRequest()
	request.Model = "typesafe/jev-1.13"
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.URL.String() != "https://openrouter.ai/api/alpha/decisions" || r.Header.Get("Authorization") != "Bearer router-secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("wrong Decisions destination/headers: %s %s", r.Method, r.URL)
		}
		var payload struct {
			Model     string
			State     map[string]any
			Questions map[string]map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != request.Model || payload.State["file"] != "safe bounded state" || len(payload.Questions) != 3 || payload.Questions["n"]["type"] != "noul" || payload.Questions["c"]["type"] != "choice" || payload.Questions["s"]["type"] != "score" {
			t.Fatalf("invalid Decisions payload: %+v", payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(openRouterResponse))}, nil
	})}
	provider, err := NewWithToken(OpenRouter(), client, "router-secret")
	if err != nil {
		t.Fatal(err)
	}
	response, err := provider.Evaluate(t.Context(), request)
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if response.Model != "typesafe/jev-1.13-20260917" || response.Usage.InputTokens != 12 || response.Usage.OutputTokens != 3 || response.Usage.CostUSD == nil || *response.Usage.CostUSD != 0.00002 || len(response.Answers) != 3 {
		t.Fatalf("normalization: %+v", response)
	}
	if response.Answers["n"].(jev.NoulAnswer).Probability != .7 || response.Answers["c"].(jev.ChoiceAnswer).Choice != "risk" || response.Answers["s"].(jev.ScoreAnswer).Score != .8 {
		t.Fatalf("typed answers: %+v", response.Answers)
	}
	for _, body := range []string{
		strings.Replace(openRouterResponse, `"type":"noul"`, `"type":"unsupported"`, 1),
		strings.Replace(openRouterResponse, `"noul":0.7`, `"noul":1.7`, 1),
		strings.Replace(openRouterResponse, `"input_tokens":12`, `"input_tokens":-1`, 1),
		strings.Replace(openRouterResponse, `"n":{"type":"noul","noul":0.7},`, ``, 1),
	} {
		if _, err := decodeResponse([]byte(body), request); !errors.Is(err, ErrProtocol) {
			t.Fatalf("invalid Decisions envelope accepted: %v", err)
		}
	}
}

// Fixture for the operator-assumed compatible OpenAI contract, not a live response.
const openAIResponse = `{"id":"gen-decision-fixture","provider":"TypeSafe","model":"decision-test-model-20260917","answers":{"n":{"type":"noul","noul":0.7},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}},"usage":{"input_tokens":12,"output_tokens":3,"cost":0.00002}}`

func TestOpenAIAssumedDecisionsContract(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "unselected-native-secret")
	request := typedRequest()
	request.Model = "decision-test-model"
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.URL.String() != "https://api.openai.com/v1/decisions" || r.Header.Get("Authorization") != "Bearer openai-secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("wrong Decisions destination/headers: %s %s", r.Method, r.URL)
		}
		var payload struct {
			Model     string
			State     map[string]any
			Questions map[string]map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != request.Model || payload.State["file"] != "safe bounded state" || len(payload.Questions) != 3 || payload.Questions["n"]["type"] != "noul" || payload.Questions["c"]["type"] != "choice" || payload.Questions["s"]["type"] != "score" {
			t.Fatalf("invalid Decisions payload: %+v", payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(openAIResponse))}, nil
	})}
	provider, err := NewWithToken(OpenAI(), client, "openai-secret")
	if err != nil {
		t.Fatal(err)
	}
	response, err := provider.Evaluate(t.Context(), request)
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if response.Model != "decision-test-model-20260917" || response.Usage.InputTokens != 12 || response.Usage.OutputTokens != 3 || response.Usage.CostUSD == nil || *response.Usage.CostUSD != 0.00002 || len(response.Answers) != 3 {
		t.Fatalf("normalization: %+v", response)
	}
	if response.Answers["n"].(jev.NoulAnswer).Probability != .7 || response.Answers["c"].(jev.ChoiceAnswer).Choice != "risk" || response.Answers["s"].(jev.ScoreAnswer).Score != .8 {
		t.Fatalf("typed answers: %+v", response.Answers)
	}
	for _, body := range []string{
		strings.Replace(openAIResponse, `"type":"noul"`, `"type":"unsupported"`, 1),
		strings.Replace(openAIResponse, `"noul":0.7`, `"noul":1.7`, 1),
		strings.Replace(openAIResponse, `"input_tokens":12`, `"input_tokens":-1`, 1),
		strings.Replace(openAIResponse, `"n":{"type":"noul","noul":0.7},`, ``, 1),
	} {
		if _, err := decodeResponse([]byte(body), request); !errors.Is(err, ErrProtocol) {
			t.Fatalf("invalid Decisions envelope accepted: %v", err)
		}
	}
}
