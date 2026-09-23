package systemone

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testProvider(t *testing.T, server *httptest.Server) *Provider {
	t.Helper()
	endpoint, err := TrustedCustom(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := New(endpoint, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestHTTPRetryClasses(t *testing.T) {
	for _, tc := range []struct {
		status   int
		attempts int
	}{
		{400, 1}, {401, 1}, {403, 1}, {404, 1}, {413, 1}, {422, 1},
		{429, 2}, {529, 2}, {500, 2}, {503, 2},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++
				if attempts < tc.attempts {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, "secret provider body")
					return
				}
				if tc.attempts == 1 {
					w.WriteHeader(tc.status)
					return
				}
				_, _ = io.WriteString(w, validResponse)
			}))
			defer server.Close()
			_, err := testProvider(t, server).Evaluate(t.Context(), typedRequest())
			if attempts != tc.attempts {
				t.Fatalf("attempts = %d, want %d", attempts, tc.attempts)
			}
			if tc.attempts == 1 {
				var status HTTPError
				if !errors.As(err, &status) || status.Status != tc.status || strings.Contains(err.Error(), "secret") {
					t.Fatalf("status error = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRetryCapAndCancellationDuringBackoff(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	provider := testProvider(t, server)
	_, err := provider.Evaluate(t.Context(), typedRequest())
	var status HTTPError
	if attempts != maxAttempts || !errors.As(err, &status) || status.Status != 429 {
		t.Fatalf("attempts = %d, error = %v", attempts, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready := make(chan struct{})
	waitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		close(ready)
	}))
	defer waitServer.Close()
	provider = testProvider(t, waitServer)
	done := make(chan error, 1)
	go func() { _, err := provider.Evaluate(ctx, typedRequest()); done <- err }()
	<-ready
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled backoff error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("backoff ignored cancellation")
	}
}

func TestBodyLimitsAndRedactedErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "token=private-token state=private-source question=private-question")
	}))
	defer server.Close()
	provider := testProvider(t, server)
	request := typedRequest()
	request.State = "private-source"
	_, err := provider.Evaluate(t.Context(), request)
	for _, secret := range []string{"private-token", "private-source", "private-question"} {
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatalf("sensitive error = %v", err)
		}
	}
	request.State = strings.Repeat("x", maxRequestBytes)
	if _, err := provider.Evaluate(t.Context(), request); !errors.Is(err, ErrBodyLimit) {
		t.Fatalf("request cap error = %v", err)
	}
	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", maxResponseBytes+1))
	}))
	defer large.Close()
	if _, err := testProvider(t, large).Evaluate(t.Context(), typedRequest()); !errors.Is(err, ErrBodyLimit) {
		t.Fatalf("response cap error = %v", err)
	}
}

func TestParentDeadlineStopsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	_, err := testProvider(t, server).Evaluate(ctx, typedRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
}

func TestPerAttemptTimeoutIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	provider := testProvider(t, server)
	provider.timeout = 10 * time.Millisecond
	start := time.Now()
	_, err := provider.Evaluate(t.Context(), typedRequest())
	if !errors.Is(err, ErrTransport) || time.Since(start) > time.Second {
		t.Fatalf("attempt timeout error = %v, elapsed = %s", err, time.Since(start))
	}
}

func TestRetryAfterCaps(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"0", 0}, {"999999999", maxRetryDelay}, {"-1", -1}, {"invalid", -1},
		{now.Add(10 * time.Second).UTC().Format(http.TimeFormat), maxRetryDelay},
	} {
		if got := retryAfter(tc.value, now); got != tc.want {
			t.Fatalf("Retry-After %q = %s, want %s", tc.value, got, tc.want)
		}
	}
}

type failingRoundTripper struct{ calls int }

func (f *failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls++
	return nil, errors.New("private-token and private-source from network")
}

func TestTransientNetworkErrorIsRetriedAndRedacted(t *testing.T) {
	transport := new(failingRoundTripper)
	endpoint, err := TrustedCustom("https://jev.example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := New(endpoint, &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Evaluate(t.Context(), typedRequest())
	if transport.calls != maxAttempts || !errors.Is(err, ErrTransport) || strings.Contains(err.Error(), "private-") {
		t.Fatalf("network attempts = %d, error = %v", transport.calls, err)
	}
}
