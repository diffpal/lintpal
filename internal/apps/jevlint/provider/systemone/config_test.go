package systemone

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPresetDestinationsAndTokenSources(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "native-secret")
	t.Setenv("OPENROUTER_API_KEY", "router-secret")
	for _, tc := range []struct {
		name  string
		value Endpoint
		url   string
		token string
	}{
		{"typesafe", TypeSafe(), typeSafeBase + "/v1/systemone", "native-secret"},
		{"openrouter", OpenRouter(), openRouterBase + "/v1/systemone", "router-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, err := tc.value.target()
			if err != nil || target.String() != tc.url || tc.value.token() != tc.token {
				t.Fatalf("target = %v, token matched = %v, error = %v", target, tc.value.token() == tc.token, err)
			}
		})
	}
}

func TestCustomRequiresExplicitTokenSource(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "native-secret")
	t.Setenv("OPENROUTER_API_KEY", "router-secret")
	t.Setenv("JEVLINT_TOKEN", "custom-secret")
	endpoint, err := TrustedCustom("http://127.0.0.1:9090/api", "")
	if err != nil || endpoint.token() != "" {
		t.Fatalf("uncredentialed custom: error = %v, token present = %v", err, endpoint.token() != "")
	}
	target, err := endpoint.target()
	if err != nil || target.String() != "http://127.0.0.1:9090/api/v1/systemone" {
		t.Fatalf("custom target = %v, error = %v", target, err)
	}
	endpoint, err = TrustedCustom("https://jev.example.test", "JEVLINT_TOKEN")
	if err != nil || endpoint.token() != "custom-secret" {
		t.Fatalf("trusted custom token source: error = %v, matched = %v", err, endpoint.token() == "custom-secret")
	}
}

func TestRejectUnsafeEndpoint(t *testing.T) {
	for _, raw := range []string{"http://evil.example", "https://user:secret@evil.example", "https://evil.example/?token=x", "https://evil.example/#fragment", "https://evil.example/v1/systemone", "https://evil.example/../other", "ftp://evil.example", ""} {
		if _, err := TrustedCustom(raw, "JEVLINT_TOKEN"); !errors.Is(err, ErrInvalidEndpoint) {
			t.Fatalf("URL %q error = %v", raw, err)
		}
	}
	if _, err := TrustedCustom("https://example.test", "BAD-NAME"); !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("invalid token env name error = %v", err)
	}
	for _, presetEnv := range []string{"TYPESAFE_API_KEY", "OPENROUTER_API_KEY"} {
		if _, err := TrustedCustom("https://example.test", presetEnv); !errors.Is(err, ErrInvalidEndpoint) {
			t.Fatalf("custom endpoint accepted preset token source %s: %v", presetEnv, err)
		}
	}
}

func TestSecureClientNeverFollowsRedirect(t *testing.T) {
	redirected := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected = true
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer source.Close()
	response, err := secureClient(nil).Get(source.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if redirected || response.StatusCode != http.StatusFound {
		t.Fatalf("redirected = %v, status = %d", redirected, response.StatusCode)
	}
}
