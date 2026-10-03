package schema_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestLintPalFindingsSchema(t *testing.T) {
	raw, err := os.ReadFile("findings-v5.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	const canonicalSHA256 = "c83b29d66ecffade32519158985b49ce93ac600172ef31fd52da6c8503e1422a"
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != canonicalSHA256 {
		t.Fatalf("schema digest = %s, want canonical %s", got, canonicalSHA256)
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	object, ok := document.(map[string]any)
	if !ok || object["$id"] != "https://github.com/diffpal/lintpal/blob/main/docs/schema/findings-v5.schema.json" {
		t.Fatalf("schema is not owned by LintPal: %v", object["$id"])
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("findings-v5.schema.json", document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("findings-v5.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"lintpal-right.json", true},
		{"diffpal-left.json", true},
		{"invalid-rule-missing-decision.json", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			err = compiled.Validate(value)
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %t, want %t: %v", err == nil, tc.valid, err)
			}
		})
	}

	for _, tc := range []struct {
		name   string
		review string
		valid  bool
	}{
		{"complete", `{"request_count":2,"review_duration_ms":12,"cost_usd":0.125}`, true},
		{"unknown-cost", `{"request_count":1,"review_duration_ms":0}`, true},
		{"zero-requests", `{"request_count":0,"review_duration_ms":0,"cost_usd":0}`, true},
		{"negative-requests", `{"request_count":-1,"review_duration_ms":1}`, false},
		{"negative-duration", `{"request_count":1,"review_duration_ms":-1}`, false},
		{"negative-cost", `{"request_count":1,"review_duration_ms":1,"cost_usd":-0.1}`, false},
		{"zero-requests-unknown-cost", `{"request_count":0,"review_duration_ms":0}`, false},
		{"zero-requests-positive-cost", `{"request_count":0,"review_duration_ms":0,"cost_usd":0.1}`, false},
		{"unknown-field", `{"request_count":1,"review_duration_ms":1,"endpoint":"secret"}`, false},
	} {
		t.Run("review-"+tc.name, func(t *testing.T) {
			var value any
			raw := []byte(`{"version":"v5","review_id":"review","base_sha":"base","head_sha":"head","findings":[],"stats":{"review":` + tc.review + `}}`)
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			err := compiled.Validate(value)
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %t, want %t: %v", err == nil, tc.valid, err)
			}
		})
	}
}
