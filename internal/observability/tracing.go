// Package observability wires OpenTelemetry tracing for RealtimeHub.
//
// Init sets up the global TracerProvider with an OTLP HTTP exporter and
// the W3C TraceContext propagator. Callers grab a named tracer via
// `otel.Tracer("name")`; before Init runs (or when no endpoint is
// configured) that returns a no-op tracer, so tests and dev runs that
// skip Jaeger pay zero cost.
package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Shutdown flushes pending spans. Call from main()'s deferred cleanup.
type Shutdown func(context.Context) error

// InitTracing installs a global TracerProvider exporting via OTLP HTTP.
// endpoint should be the OTLP base URL (e.g. http://localhost:4318). An
// empty endpoint disables export — Init still returns a Shutdown
// (no-op) so callers can `defer shutdown(ctx)` unconditionally.
func InitTracing(ctx context.Context, serviceName, endpoint string) (Shutdown, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint+"/v1/traces"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
