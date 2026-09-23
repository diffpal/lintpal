package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrozenCorpusBaseline(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	corpus, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	summary, reports, err := RunOffline(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Cases) != 8 || len(reports) != 8 || summary.TruePositive != 3 ||
		summary.TrueNegative != 3 || summary.FalsePositive != 1 || summary.FalseNegative != 1 {
		t.Fatalf("offline counts: %+v", summary)
	}
	for i, c := range summary.Cases {
		if c.Diagnostics != len(reports[i].Diagnostics) {
			t.Fatalf("case %s diagnostic count differs from report", c.ID)
		}
	}
	assertBaseline(t, "summary.json", summary)
	assertBaseline(t, "reports.json", reports)
	second, again, err := RunOffline(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := json.Marshal([]any{summary, reports})
	secondBytes, _ := json.Marshal([]any{second, again})
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("offline evaluation is nondeterministic")
	}
}

func TestCorpusRejectsMalformedAndOversized(t *testing.T) {
	for _, input := range []string{"", `{"schema_version":"lintpal.eval.corpus.v1","cases":[]}`,
		`{"schema_version":"lintpal.eval.corpus.v1","cases":[{"id":"x"}]}`,
		`{"schema_version":"lintpal.eval.corpus.v1","cases":[],"unexpected":true}`,
		strings.Repeat("x", MaxCorpusBytes+1)} {
		if corpus, err := Load(strings.NewReader(input)); !errors.Is(err, ErrInvalidCorpus) || len(corpus.Cases) != 0 {
			t.Fatalf("accepted invalid corpus of %d bytes: %v", len(input), err)
		}
	}
}

func assertBaseline(t *testing.T, name string, value any) {
	t.Helper()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_EVAL_BASELINE") == "1" {
		if err := os.WriteFile(path, payload, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, want) {
		t.Fatalf("%s changed; review case decisions before updating baseline", name)
	}
}
