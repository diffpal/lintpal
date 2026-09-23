package cli

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/app"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
)

var ErrInvalidOptions = errors.New("invalid lint options")

// RawOptions contains only process-owned flag values. Changed distinguishes an
// explicit empty flag from an absent flag when environment values are present.
type RawOptions struct {
	Base, Head, Provider, Model, Rules, Format, Out, FailOn string
	Timeout, MaxConcurrency, BaseURL, AuthTokenEnv          string
	Metrics                                                 bool
	Changed                                                 map[string]bool
}

type Options struct {
	Base, Head, Provider, Model, Rules, Out string
	BaseURL, AuthTokenEnv                   string
	Format                                  report.Format
	FailOn                                  report.Threshold
	Limits                                  app.Limits
	Metrics                                 bool
}

type LookupEnv func(string) (string, bool)

var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/:@-]{0,127}$`)

// Resolve uses flag > LINTPAL_* environment > default precedence. Rule YAML
// never enters this function, so it cannot choose an endpoint or token source.
func Resolve(raw RawOptions, lookup LookupEnv) (Options, error) {
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}
	choose := func(flag, value, env, fallback string) string {
		if raw.Changed[flag] {
			return value
		}
		if candidate, ok := lookup(env); ok {
			return candidate
		}
		return fallback
	}
	o := Options{
		Base:         choose("base", raw.Base, "LINTPAL_BASE", ""),
		Head:         choose("head", raw.Head, "LINTPAL_HEAD", ""),
		Provider:     choose("provider", raw.Provider, "LINTPAL_PROVIDER", "jev"),
		Model:        choose("model", raw.Model, "LINTPAL_MODEL", "jev-latest"),
		Rules:        choose("rules", raw.Rules, "LINTPAL_RULES", ""),
		Out:          choose("out", raw.Out, "LINTPAL_OUT", ""),
		BaseURL:      choose("base-url", raw.BaseURL, "LINTPAL_BASE_URL", ""),
		AuthTokenEnv: choose("auth-token-env", raw.AuthTokenEnv, "LINTPAL_AUTH_TOKEN_ENV", ""),
		Format:       report.Format(choose("format", raw.Format, "LINTPAL_FORMAT", "human")),
		FailOn:       report.Threshold(choose("fail-on", raw.FailOn, "LINTPAL_FAIL_ON", "high")),
		Metrics:      raw.Metrics,
	}
	if o.Base == "" || o.Head == "" || !modelName.MatchString(o.Model) || len(o.Base) > 256 || len(o.Head) > 256 ||
		len(o.Rules) > 4096 || len(o.Out) > 4096 || len(o.BaseURL) > 2048 || len(o.AuthTokenEnv) > 128 {
		return Options{}, ErrInvalidOptions
	}
	switch o.Provider {
	case "jev", "openrouter", "custom":
	default:
		return Options{}, ErrInvalidOptions
	}
	switch o.Format {
	case report.JSON, report.Human:
	default:
		return Options{}, ErrInvalidOptions
	}
	switch o.FailOn {
	case report.None, report.Low, report.Medium, report.High, report.Critical:
	default:
		return Options{}, ErrInvalidOptions
	}
	if o.Provider == "custom" {
		if o.BaseURL == "" {
			return Options{}, ErrInvalidOptions
		}
		if o.AuthTokenEnv == "" {
			o.AuthTokenEnv = "LINTPAL_TOKEN"
		}
	} else if o.BaseURL != "" || o.AuthTokenEnv != "" {
		return Options{}, ErrInvalidOptions
	}
	if o.Out == "-" || strings.ContainsRune(o.Out, 0) || o.Rules == "-" || strings.ContainsRune(o.Rules, 0) {
		return Options{}, ErrInvalidOptions
	}
	if o.Out != "" {
		o.Out = filepath.Clean(o.Out)
		if existing, err := os.Stat(o.Out); err == nil && existing.IsDir() {
			return Options{}, ErrInvalidOptions
		}
	}
	concurrency, err := strconv.Atoi(choose("max-concurrency", raw.MaxConcurrency, "LINTPAL_MAX_CONCURRENCY", "4"))
	if err != nil || concurrency < 1 || concurrency > 16 {
		return Options{}, ErrInvalidOptions
	}
	timeout, err := time.ParseDuration(choose("timeout", raw.Timeout, "LINTPAL_TIMEOUT", "2m"))
	if err != nil || timeout <= 0 || timeout > 10*time.Minute {
		return Options{}, ErrInvalidOptions
	}
	o.Limits = app.Limits{Concurrency: concurrency, Timeout: timeout}
	return o, nil
}
