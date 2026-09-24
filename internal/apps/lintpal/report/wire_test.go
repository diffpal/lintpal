package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestWireConformsToSharedSchemaOnBothSides(t *testing.T) {
	items := []git.WorkItem{
		{ID: strings.Repeat("1", 64), Path: "old.go", OldPath: "old.go", Side: git.Left, StartLine: 3, EndLine: 3, Hunk: 1},
		{ID: strings.Repeat("2", 64), Path: "new.go", NewPath: "new.go", Side: git.Right, StartLine: 4, EndLine: 4, Hunk: 1},
	}
	result := git.Result{Revisions: git.Revisions{Base: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), MergeBase: strings.Repeat("a", 40)}, Items: items}
	decisions := []rules.Decision{
		{RuleID: "go/errors.md", WorkItemID: items[0].ID, Path: items[0].Path, Side: items[0].Side, StartLine: 3, EndLine: 3, Severity: rules.High, Title: "Check errors", Message: "Changed code may violate go/errors.md.", Kind: "noul_probability", Value: .97},
		{RuleID: "go/errors.md", WorkItemID: items[1].ID, Path: items[1].Path, Side: items[1].Side, StartLine: 4, EndLine: 4, Severity: rules.Medium, Title: "Check errors", Message: "Changed code may violate go/errors.md.", Kind: "noul_probability", Value: .96},
	}
	artifact, err := New(result, decisions, "systemone", "model", Stats{WorkItems: 2})
	if err != nil {
		t.Fatal(err)
	}
	artifact = WithGate(artifact, High)
	var output bytes.Buffer
	if err := WriteJSON(&output, artifact); err != nil {
		t.Fatal(err)
	}
	schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "schema", "findings-v5.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schemaDoc, wireDoc any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &wireDoc); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("v5.schema.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("v5.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(wireDoc); err != nil {
		t.Fatalf("report violates shared v5 schema: %v\n%s", err, output.String())
	}
	var bundle wireBundle
	if err := json.Unmarshal(output.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Findings) != 2 || bundle.Findings[0].Path != "new.go" || bundle.Findings[0].ChangedSpan.Side != git.Right ||
		bundle.Findings[1].Path != "old.go" || bundle.Findings[1].ChangedSpan.Side != git.Left ||
		bundle.Findings[0].Blocking || !bundle.Findings[1].Blocking || bundle.Findings[0].ID == bundle.Findings[1].ID {
		t.Fatalf("wrong wire findings: %+v", bundle.Findings)
	}
}
