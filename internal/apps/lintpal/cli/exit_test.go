package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/provider/decisions"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
)

func TestExitCodeCategories(t *testing.T) {
	for _, test := range []struct {
		err  error
		code int
	}{
		{nil, 0}, {ErrInvalidOptions, 2}, {git.ErrInvalidRevision, 2},
		{decisions.ErrMissingCredential, 2}, {decisions.HTTPError{Status: 401}, 2},
		{decisions.ErrTransport, 3}, {decisions.HTTPError{Status: 429}, 3},
		{context.DeadlineExceeded, 3}, {report.ErrExport, 4}, {errors.New("unknown"), 5},
		{report.ErrGate, 10}, {context.Canceled, 130},
		{errors.Join(context.Canceled, report.ErrExport), 130},
		{errors.Join(report.ErrExport, report.ErrGate), 4},
	} {
		if got := ExitCode(test.err); got != test.code {
			t.Fatalf("ExitCode(%v)=%d, want %d", test.err, got, test.code)
		}
	}
}

func TestInvalidCommandInputExitCode(t *testing.T) {
	t.Setenv("GITHUB_EVENT_PATH", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	input := "../../../../docs/schema/testdata/diffpal-left.json"
	for _, args := range [][]string{
		{"rule", "view"}, {"rule", "view", "one", "two"},
		{"rule", "import"}, {"rule", "list", "extra"}, {"rule", "validate", "extra"},
		{"feedback", "extra"},
		{"feedback", "github", "--in", input, "--dry-run"},
	} {
		root := NewRoot(nil, "test")
		root.SetArgs(args)
		if err := root.ExecuteContext(t.Context()); ExitCode(err) != 2 {
			t.Fatalf("%v: error %v, exit %d", args, err, ExitCode(err))
		}
	}
}
