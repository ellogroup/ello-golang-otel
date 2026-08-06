package provider

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/ellogroup/ello-golang-otel/config"
)

// Option configures the TracerProvider.
type Option func(*tracerOptions)

type tracerOptions struct {
	lambda bool
	xray   bool
}

// WithLambda configures synchronous span export via SimpleSpanProcessor so spans are
// delivered to the OTLP endpoint before the Lambda container is frozen between invocations.
func WithLambda() Option {
	return func(o *tracerOptions) { o.lambda = true }
}

// WithXRay configures X-Ray compatibility: uses the X-Ray ID generator (trace IDs contain
// a valid Unix timestamp prefix required by ADOT when converting OTLP to X-Ray segments)
// and registers the X-Ray propagator so X-Amzn-Trace-Id headers are extracted as parent context.
func WithXRay() Option {
	return func(o *tracerOptions) { o.xray = true }
}

// NewTracerProvider creates and registers a global TracerProvider configured from cfg.
// Use WithLambda() for Lambda execution environments and WithXRay() when sending to AWS X-Ray via ADOT.
//
// Returns a Tracer scoped to cfg.ServiceName and a shutdown function to flush on exit.
// When cfg.Enabled is false a no-op tracer is returned with zero overhead.
// W3C TraceContext and Baggage propagators are always registered globally; WithXRay also adds
// the X-Ray propagator.
func NewTracerProvider(ctx context.Context, cfg config.Config, opts ...Option) (trace.Tracer, func(context.Context) error, error) {
	o := &tracerOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return newTracerProvider(ctx, cfg, o)
}

// NewLambdaTracerProvider is a convenience wrapper for Lambda functions sending traces to AWS
// X-Ray via ADOT. It is equivalent to NewTracerProvider(ctx, cfg, WithLambda(), WithXRay()).
func NewLambdaTracerProvider(ctx context.Context, cfg config.Config) (trace.Tracer, func(context.Context) error, error) {
	return NewTracerProvider(ctx, cfg, WithLambda(), WithXRay())
}

func newTracerProvider(ctx context.Context, cfg config.Config, o *tracerOptions) (trace.Tracer, func(context.Context) error, error) {
	propagators := []propagation.TextMapPropagator{
		propagation.Baggage{},
		propagation.TraceContext{},
	}
	if o.xray {
		propagators = append(propagators, xray.Propagator{})
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagators...))

	if !cfg.Enabled {
		return noop.NewTracerProvider().Tracer(cfg.ServiceName), func(context.Context) error { return nil }, nil
	}

	endpoint, err := resolveSignalEndpoint(cfg.Endpoint, "v1/traces")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving OTLP trace endpoint: %w", err)
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
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
	if o.lambda {
		processor = sdktrace.NewSimpleSpanProcessor(exp)
	} else {
		processor = sdktrace.NewBatchSpanProcessor(exp)
	}

	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithSpanProcessor(processor),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sampler)),
	}
	if o.xray {
		tpOpts = append(tpOpts, sdktrace.WithIDGenerator(xray.NewIDGenerator()))
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)

	return tp.Tracer(cfg.ServiceName), tp.Shutdown, nil
}
