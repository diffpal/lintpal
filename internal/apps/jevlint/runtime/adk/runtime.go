// Package adk owns the optional, local ADK observability lifecycle.
package adk

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/adk/v2/telemetry"

	"github.com/diffpal/jevlint/internal/apps/jevlint/app"
)

// Runtime owns local telemetry providers. It does not set global providers or
// configure an exporter, so constructing it cannot send source or secret data.
type Runtime struct {
	providers *telemetry.Providers
	mu        sync.Mutex
	events    []Metric
}

// Metric has a closed schema and contains only aggregate count and duration.
type Metric struct {
	Stage    app.Stage
	Status   app.Status
	Count    int
	Duration time.Duration
}

// New creates providers without exporters or global registration.
func New() *Runtime {
	return newRuntime(sdktrace.NewTracerProvider(), sdklog.NewLoggerProvider())
}

func newRuntime(tracer *sdktrace.TracerProvider, logger *sdklog.LoggerProvider) *Runtime {
	return &Runtime{providers: &telemetry.Providers{
		TracerProvider: tracer,
		LoggerProvider: logger,
	}}
}

// Tracer provides local instrumentation to later adapters. Callers must only
// record safe metadata; core decisions and source text do not enter ADK.
func (r *Runtime) Tracer() trace.Tracer {
	return r.providers.TracerProvider.Tracer("jevlint")
}

// Observe records fixed local dimensions. Unknown values are discarded rather
// than becoming telemetry attributes.
func (r *Runtime) Observe(stage app.Stage, status app.Status, count int, duration time.Duration) {
	switch stage {
	case app.StageCompare, app.StageAssemble, app.StageSelect, app.StagePlan,
		app.StageEvaluate, app.StageReport, app.StageWrite:
	default:
		return
	}
	if status != app.StatusOK && status != app.StatusError || count < 0 || duration < 0 {
		return
	}
	r.mu.Lock()
	r.events = append(r.events, Metric{Stage: stage, Status: status, Count: count, Duration: duration})
	r.mu.Unlock()
	_, span := r.Tracer().Start(context.Background(), "jevlint."+string(stage),
		trace.WithAttributes(attribute.String("status", string(status)), attribute.Int("count", count)))
	span.End()
}

func (r *Runtime) Snapshot() []Metric {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Metric(nil), r.events...)
}

// Shutdown releases the ADK telemetry providers.
func (r *Runtime) Shutdown(ctx context.Context) error {
	return r.providers.Shutdown(ctx)
}
