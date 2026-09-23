package systemone

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
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
	}{
		{"typesafe", TypeSafe(), typeSafeBase + "/v1/systemone", "native-secret"},
		{"openrouter", OpenRouter(), openRouterBase + "/v1/systemone", "router-secret"},
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
	t.Setenv("JEVLINT_TOKEN", "custom-secret")
	endpoint, err := TrustedCustom(source.URL, "JEVLINT_TOKEN")
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
