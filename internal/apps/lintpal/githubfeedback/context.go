package githubfeedback

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

var ErrInvalidContext = errors.New("invalid GitHub feedback context")

const maxEventBytes = 2 << 20

type Context struct {
	Repo       string
	PRNumber   int
	BaseSHA    string
	HeadSHA    string
	BaseRepo   string
	HeadRepo   string
	UnsafeFork bool
}

type ContextOptions struct {
	Repo, BaseSHA, HeadSHA string
	PRNumber               int
	EventPath              string
}

func ResolveContext(options ContextOptions) (Context, error) {
	ctx := Context{Repo: strings.TrimSpace(options.Repo), PRNumber: options.PRNumber,
		BaseSHA: strings.TrimSpace(options.BaseSHA), HeadSHA: strings.TrimSpace(options.HeadSHA)}
	if ctx.Repo == "" {
		ctx.Repo = strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY"))
	}
	eventPath := strings.TrimSpace(options.EventPath)
	if eventPath == "" {
		eventPath = strings.TrimSpace(os.Getenv("GITHUB_EVENT_PATH"))
	}
	if eventPath != "" {
		file, err := os.Open(eventPath)
		if err != nil {
			return Context{}, ErrInvalidContext
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxEventBytes+1))
		_ = file.Close()
		if readErr != nil || len(data) > maxEventBytes {
			return Context{}, ErrInvalidContext
		}
		var event struct {
			Number     int `json:"number"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			PullRequest struct {
				Number int `json:"number"`
				Base   struct {
					SHA  string `json:"sha"`
					Repo struct {
						FullName string `json:"full_name"`
					} `json:"repo"`
				} `json:"base"`
				Head struct {
					SHA  string `json:"sha"`
					Repo struct {
						FullName string `json:"full_name"`
					} `json:"repo"`
				} `json:"head"`
			} `json:"pull_request"`
		}
		if json.Unmarshal(data, &event) != nil {
			return Context{}, ErrInvalidContext
		}
		if ctx.Repo == "" {
			ctx.Repo = event.Repository.FullName
		}
		if ctx.PRNumber == 0 {
			ctx.PRNumber = event.Number
			if ctx.PRNumber == 0 {
				ctx.PRNumber = event.PullRequest.Number
			}
		}
		if ctx.BaseSHA == "" {
			ctx.BaseSHA = event.PullRequest.Base.SHA
		}
		if ctx.HeadSHA == "" {
			ctx.HeadSHA = event.PullRequest.Head.SHA
		}
		ctx.BaseRepo = event.PullRequest.Base.Repo.FullName
		ctx.HeadRepo = event.PullRequest.Head.Repo.FullName
	}
	if ctx.PRNumber == 0 {
		ctx.PRNumber, _ = strconv.Atoi(strings.TrimSpace(os.Getenv("GITHUB_PR_NUMBER")))
	}
	if !validRepo(ctx.Repo) || ctx.PRNumber <= 0 || ctx.BaseSHA == "" || ctx.HeadSHA == "" {
		return Context{}, ErrInvalidContext
	}
	ctx.UnsafeFork = ctx.BaseRepo != "" && ctx.HeadRepo != "" && ctx.BaseRepo != ctx.HeadRepo
	return ctx, nil
}

func validRepo(repo string) bool {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return false
	}
	for _, part := range []string{owner, name} {
		for _, char := range part {
			if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-_.", char) {
				continue
			}
			return false
		}
	}
	return true
}
