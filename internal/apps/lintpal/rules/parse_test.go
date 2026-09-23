package rules

import (
	"errors"
	"strings"
	"testing"
)

const minimalPack = `schema: lintpal.rules.v1
rules:
  - id: demo.test
    type: noul
    instructions: Is this a problem?
    threshold: 0.9
    severity: high
    title: Problem
    message: This is a problem.
`

func TestParseStrictYAML(t *testing.T) {
	pack, err := parse(strings.NewReader(minimalPack))
	if err != nil || pack.Schema != schemaV1 || len(pack.Rules) != 1 || pack.Rules[0].ID != "demo.test" {
		t.Fatalf("valid pack: %+v, %v", pack, err)
	}
	for _, body := range []string{
		"schema: lintpal.rules.v1\ncommand: ./evil.sh\n",
		"schema: lintpal.rules.v1\nschema: lintpal.rules.v1\n",
		"schema: lintpal.rules.v1\nrules: &anchor []\n",
		"schema: lintpal.rules.v1\nrules: *anchor\n",
		"schema: lintpal.rules.v1\nrules: []\n---\nschema: lintpal.rules.v1\n",
		"schema: lintpal.rules.v1\nrules: !!binary Zm9v\n",
		"schema: lintpal.rules.v1\nrules: []\nextra: secret\n",
		"schema: lintpal.rules.v1\nrules: []\n<<: {extra: secret}\n",
	} {
		got, err := parse(strings.NewReader(body))
		if !errors.Is(err, ErrInvalidPack) || len(got.Rules) != 0 || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe YAML %q: %+v, %v", body, got, err)
		}
	}
	if _, err := parse(strings.NewReader("schema: future\nrules: []\n")); !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatal(err)
	}
	if _, err := parse(strings.NewReader(strings.Repeat("x", maxFileBytes+1))); !errors.Is(err, ErrPackLimit) {
		t.Fatal(err)
	}
}
