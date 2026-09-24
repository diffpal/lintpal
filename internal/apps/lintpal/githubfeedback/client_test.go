package githubfeedback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublisherCreatesLeftMultilineReviewAndSkipsUnchangedRerun(t *testing.T) {
	identity, _ := NewIdentity("")
	var posted map[string]any
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("missing auth header")
		}
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/reviews"):
			if posted == nil {
				_, _ = writer.Write([]byte(`[]`))
				return
			}
			body := posted["body"].(string)
			_, _ = fmt.Fprintf(writer, `[{"id":41,"body":%q,"state":"COMMENTED"}]`, body)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/reviews"):
			posts++
			if err := json.NewDecoder(request.Body).Decode(&posted); err != nil {
				t.Fatal(err)
			}
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	publisher, err := NewPublisher(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	reviewCtx := Context{Repo: "owner/repo", PRNumber: 7, HeadSHA: "head"}
	plan := Plan{Comments: []Comment{{FindingID: "finding-1", Body: "stored body", Path: "old.go", StartLine: 3, EndLine: 5, Side: "LEFT", Digest: "digest"}}}
	if err := publisher.Publish(context.Background(), "secret-token", reviewCtx, identity, "No blocking findings", plan); err != nil {
		t.Fatal(err)
	}
	comments := posted["comments"].([]any)
	comment := comments[0].(map[string]any)
	if comment["side"] != "LEFT" || comment["start_side"] != "LEFT" || comment["line"] != float64(5) || comment["start_line"] != float64(3) || !strings.Contains(comment["body"].(string), "lintpal:finding") {
		t.Fatalf("wrong inline payload: %+v", comment)
	}
	if err := publisher.Publish(context.Background(), "secret-token", reviewCtx, identity, "No blocking findings", Plan{}); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("expected unchanged rerun to skip publication, got %d posts", posts)
	}
}

func TestPublisherRejectsCrossOriginPaginationAndRedactsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Link", `<https://evil.example/reviews?page=2>; rel="next"`)
		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()
	publisher, _ := NewPublisher(server.URL, server.Client())
	identity, _ := NewIdentity("")
	err := publisher.Publish(context.Background(), "secret-token", Context{Repo: "owner/repo", PRNumber: 1, HeadSHA: "head"}, identity, "result", Plan{})
	if !errors.Is(err, ErrGitHubAPI) || strings.Contains(fmt.Sprint(err), "secret-token") || strings.Contains(fmt.Sprint(err), "evil.example") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublisherSkipsUnsafeForkBeforeNetwork(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	publisher, _ := NewPublisher(server.URL, server.Client())
	identity, _ := NewIdentity("")
	err := publisher.Publish(context.Background(), "token", Context{Repo: "owner/repo", PRNumber: 1, HeadSHA: "head", UnsafeFork: true}, identity, "result", Plan{})
	if !errors.Is(err, ErrUnsafeFork) || requests != 0 {
		t.Fatalf("unsafe fork was not skipped: requests=%d err=%v", requests, err)
	}
}

func TestActiveFindingsPaginatesAndIgnoresResolved(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		page++
		if page == 1 {
			_, _ = writer.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"isResolved":false,"comments":{"nodes":[{"body":"x <!-- lintpal:finding:lintpal id:one digest:first -->"}]}}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor"}}}}}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"isResolved":true,"comments":{"nodes":[{"body":"<!-- lintpal:finding:lintpal id:ignored digest:nope -->"}]}},{"isResolved":false,"comments":{"nodes":[{"body":"<!-- lintpal:finding:lintpal id:two digest:second -->"}]}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}}`))
	}))
	defer server.Close()
	publisher, _ := NewPublisher(server.URL, server.Client())
	identity, _ := NewIdentity("")
	ids, err := publisher.ActiveFindings(context.Background(), "token", Context{Repo: "owner/repo", PRNumber: 1}, identity)
	if err != nil || len(ids) != 2 {
		t.Fatalf("unexpected active IDs: %+v %v", ids, err)
	}
	if _, ok := ids["one"]; !ok {
		t.Fatal("missing first page ID")
	}
	if _, ok := ids["two"]; !ok {
		t.Fatal("missing second page ID")
	}
	if ids["one"] != "first" || ids["two"] != "second" {
		t.Fatalf("wrong digests: %+v", ids)
	}
}
