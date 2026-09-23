package cli

import (
	"errors"
	"testing"
	"time"

	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
)

func TestResolvePrecedenceAndTrust(t *testing.T) {
	env := map[string]string{"LINTPAL_BASE": "env-base", "LINTPAL_HEAD": "env-head", "LINTPAL_PROVIDER": "openrouter", "LINTPAL_FAIL_ON": "none", "LINTPAL_TIMEOUT": "3m"}
	lookup := func(name string) (string, bool) { value, ok := env[name]; return value, ok }
	o, err := Resolve(RawOptions{Base: "flag-base", Changed: map[string]bool{"base": true}}, lookup)
	if err != nil || o.Base != "flag-base" || o.Head != "env-head" || o.Provider != "openrouter" || o.FailOn != report.None || o.Limits.Timeout != 3*time.Minute {
		t.Fatalf("precedence: %+v, %v", o, err)
	}
	o, err = Resolve(RawOptions{Base: "a", Head: "b", Changed: map[string]bool{"base": true, "head": true}}, nil)
	if err != nil || o.Provider != "jev" || o.Model != "jev-latest" || o.FailOn != report.High || o.Limits.Concurrency != 4 {
		t.Fatalf("defaults: %+v, %v", o, err)
	}
	for _, raw := range []RawOptions{
		{Base: "", Head: "b", Changed: map[string]bool{"base": true, "head": true}},
		{Base: "a", Head: "b", Provider: "custom", Changed: map[string]bool{"base": true, "head": true, "provider": true}},
		{Base: "a", Head: "b", BaseURL: "https://host.test", Changed: map[string]bool{"base": true, "head": true, "base-url": true}},
		{Base: "a", Head: "b", AuthTokenEnv: "OTHER_TOKEN", Changed: map[string]bool{"base": true, "head": true, "auth-token-env": true}},
		{Base: "a", Head: "b", FailOn: "fatal", Changed: map[string]bool{"base": true, "head": true, "fail-on": true}},
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
