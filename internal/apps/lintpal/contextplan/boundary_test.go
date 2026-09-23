package contextplan

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
	"github.com/diffpal/lintpal/internal/apps/lintpal/provider/systemone"
)

func TestPlanRequestMatchesTransportEncoding(t *testing.T) {
	groups, _ := testPlanInput(t)
	item := groups[0].Items[0]
	bindings := []Binding{
		{groups[0].ID, item.ID, "n", jev.NoulQuestion{Instructions: "N?", Criteria: &jev.NoulCriteria{True: "yes", False: "no"}}},
		{groups[0].ID, item.ID, "c", jev.ChoiceQuestion{Instructions: "C?", Criteria: map[string]string{"a": "A", "b": "B"}}},
		{groups[0].ID, item.ID, "s", jev.ScoreQuestion{Instructions: "S?", Criteria: []string{"low", "high"}}},
	}
	batches, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", Limits{})
	if err != nil || len(batches) != 1 {
		t.Fatalf("plan: %v, batches=%d", err, len(batches))
	}
	want, err := requestBody(batches[0].Request)
	if err != nil {
		t.Fatal(err)
	}
	var actual []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"model":"jev-1.13.0","answers":{"n":{"type":"noul","noul":0.8},"c":{"type":"choice","choice":"b","probabilities":{"a":0.2,"b":0.8},"confidence":0.8},"s":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.2,"1":0.8},"confidence":0.8}},"usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer server.Close()
	endpoint, err := systemone.TrustedCustom(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := systemone.New(endpoint, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Evaluate(t.Context(), batches[0].Request); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatalf("planner and transport JSON differ\nplanner=%s\ntransport=%s", want, actual)
	}
}

func TestPlanByteBoundaryAndIdenticalState(t *testing.T) {
	groups, bindings := testPlanInput(t)
	// An identical state may serve distinct work items while retaining both IDs.
	groups[1].State = groups[0].State
	combined, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", Limits{})
	if err != nil || len(combined) != 1 || len(combined[0].GroupIDs) != 2 {
		t.Fatalf("identical-state batch: %v, %+v", err, combined)
	}
	maxSingle := 0
	for _, binding := range bindings {
		body, err := requestBody(jev.Request{Model: "jev-1.13.0", State: groups[0].State,
			Questions: map[string]jev.Question{binding.QuestionID: binding.Question}})
		if err != nil {
			t.Fatal(err)
		}
		if len(body) > maxSingle {
			maxSingle = len(body)
		}
	}
	bounded, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", Limits{MaxRequestBytes: maxSingle})
	if err != nil || len(bounded) != len(bindings) {
		t.Fatalf("byte split: %v, batches=%d", err, len(bounded))
	}
	for _, batch := range bounded {
		body, err := requestBody(batch.Request)
		if err != nil || len(body) > maxSingle {
			t.Fatalf("oversized batch: %v, bytes=%d", err, len(body))
		}
	}
	if _, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", Limits{MaxRequestBytes: maxSingle - 1}); !errors.Is(err, ErrLimit) {
		t.Fatalf("single question should refuse: %v", err)
	}
}
