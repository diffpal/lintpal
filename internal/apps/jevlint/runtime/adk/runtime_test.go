package adk

import (
	"context"
	"testing"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type recordingExporter struct {
	exported bool
	closed   bool
}

func (e *recordingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	e.exported = true
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error {
	e.closed = true
	return nil
}

func TestLocalTelemetryLifecycle(t *testing.T) {
	exporter := new(recordingExporter)
	runtime := newRuntime(
		sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)),
		sdklog.NewLoggerProvider(),
	)
	ctx, span := runtime.Tracer().Start(t.Context(), "safe operation")
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		t.Fatal("missing valid trace context")
	}
	span.End()
	if !exporter.exported {
		t.Fatal("local span was not recorded")
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !exporter.closed {
		t.Fatal("ADK provider shutdown did not close the local exporter")
	}
}
