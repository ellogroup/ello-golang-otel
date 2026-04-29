package provider

import (
	"context"
	"fmt"

	"github.com/ellogroup/ello-golang-otel/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// NewTracerProvider creates and registers a global TracerProvider configured from cfg.
//
// Returns:
//   - A trace.Tracer scoped to cfg.ServiceName, ready for creating spans.
//   - A shutdown function that flushes and stops the exporter; call it on Lambda shutdown.
//
// When cfg.Enabled is false a no-op tracer is returned with zero overhead.
// The W3C TraceContext and Baggage propagators are always registered globally so that
// context extraction in middleware works even in the disabled case.
func NewTracerProvider(ctx context.Context, cfg config.Config) (trace.Tracer, func(context.Context) error, error) {
	// Always register propagators so middleware can extract incoming trace context.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		return noop.NewTracerProvider().Tracer(cfg.ServiceName), func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTEL resource: %w", err)
	}

	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sampler)),
	)

	otel.SetTracerProvider(tp)

	return tp.Tracer(cfg.ServiceName), tp.Shutdown, nil
}
