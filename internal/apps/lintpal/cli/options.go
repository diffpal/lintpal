package cli

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/app"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

var ErrInvalidOptions = errors.New("invalid lint options")

// RawOptions contains only process-owned flag values. Changed distinguishes an
// explicit empty flag from an absent flag when environment values are present.
type RawOptions struct {
	Base, Head, Provider, Model, Rules, Format, Out, FailOn, BlockOn string
	Timeout, MaxConcurrency, BaseURL, AuthTokenEnv                   string
	RuleThreshold, RuleSeverity                                      string
	Include, Exclude                                                 []string
	Metrics                                                          bool
	Uncommitted                                                      bool
	Changed                                                          map[string]bool
}

type Options struct {
	Base, Head, Provider, Model, Rules, Out string
	BaseURL, AuthTokenEnv                   string
	Credential                              string
	CredentialResolved                      bool
	Format                                  report.Format
	FailOn                                  report.Threshold
	Gate                                    bool
	RuleThreshold                           float64
	RuleSeverity                            rules.Severity
	RuleThresholdSet, RuleSeveritySet       bool
	Include, Exclude                        []string
	Limits                                  app.Limits
	Metrics                                 bool
	Uncommitted                             bool
}

type LookupEnv func(string) (string, bool)

var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/:@-]{0,127}$`)

// Resolve uses flag > LINTPAL_* environment > default precedence. Rule content
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
	choosePolicy := func(flag, value, env, fallback string) (string, bool) {
		if raw.Changed[flag] {
			return value, true
		}
		if candidate, ok := lookup(env); ok {
			return candidate, true
		}
		return fallback, false
	}
	if raw.Changed["block-on"] && raw.Changed["fail-on"] {
		return Options{}, ErrInvalidOptions
	}
	if raw.Uncommitted && (raw.Changed["base"] || raw.Changed["head"] || raw.Base != "" || raw.Head != "") {
		return Options{}, ErrInvalidOptions
	}
	threshold := choose("fail-on", raw.FailOn, "LINTPAL_FAIL_ON", "high")
	gate := true
	if raw.Changed["block-on"] {
		threshold = raw.BlockOn
		gate = false
	}
	severity, severitySet := choosePolicy("rule-severity", raw.RuleSeverity, "LINTPAL_RULE_SEVERITY", "medium")
	thresholdText, thresholdSet := choosePolicy("rule-threshold", raw.RuleThreshold, "LINTPAL_RULE_THRESHOLD", "0.95")
	base := choose("base", raw.Base, "LINTPAL_BASE", "")
	head := choose("head", raw.Head, "LINTPAL_HEAD", "")
	if raw.Uncommitted {
		base = ""
		head = ""
	}
	o := Options{
		Base:             base,
		Head:             head,
		Uncommitted:      raw.Uncommitted,
		Provider:         choose("provider", raw.Provider, "LINTPAL_PROVIDER", "jev"),
		Model:            choose("model", raw.Model, "LINTPAL_MODEL", "jev-latest"),
		Rules:            choose("rules", raw.Rules, "LINTPAL_RULES", ""),
		Out:              choose("out", raw.Out, "LINTPAL_OUT", ""),
		BaseURL:          choose("base-url", raw.BaseURL, "LINTPAL_BASE_URL", ""),
		AuthTokenEnv:     choose("auth-token-env", raw.AuthTokenEnv, "LINTPAL_AUTH_TOKEN_ENV", ""),
		Format:           report.Format(choose("format", raw.Format, "LINTPAL_FORMAT", "markdown")),
		FailOn:           report.Threshold(threshold),
		Gate:             gate,
		Metrics:          raw.Metrics,
		RuleSeverity:     rules.Severity(severity),
		RuleSeveritySet:  severitySet,
		RuleThresholdSet: thresholdSet,
		Include:          append([]string(nil), raw.Include...),
		Exclude:          append([]string(nil), raw.Exclude...),
	}
	ruleThreshold, err := strconv.ParseFloat(thresholdText, 64)
	if err != nil || math.IsNaN(ruleThreshold) || math.IsInf(ruleThreshold, 0) || ruleThreshold < 0 || ruleThreshold > 1 {
		return Options{}, ErrInvalidOptions
	}
	o.RuleThreshold = ruleThreshold
	switch o.RuleSeverity {
	case rules.Low, rules.Medium, rules.High, rules.Critical:
	default:
		return Options{}, ErrInvalidOptions
	}
	if err := rules.ValidateSelectors(o.Include, o.Exclude); err != nil {
		return Options{}, ErrInvalidOptions
	}
	if !o.Uncommitted && (o.Base == "" || o.Head == "") {
		return Options{}, ErrInvalidOptions
	}
	if o.Uncommitted && (o.Base != "" || o.Head != "") {
		return Options{}, ErrInvalidOptions
	}
	if len(o.Base) > 256 || len(o.Head) > 256 || !modelName.MatchString(o.Model) ||
		len(o.Rules) > 4096 || len(o.Out) > 4096 || len(o.BaseURL) > 2048 || len(o.AuthTokenEnv) > 128 {
		return Options{}, ErrInvalidOptions
	}
	switch o.Provider {
	case "jev", "openrouter", "custom":
	default:
		return Options{}, ErrInvalidOptions
	}
	switch o.Format {
	case report.JSON, report.Markdown:
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
	switch o.Provider {
	case "jev":
		o.Credential, _ = lookup("TYPESAFE_API_KEY")
	case "openrouter":
		o.Credential, _ = lookup("OPENROUTER_API_KEY")
	case "custom":
		o.Credential, _ = lookup(o.AuthTokenEnv)
	}
	o.CredentialResolved = true
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
