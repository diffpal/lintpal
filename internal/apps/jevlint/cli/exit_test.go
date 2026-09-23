package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/provider/systemone"
	"github.com/diffpal/jevlint/internal/apps/jevlint/report"
)

func TestExitCodeCategories(t *testing.T) {
	for _, test := range []struct {
		err  error
		code int
	}{
		{nil, 0}, {ErrInvalidOptions, 2}, {git.ErrInvalidRevision, 2},
		{systemone.ErrMissingCredential, 2}, {systemone.HTTPError{Status: 401}, 2},
		{systemone.ErrTransport, 3}, {systemone.HTTPError{Status: 429}, 3},
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
