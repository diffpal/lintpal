package contextplan

import (
	"errors"
	"testing"
)

func TestLimits(t *testing.T) {
	defaults, err := (Limits{}).normalized()
	if err != nil || defaults.MaxStateBytes != defaultStateBytes || defaults.MaxRequestBytes >= 1<<20 || defaults.ByteBudget >= 32_000 {
		t.Fatalf("invalid defaults: %+v, %v", defaults, err)
	}
	for _, limits := range []Limits{{MaxStateBytes: -1}, {MaxTotalStateBytes: hardTotalStateBytes + 1}, {MaxRequestBytes: hardRequestBytes + 1},
		{ByteBudget: hardByteBudget + 1}, {MaxItems: hardMaxItems + 1},
		{MaxGroups: hardMaxGroups + 1}, {MaxQuestions: hardMaxQuestions + 1},
		{MaxQuestionsPerBatch: hardQuestionsPerBatch + 1}, {MaxPlanBytes: hardPlanBytes + 1}} {
		if _, err := limits.normalized(); !errors.Is(err, ErrInvalidLimits) {
			t.Fatalf("limits %+v: got %v", limits, err)
		}
	}
	custom, err := (Limits{MaxStateBytes: 100, MaxRequestBytes: 200, ByteBudget: 150}).normalized()
	if err != nil || custom.MaxStateBytes != 100 || custom.MaxRequestBytes != 200 || custom.ByteBudget != 150 {
		t.Fatalf("custom limits: %+v, %v", custom, err)
	}
}
