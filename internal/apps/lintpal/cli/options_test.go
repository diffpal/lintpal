package cli

import (
	"errors"
	"testing"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

func TestResolvePrecedenceAndTrust(t *testing.T) {
	env := map[string]string{"LINTPAL_BASE": "env-base", "LINTPAL_HEAD": "env-head", "LINTPAL_PROVIDER": "openrouter", "LINTPAL_FAIL_ON": "none", "LINTPAL_TIMEOUT": "3m"}
	lookup := func(name string) (string, bool) { value, ok := env[name]; return value, ok }
	o, err := Resolve(RawOptions{Base: "flag-base", Changed: map[string]bool{"base": true}}, lookup)
	if err != nil || o.Base != "flag-base" || o.Head != "env-head" || o.Provider != "openrouter" || o.FailOn != report.None || !o.Gate || o.Limits.Timeout != 3*time.Minute {
		t.Fatalf("precedence: %+v, %v", o, err)
	}
	o, err = Resolve(RawOptions{Base: "a", Head: "b", Changed: map[string]bool{"base": true, "head": true}}, nil)
	if err != nil || o.Provider != "jev" || o.Model != "jev-latest" || o.FailOn != report.High || !o.Gate || o.Limits.Concurrency != 4 {
		t.Fatalf("defaults: %+v, %v", o, err)
	}
	if o.Format != report.Markdown {
		t.Fatalf("wrong default format: %s", o.Format)
	}
	for _, raw := range []RawOptions{
		{Base: "", Head: "b", Changed: map[string]bool{"base": true, "head": true}},
		{Base: "a", Head: "b", Provider: "custom", Changed: map[string]bool{"base": true, "head": true, "provider": true}},
		{Base: "a", Head: "b", BaseURL: "https://host.test", Changed: map[string]bool{"base": true, "head": true, "base-url": true}},
		{Base: "a", Head: "b", AuthTokenEnv: "OTHER_TOKEN", Changed: map[string]bool{"base": true, "head": true, "auth-token-env": true}},
		{Base: "a", Head: "b", FailOn: "fatal", Changed: map[string]bool{"base": true, "head": true, "fail-on": true}},
		{Base: "a", Head: "b", Format: "human", Changed: map[string]bool{"base": true, "head": true, "format": true}},
		{Base: "a", Head: "b", MaxConcurrency: "17", Changed: map[string]bool{"base": true, "head": true, "max-concurrency": true}},
	} {
		if _, err := Resolve(raw, lookup); !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("accepted invalid %+v: %v", raw, err)
		}
	}
	o, err = Resolve(RawOptions{Base: "a", Head: "b", Provider: "custom", BaseURL: "http://127.0.0.1:1234", Changed: map[string]bool{"base": true, "head": true, "provider": true, "base-url": true}}, nil)
	if err != nil || o.AuthTokenEnv != "LINTPAL_TOKEN" {
		t.Fatalf("custom default: %+v, %v", o, err)
	}
	if _, err := Resolve(RawOptions{Base: "a", Head: "b", Out: t.TempDir(), Changed: map[string]bool{"base": true, "head": true, "out": true}}, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("directory output accepted: %v", err)
	}
}

func TestResolveBlockOn(t *testing.T) {
	base := RawOptions{Base: "a", Head: "b", BlockOn: "medium", Changed: map[string]bool{"base": true, "head": true, "block-on": true}}
	options, err := Resolve(base, func(name string) (string, bool) {
		if name == "LINTPAL_FAIL_ON" {
			return "critical", true
		}
		return "", false
	})
	if err != nil || options.FailOn != report.Medium || options.Gate {
		t.Fatalf("block-on did not select deferred mode: %+v, %v", options, err)
	}
	base.FailOn = "high"
	base.Changed["fail-on"] = true
	if _, err := Resolve(base, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("accepted conflicting flags: %v", err)
	}
	delete(base.Changed, "fail-on")
	base.BlockOn = "bad"
	if _, err := Resolve(base, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("accepted invalid block-on: %v", err)
	}
}

func TestResolveCredentialFromLayeredLookup(t *testing.T) {
	raw := RawOptions{Base: "a", Head: "b", Changed: map[string]bool{"base": true, "head": true}}
	lookup := LayeredLookup(func(name string) (string, bool) {
		if name == "TYPESAFE_API_KEY" {
			return "process-secret", true
		}
		return "", false
	}, map[string]string{"TYPESAFE_API_KEY": "file-secret"})
	options, err := Resolve(raw, lookup)
	if err != nil || !options.CredentialResolved || options.Credential != "process-secret" {
		t.Fatalf("process credential did not win: resolved = %v, err = %v", options.CredentialResolved, err)
	}
	options, err = Resolve(raw, LayeredLookup(nil, map[string]string{"TYPESAFE_API_KEY": "file-secret"}))
	if err != nil || options.Credential != "file-secret" {
		t.Fatalf("file credential unavailable: resolved = %v, err = %v", options.CredentialResolved, err)
	}
}

func TestResolveMarkdownRulePolicy(t *testing.T) {
	base := RawOptions{Base: "a", Head: "b", Changed: map[string]bool{"base": true, "head": true}}
	defaultOptions, err := Resolve(base, nil)
	if err != nil || defaultOptions.RuleThreshold != 0.95 || defaultOptions.RuleSeverity != rules.Medium ||
		defaultOptions.RuleThresholdSet || defaultOptions.RuleSeveritySet {
		t.Fatalf("defaults: %+v, %v", defaultOptions, err)
	}
	base.RuleThreshold, base.RuleSeverity = "0.8", "critical"
	base.Include, base.Exclude = []string{"*.go"}, []string{"vendor/*"}
	base.Changed["rule-threshold"], base.Changed["rule-severity"] = true, true
	selected, err := Resolve(base, nil)
	if err != nil || selected.RuleThreshold != 0.8 || selected.RuleSeverity != rules.Critical ||
		!selected.RuleThresholdSet || !selected.RuleSeveritySet ||
		selected.FailOn != report.High || len(selected.Include) != 1 || len(selected.Exclude) != 1 {
		t.Fatalf("overrides: %+v, %v", selected, err)
	}
	for _, value := range []string{"", "NaN", "Inf", "-0.1", "1.1"} {
		base.RuleThreshold = value
		if _, err := Resolve(base, nil); !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("accepted threshold %q: %v", value, err)
		}
	}
	base.RuleThreshold, base.RuleSeverity = "0.8", "fatal"
	if _, err := Resolve(base, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("accepted severity: %v", err)
	}
	base.RuleSeverity, base.Include = "medium", []string{"[bad"}
	if _, err := Resolve(base, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("accepted selector: %v", err)
	}
	envOptions, err := Resolve(RawOptions{Base: "a", Head: "b", Changed: map[string]bool{"base": true, "head": true}},
		func(key string) (string, bool) {
			switch key {
			case "LINTPAL_RULE_THRESHOLD":
				return "0.7", true
			case "LINTPAL_RULE_SEVERITY":
				return "high", true
			default:
				return "", false
			}
		})
	if err != nil || envOptions.RuleThreshold != .7 || envOptions.RuleSeverity != rules.High ||
		!envOptions.RuleThresholdSet || !envOptions.RuleSeveritySet {
		t.Fatalf("environment policy: %+v, %v", envOptions, err)
	}
}
