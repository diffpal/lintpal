package githubfeedback

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveContextExplicitPrecedenceAndForkSafety(t *testing.T) {
	event := `{"number":7,"repository":{"full_name":"owner/repo"},"pull_request":{"base":{"sha":"event-base","repo":{"full_name":"owner/repo"}},"head":{"sha":"event-head","repo":{"full_name":"fork/repo"}}}}`
	path := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(path, []byte(event), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, err := ResolveContext(ContextOptions{Repo: "explicit/repo", PRNumber: 9, BaseSHA: "explicit-base", HeadSHA: "explicit-head", EventPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Repo != "explicit/repo" || ctx.PRNumber != 9 || ctx.BaseSHA != "explicit-base" || ctx.HeadSHA != "explicit-head" || !ctx.UnsafeFork {
		t.Fatalf("unexpected context: %+v", ctx)
	}
}

func TestResolveContextRejectsMissingData(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_EVENT_PATH", "")
	t.Setenv("GITHUB_PR_NUMBER", "")
	if _, err := ResolveContext(ContextOptions{}); err != ErrInvalidContext {
		t.Fatalf("unexpected error: %v", err)
	}
}
