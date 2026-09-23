package systemone

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

func typedRequest() jev.Request {
	return jev.Request{
		Model: "jev-1.13",
		State: map[string]any{"file": "safe bounded state"},
		Questions: map[string]jev.Question{
			"n": jev.NoulQuestion{Instructions: "Is it risky?"},
			"c": jev.ChoiceQuestion{Instructions: "Which class?", Criteria: map[string]string{"safe": "safe", "risk": "risk"}},
			"s": jev.ScoreQuestion{Instructions: "How severe?", Criteria: []string{"low", "high"}},
		},
	}
}

const validResponse = `{"model":"jev-1.13.0","answers":{"n":{"type":"noul","noul":0.7},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6}},"usage":{"input_tokens":12,"output_tokens":3}}`

func TestTypedSystemOneRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Authorization") != "Bearer custom-secret" {
			t.Errorf("unexpected request method/path/headers: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var request struct {
			State     map[string]any            `json:"state"`
			Model     string                    `json:"model"`
			Questions map[string]map[string]any `json:"questions"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
		}
		if request.Model != "jev-1.13" || request.State["file"] != "safe bounded state" ||
			request.Questions["n"]["type"] != "noul" || request.Questions["c"]["type"] != "choice" || request.Questions["s"]["type"] != "score" {
			t.Errorf("wrong typed request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()
	t.Setenv("LINTPAL_TOKEN", "custom-secret")
	endpoint, err := TrustedCustom(server.URL, "LINTPAL_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := New(endpoint, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	response, err := provider.Evaluate(t.Context(), typedRequest())
	if err != nil {
		t.Fatal(err)
	}
	if response.Model != "jev-1.13.0" || len(response.Answers) != 3 || response.Usage.InputTokens != 12 {
		t.Fatalf("response = %+v", response)
	}
}

func TestRejectInvalidSystemOneResponse(t *testing.T) {
	request := typedRequest()
	for _, body := range []string{
		`{"model":"jev","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`,
		`{"model":"jev","answers":{"n":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		`{"model":"jev","answers":{"n":{"type":"noul","noul":0.5},"c":{"type":"choice","choice":"risk","probabilities":{"safe":0.2,"risk":0.8},"confidence":0.6},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.6},"extra":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		`{"model":"jev","answers":{"n":{"type":"choice","choice":"risk"}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		`not-json`,
	} {
		if _, err := decodeResponse([]byte(body), request); !errors.Is(err, ErrProtocol) {
			t.Fatalf("body %q error = %v", body, err)
		}
	}
}

func TestInvalidRequestNeverCallsServer(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	endpoint, err := TrustedCustom(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := New(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Evaluate(context.Background(), jev.Request{Model: "jev", State: 4})
	if !errors.Is(err, jev.ErrInvalidRequest) || called {
		t.Fatalf("invalid request error = %v, called = %v", err, called)
	}
}
