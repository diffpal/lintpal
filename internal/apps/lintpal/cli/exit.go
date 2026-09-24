package cli

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/app"
	"github.com/diffpal/lintpal/internal/apps/lintpal/contextplan"
	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
	"github.com/diffpal/lintpal/internal/apps/lintpal/packs"
	"github.com/diffpal/lintpal/internal/apps/lintpal/provider/systemone"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

// ExitCode classifies errors without exposing their potentially untrusted text.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	if errors.Is(err, report.ErrExport) || errors.Is(err, app.ErrReportLimit) {
		return 4
	}
	if errors.Is(err, report.ErrGate) {
		return 10
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, systemone.ErrTransport) {
		return 3
	}
	if strings.HasPrefix(err.Error(), "unknown command") {
		return 2
	}
	var status systemone.HTTPError
	if errors.As(err, &status) {
		if status.Status == http.StatusTooManyRequests || status.Status == 529 || status.Status >= 500 && status.Status <= 599 {
			return 3
		}
		return 2
	}
	if errors.Is(err, ErrInvalidOptions) || errors.Is(err, ErrInvalidEnvFile) ||
		errors.Is(err, ErrEnvFileLimit) || errors.Is(err, packs.ErrSource) ||
		errors.Is(err, packs.ErrLock) || errors.Is(err, packs.ErrLegacyFormat) || errors.Is(err, packs.ErrConflict) ||
		errors.Is(err, packs.ErrDrift) || errors.Is(err, packs.ErrStorage) ||
		errors.Is(err, git.ErrInvalidRevision) ||
		errors.Is(err, report.ErrInvalidReport) || errors.Is(err, report.ErrInvalidFormat) || errors.Is(err, report.ErrInvalidThreshold) ||
		errors.Is(err, git.ErrAmbiguousBase) || errors.Is(err, git.ErrInvalidLimits) ||
		errors.Is(err, git.ErrLimit) || errors.Is(err, contextplan.ErrInvalidLimits) ||
		errors.Is(err, contextplan.ErrLimit) || errors.Is(err, systemone.ErrInvalidEndpoint) ||
		errors.Is(err, systemone.ErrMissingCredential) || errors.Is(err, jev.ErrInvalidRequest) ||
		errors.Is(err, rules.ErrInvalidPack) || errors.Is(err, rules.ErrInvalidRule) ||
		errors.Is(err, rules.ErrUnsupportedSchema) || errors.Is(err, rules.ErrPackLimit) ||
		errors.Is(err, os.ErrNotExist) {
		return 2
	}
	return 5
}

// ExitMessageFor renders a specific safe migration hint for legacy YAML packs.
func ExitMessageFor(err error) string {
	if errors.Is(err, packs.ErrLegacyFormat) {
		return packs.ErrLegacyFormat.Error()
	}
	return ExitMessage(ExitCode(err))
}

func ExitMessage(code int) string {
	switch code {
	case 2:
		return "invalid lint input or configuration"
	case 3:
		return "provider temporarily unavailable or timed out"
	case 4:
		return "report export failed"
	case 5:
		return "lint failed"
	case 10:
		return "lint severity gate failed"
	case 130:
		return "lint interrupted"
	default:
		return "lint failed"
	}
}
