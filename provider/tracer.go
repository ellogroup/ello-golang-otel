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
// Spans are batched and exported in the background — suitable for long-running services.
// For Lambda, use NewLambdaTracerProvider instead.
//
// Returns a Tracer scoped to cfg.ServiceName and a shutdown function to flush on exit.
// When cfg.Enabled is false a no-op tracer is returned with zero overhead.
// W3C TraceContext and Baggage propagators are always registered globally.
func NewTracerProvider(ctx context.Context, cfg config.Config) (trace.Tracer, func(context.Context) error, error) {
	return newTracerProvider(ctx, cfg, false)
}

// NewLambdaTracerProvider creates and registers a global TracerProvider configured from cfg.
// Spans are exported synchronously on span.End(), ensuring delivery to the OTLP endpoint
// before the Lambda container is frozen between invocations.
//
// Returns a Tracer scoped to cfg.ServiceName and a shutdown function to call on SIGTERM.
// When cfg.Enabled is false a no-op tracer is returned with zero overhead.
// W3C TraceContext and Baggage propagators are always registered globally.
func NewLambdaTracerProvider(ctx context.Context, cfg config.Config) (trace.Tracer, func(context.Context) error, error) {
	return newTracerProvider(ctx, cfg, true)
}

func newTracerProvider(ctx context.Context, cfg config.Config, lambda bool) (trace.Tracer, func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		return noop.NewTracerProvider().Tracer(cfg.ServiceName), func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
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

	var processor sdktrace.SpanProcessor
	if lambda {
		processor = sdktrace.NewSimpleSpanProcessor(exp)
	} else {
		processor = sdktrace.NewBatchSpanProcessor(exp)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(processor),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sampler)),
	)

	otel.SetTracerProvider(tp)

	return tp.Tracer(cfg.ServiceName), tp.Shutdown, nil
}
