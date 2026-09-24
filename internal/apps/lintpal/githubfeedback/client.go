package githubfeedback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrGitHubAPI = errors.New("GitHub API request failed")
var ErrUnsafeFork = errors.New("GitHub publication disabled for fork pull request")

const maxResponseBytes = 2 << 20
const maxRequestBytes = 2 << 20

type Publisher struct {
	apiBase string
	client  *http.Client
}

func NewPublisher(apiBase string, client *http.Client) (*Publisher, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(apiBase), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, ErrInvalidContext
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Publisher{apiBase: parsed.String(), client: client}, nil
}

func (publisher *Publisher) ActiveFindings(ctx context.Context, token string, reviewCtx Context, identity Identity) (map[string]string, error) {
	owner, name, ok := strings.Cut(reviewCtx.Repo, "/")
	if !ok || owner == "" || name == "" || reviewCtx.PRNumber <= 0 {
		return nil, ErrInvalidContext
	}
	const query = `query($owner:String!,$name:String!,$number:Int!,$cursor:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100,after:$cursor){nodes{isResolved comments(first:100){nodes{body}}}pageInfo{hasNextPage endCursor}}}}}`
	active := map[string]string{}
	var cursor any
	for {
		request := map[string]any{"query": query, "variables": map[string]any{
			"owner": owner, "name": name, "number": reviewCtx.PRNumber, "cursor": cursor,
		}}
		var response struct {
			Data struct {
				Repository struct {
					PullRequest struct {
						ReviewThreads struct {
							Nodes []struct {
								IsResolved bool `json:"isResolved"`
								Comments   struct {
									Nodes []struct {
										Body string `json:"body"`
									} `json:"nodes"`
								} `json:"comments"`
							} `json:"nodes"`
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"reviewThreads"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
			Errors []json.RawMessage `json:"errors"`
		}
		if err := publisher.doJSON(ctx, token, http.MethodPost, publisher.apiBase+"/graphql", request, &response); err != nil {
			return nil, err
		}
		if len(response.Errors) != 0 {
			return nil, ErrGitHubAPI
		}
		threads := response.Data.Repository.PullRequest.ReviewThreads
		for _, thread := range threads.Nodes {
			if thread.IsResolved {
				continue
			}
			for _, comment := range thread.Comments.Nodes {
				if id, digest := findingIdentity(comment.Body, identity); id != "" && digest != "" {
					active[id] = digest
				}
			}
		}
		if !threads.PageInfo.HasNextPage {
			return active, nil
		}
		if threads.PageInfo.EndCursor == "" {
			return nil, ErrGitHubAPI
		}
		cursor = threads.PageInfo.EndCursor
	}
}

func (publisher *Publisher) Publish(ctx context.Context, token string, reviewCtx Context, identity Identity, result string, plan Plan) error {
	if reviewCtx.UnsafeFork {
		return ErrUnsafeFork
	}
	if !validRepo(reviewCtx.Repo) || reviewCtx.PRNumber <= 0 || reviewCtx.HeadSHA == "" || strings.TrimSpace(token) == "" {
		return ErrInvalidContext
	}
	reviewsURL := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews", publisher.apiBase, reviewCtx.Repo, reviewCtx.PRNumber)
	existingID, existingBody, err := publisher.findResult(ctx, token, reviewsURL, identity.ResultMarker(reviewCtx.HeadSHA))
	if err != nil {
		return err
	}
	body := identity.ResultMarker(reviewCtx.HeadSHA) + "\n" + strings.TrimSpace(result) + "\n"
	comments := make([]map[string]any, 0, len(plan.Comments))
	for _, comment := range plan.Comments {
		if comment.FindingID == "" || comment.Digest == "" || comment.Path == "" || comment.StartLine < 1 ||
			comment.EndLine < comment.StartLine || (comment.Side != "LEFT" && comment.Side != "RIGHT") {
			return ErrInvalidFinding
		}
		item := map[string]any{"path": comment.Path, "line": comment.EndLine, "side": comment.Side,
			"body": strings.TrimSpace(comment.Body) + "\n" + identity.FindingMarker(comment.FindingID, comment.Digest)}
		if comment.EndLine > comment.StartLine {
			item["start_line"] = comment.StartLine
			item["start_side"] = comment.Side
		}
		comments = append(comments, item)
	}
	if existingID > 0 && len(comments) == 0 && strings.TrimSpace(existingBody) == strings.TrimSpace(body) {
		return nil
	}
	payload := map[string]any{"commit_id": reviewCtx.HeadSHA, "event": "COMMENT", "body": body}
	if len(comments) != 0 {
		payload["comments"] = comments
	}
	return publisher.doJSON(ctx, token, http.MethodPost, reviewsURL, payload, nil)
}

func (publisher *Publisher) findResult(ctx context.Context, token, firstURL, marker string) (int64, string, error) {
	next := firstURL + "?per_page=100"
	var found int64
	var foundBody string
	for next != "" {
		var reviews []struct {
			ID    int64  `json:"id"`
			Body  string `json:"body"`
			State string `json:"state"`
		}
		headers, err := publisher.getJSON(ctx, token, next, &reviews)
		if err != nil {
			return 0, "", err
		}
		for _, review := range reviews {
			if strings.EqualFold(review.State, "COMMENTED") && strings.HasPrefix(strings.TrimSpace(review.Body), marker) {
				found = review.ID
				foundBody = review.Body
			}
		}
		next, err = trustedNext(headers.Get("Link"), next)
		if err != nil {
			return 0, "", err
		}
	}
	return found, foundBody, nil
}

func (publisher *Publisher) doJSON(ctx context.Context, token, method, target string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil || len(data) > maxRequestBytes {
			return ErrGitHubAPI
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return ErrGitHubAPI
	}
	setHeaders(req, token)
	resp, err := publisher.client.Do(req)
	if err != nil {
		return ErrGitHubAPI
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrGitHubAPI, resp.StatusCode)
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return nil
	}
	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil || len(data) > maxResponseBytes || json.Unmarshal(data, output) != nil {
		return ErrGitHubAPI
	}
	return nil
}

func (publisher *Publisher) getJSON(ctx context.Context, token, target string, output any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, ErrGitHubAPI
	}
	setHeaders(req, token)
	resp, err := publisher.client.Do(req)
	if err != nil {
		return nil, ErrGitHubAPI
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrGitHubAPI, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes || json.Unmarshal(data, output) != nil {
		return nil, ErrGitHubAPI
	}
	return resp.Header.Clone(), nil
}

func setHeaders(request *http.Request, token string) {
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/json")
}

func trustedNext(header, current string) (string, error) {
	currentURL, err := url.Parse(current)
	if err != nil {
		return "", ErrGitHubAPI
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) || !strings.HasPrefix(part, "<") {
			continue
		}
		end := strings.Index(part, ">")
		if end < 2 {
			return "", ErrGitHubAPI
		}
		nextURL, err := url.Parse(part[1:end])
		if err != nil {
			return "", ErrGitHubAPI
		}
		if !nextURL.IsAbs() {
			nextURL = currentURL.ResolveReference(nextURL)
		}
		if !strings.EqualFold(nextURL.Scheme, currentURL.Scheme) || !strings.EqualFold(nextURL.Host, currentURL.Host) {
			return "", ErrGitHubAPI
		}
		return nextURL.String(), nil
	}
	return "", nil
}

func findingIdentity(body string, identity Identity) (string, string) {
	prefix := "<!-- lintpal:finding:" + identity.Channel + " id:"
	start := strings.Index(body, prefix)
	if start < 0 {
		return "", ""
	}
	remainder := body[start+len(prefix):]
	end := strings.Index(remainder, " -->")
	if end < 0 {
		return "", ""
	}
	fields := strings.Fields(remainder[:end])
	if len(fields) != 2 || !strings.HasPrefix(fields[1], "digest:") {
		return "", ""
	}
	return fields[0], strings.TrimPrefix(fields[1], "digest:")
}
