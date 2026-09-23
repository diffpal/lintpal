package adk

import (
	"strings"
	"testing"
	"time"

	"github.com/diffpal/jevlint/internal/apps/jevlint/app"
)

func TestLocalMetricSchema(t *testing.T) {
	runtime := New()
	runtime.Observe(app.StageCompare, app.StatusOK, 3, 5*time.Millisecond)
	runtime.Observe(app.Stage("source-secret"), app.Status("token-secret"), 1, time.Second)
	events := runtime.Snapshot()
	if len(events) != 1 || events[0].Stage != app.StageCompare || events[0].Status != app.StatusOK ||
		events[0].Count != 3 || events[0].Duration != 5*time.Millisecond {
		t.Fatalf("local metric snapshot: %+v", events)
	}
	if strings.Contains(string(events[0].Stage), "secret") {
		t.Fatal("untrusted stage retained")
	}
}
