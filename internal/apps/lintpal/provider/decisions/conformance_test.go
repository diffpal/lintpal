package decisions

import (
	"encoding/json"
	"errors"
	"io"
	"math"
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
	for _, tc := range []struct {
		name     string
		endpoint Endpoint
		url      string
		token    string
		costUSD  *float64
	}{
		{"typesafe", TypeSafe(), "https://api.typesafe.ai/v1/systemone", "native-secret", func() *float64 { value := 12 * typeSafeInputUSDPerMillion / 1_000_000; return &value }()},
		{"openrouter", OpenRouter(), "https://openrouter.ai/api/alpha/decisions", "router-secret", nil},
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
			if err != nil || calls != 1 || len(response.Answers) != 3 ||
				(response.Usage.CostUSD == nil) != (tc.costUSD == nil) ||
				response.Usage.CostUSD != nil && *response.Usage.CostUSD != *tc.costUSD {
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

// Mirrors the official OpenAI Decisions response shape documented on 2026-10-06.
const openAIResponse = `{"model":"gpt-6-luna","answers":[{"type":"predicate","name":"n","probability":0.7},{"type":"choice","name":"c","choice":"risk","probabilities":[{"value":"safe","probability":0.2},{"value":"risk","probability":0.8}],"confidence":0.6},{"type":"score","name":"s","score":0.8,"probabilities":[{"label":"0","value":0,"probability":0.2},{"label":"1","value":1,"probability":0.8}],"confidence":0.6}],"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":10},"output_tokens":5,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":105}}`

func TestOpenAIDecisionsContract(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "unselected-native-secret")
	request := typedRequest()
	request.Model = "gpt-6-luna"
	request.State = "safe bounded state"
	calls := 0
	actualSize := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.URL.String() != "https://api.openai.com/v1/decisions" || r.Header.Get("Authorization") != "Bearer openai-secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("wrong Decisions destination/headers: %s %s", r.Method, r.URL)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		actualSize = len(body)
		var payload openAIRequestWire
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != request.Model || payload.Input != "safe bounded state" || len(payload.Questions) != 3 ||
			payload.Questions[0].Name != "c" || payload.Questions[0].Type != "choice" ||
			payload.Questions[1].Name != "n" || payload.Questions[1].Type != "predicate" ||
			payload.Questions[2].Name != "s" || payload.Questions[2].Type != "score" {
			t.Fatalf("invalid Decisions payload: %+v", payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(openAIResponse))}, nil
	})}
	provider, err := NewWithToken(OpenAI(), client, "openai-secret")
	if err != nil {
		t.Fatal(err)
	}
	wantSize, err := provider.RequestSize(request)
	if err != nil {
		t.Fatal(err)
	}
	response, err := provider.Evaluate(t.Context(), request)
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if actualSize != wantSize || response.Model != "gpt-6-luna" || response.Usage.InputTokens != 100 || response.Usage.OutputTokens != 5 ||
		response.Usage.CostUSD == nil || math.Abs(*response.Usage.CostUSD-0.00001095) > 1e-15 || len(response.Answers) != 3 {
		t.Fatalf("normalization: %+v", response)
	}
	if response.Answers["n"].(jev.NoulAnswer).Probability != .7 || response.Answers["c"].(jev.ChoiceAnswer).Choice != "risk" || response.Answers["s"].(jev.ScoreAnswer).Score != .8 {
		t.Fatalf("typed answers: %+v", response.Answers)
	}
	for _, body := range []string{
		strings.Replace(openAIResponse, `"type":"predicate"`, `"type":"unsupported"`, 1),
		strings.Replace(openAIResponse, `"probability":0.7`, `"probability":1.7`, 1),
		strings.Replace(openAIResponse, `"input_tokens":100`, `"input_tokens":-1`, 1),
		strings.Replace(openAIResponse, `{"type":"predicate","name":"n","probability":0.7},`, ``, 1),
		strings.Replace(openAIResponse, `"type":"predicate"`, `"type":"refusal"`, 1),
		strings.Replace(openAIResponse, `"name":"n"`, `"name":null`, 1),
		strings.Replace(openAIResponse, `"name":"n"`, `"name":"unknown"`, 1),
		strings.Replace(openAIResponse, `"name":"n"`, `"name":"c"`, 1),
		strings.Replace(openAIResponse, `"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":10},`, ``, 1),
	} {
		if _, err := (openAICodec{}).decode([]byte(body), request); !errors.Is(err, ErrProtocol) {
			t.Fatalf("invalid Decisions envelope accepted: %v", err)
		}
	}
}
