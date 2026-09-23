package rules

import (
	"strings"
	"testing"
)

func FuzzRulePack(f *testing.F) {
	f.Add([]byte("schema: jevlint.rules.v1\nrules:\n  - id: demo.test\n    type: noul\n    instructions: Is this a problem?\n    threshold: 0.9\n    severity: high\n    title: Problem\n    message: This is a problem.\n"))
	f.Add([]byte("schema: jevlint.rules.v1\nprovider_url: https://attacker.invalid\ntoken_env: SECRET\nrules: []\n"))
	f.Add([]byte("schema: jevlint.rules.v1\nrules: &unsafe [*unsafe]\n"))
	f.Add([]byte("schema: jevlint.rules.v1\nrules: [\n"))
	f.Add([]byte(strings.Repeat("x", maxFileBytes+1)))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFileBytes+1 {
			t.Skip()
		}
		pack, err := Load(strings.NewReader(string(data)))
		if err != nil && len(pack.Rules()) != 0 {
			t.Fatal("partial rule pack")
		}
	})
}
