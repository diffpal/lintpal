package di

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/cli"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

type providerRoundTrip func(*http.Request) (*http.Response, error)

func (f providerRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConfiguredProviderDestinations(t *testing.T) {
	for _, tc := range []struct {
		options cli.Options
		url     string
	}{
		{cli.Options{Provider: "openai"}, "https://api.openai.com/v1/decisions"},
		{cli.Options{Provider: "custom", BaseURL: "https://example.test/api/", APIPath: "/decisions", AuthTokenEnv: "MY_KEY"}, "https://example.test/api/decisions"},
		{cli.Options{Provider: "custom", BaseURL: "https://example.test/api", AuthTokenEnv: "MY_KEY"}, "https://example.test/api/v1/systemone"},
	} {
		t.Run(tc.options.Provider+tc.options.APIPath, func(t *testing.T) {
			tc.options.Credential = "bound-secret"
			tc.options.CredentialResolved = true
			calls := 0
			client := &http.Client{Transport: providerRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != tc.url || r.Header.Get("Authorization") != "Bearer bound-secret" {
					t.Error("wrong destination or bound credential")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"model":"test-model","answers":{"check":{"type":"noul","noul":0.3}},"usage":{"input_tokens":1,"output_tokens":1}}`))}, nil
			})}
			provider, err := newProvider(tc.options, client)
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Evaluate(t.Context(), jev.Request{Model: "test-model", State: "state", Questions: map[string]jev.Question{"check": jev.NoulQuestion{Instructions: "Check state"}}})
			if err != nil || calls != 1 {
				t.Fatalf("calls = %d, error = %v", calls, err)
			}
		})
	}
}
