package contextplan

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

func testPlanInput(t *testing.T) ([]Group, []Binding) {
	t.Helper()
	result, _ := testResult(t)
	groups, err := Assemble(t.Context(), result, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var bindings []Binding
	for _, group := range groups {
		for _, item := range group.Items {
			bindings = append(bindings, Binding{GroupID: group.ID, WorkItemID: item.ID,
				QuestionID: item.ID[:10], Question: jev.NoulQuestion{Instructions: "Is this safe?"}})
		}
	}
	return groups, bindings
}

func TestPlanSharedStateAndSplit(t *testing.T) {
	groups, bindings := testPlanInput(t)
	got, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", Limits{})
	if err != nil || len(got) != len(groups) {
		t.Fatalf("plan: %v, batches=%d", err, len(got))
	}
	seen := map[string]bool{}
	for _, batch := range got {
		if len(batch.GroupIDs) != 1 || len(batch.Request.Questions) != len(groups[0].Items) {
			t.Fatalf("unexpected batch: %+v", batch)
		}
		for id := range batch.Request.Questions {
			if seen[id] || batch.ItemByQuestion[id] == "" {
				t.Fatalf("duplicate/unmapped %s", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != len(bindings) {
		t.Fatalf("questions seen=%d want=%d", len(seen), len(bindings))
	}
	split, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", Limits{MaxQuestionsPerBatch: 1})
	if err != nil || len(split) != len(bindings) {
		t.Fatalf("split: %v, batches=%d", err, len(split))
	}
	for _, batch := range split {
		if len(batch.Request.Questions) != 1 {
			t.Fatal("unsplit questions")
		}
	}
	reversed := append([]Binding(nil), bindings...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	again, err := Plan(t.Context(), groups, reversed, "jev-1.13.0", Limits{})
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("nondeterministic plan: %v", err)
	}
}

func TestPlanAtomicRefusalAndRedaction(t *testing.T) {
	groups, bindings := testPlanInput(t)
	bindings[0].Question = jev.NoulQuestion{Instructions: strings.Repeat("SECRET_QUESTION", 2000)}
	for _, limits := range []Limits{{}, {MaxRequestBytes: 100}, {ByteBudget: 100},
		{MaxTotalStateBytes: 100}, {MaxItems: 1}, {MaxPlanBytes: 100}, {MaxQuestions: 1}} {
		got, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", limits)
		if !errors.Is(err, ErrLimit) || len(got) != 0 || strings.Contains(err.Error(), "SECRET_QUESTION") {
			t.Fatalf("limits %+v: %v, batches=%d", limits, err, len(got))
		}
	}
	bindings[0] = bindings[1]
	got, err := Plan(t.Context(), groups, bindings, "jev-1.13.0", Limits{})
	if !errors.Is(err, ErrInvalidInput) || len(got) != 0 {
		t.Fatalf("duplicate: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err = Plan(ctx, groups, bindings, "jev-1.13.0", Limits{})
	if !errors.Is(err, context.Canceled) || len(got) != 0 {
		t.Fatalf("canceled: %v", err)
	}
}
