// Package middleware provides OpenTelemetry Lambda middleware that wraps each invocation
// in a root trace span and records invocation metrics.
package middleware

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	awsmiddleware "github.com/ellogroup/ello-golang-aws/lambda/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// warmedUp tracks whether this execution environment has handled at least one invocation.
// The first invocation on any Lambda instance is a cold start; subsequent ones are not.
var warmedUp atomic.Bool

func isColdStart() bool {
	return !warmedUp.Swap(true)
}

// NewAPIGatewayV1 returns a WithResponse middleware that:
//  1. Extracts W3C traceparent / tracestate from incoming API Gateway headers.
//  2. Starts a root server span named "{METHOD} {path}".
//  3. Sets the span status to Error on 5xx responses or returned errors.
//  4. Ends the span when the handler returns.
//  5. Records faas.invocations and faas.duration metrics with faas.coldstart and
//     http.response.status_code attributes.
//
// Use provider.NewLambdaTracerProvider to ensure spans are exported synchronously
// before the Lambda container is frozen. Prepend this middleware so the span covers
// the full request lifecycle:
//
//	allMiddlewares := append(middleware.APIGatewayV1{otelmiddleware.NewAPIGatewayV1(tracer, meter)}, commonMiddlewares...)
func NewAPIGatewayV1(tracer trace.Tracer, meter metric.Meter) awsmiddleware.WithResponse[events.APIGatewayProxyRequest, events.APIGatewayProxyResponse] {
	invocations, _ := meter.Int64Counter("faas.invocations",
		metric.WithDescription("Number of Lambda handler invocations"),
	)
	duration, _ := meter.Float64Histogram("faas.duration",
		metric.WithDescription("Duration of Lambda handler invocations in milliseconds"),
		metric.WithUnit("ms"),
	)
	var metricFlush func(context.Context) error
	if fp, ok := otel.GetMeterProvider().(interface{ ForceFlush(context.Context) error }); ok {
		metricFlush = fp.ForceFlush
	}
	return &apiGatewayV1SpanMiddleware{
		tracer:      tracer,
		invocations: invocations,
		duration:    duration,
		metricFlush: metricFlush,
	}
}

type apiGatewayV1SpanMiddleware struct {
	tracer      trace.Tracer
	invocations metric.Int64Counter
	duration    metric.Float64Histogram
	metricFlush func(context.Context) error
}

func (m *apiGatewayV1SpanMiddleware) Wrap(
	next func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error),
) func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return func(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		start := time.Now()
		isColdStart := isColdStart()

		// Extract trace context from incoming API Gateway headers.
		// HeaderCarrier uses http.Header's case-insensitive lookup, which is required for
		// the X-Ray propagator (X-Amzn-Trace-Id) alongside W3C traceparent/tracestate.
		carrier := propagation.HeaderCarrier(headersToHTTP(event.Headers))
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

		spanName := event.RequestContext.HTTPMethod + " " + event.RequestContext.Path

		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(event.RequestContext.HTTPMethod),
				semconv.URLPath(event.RequestContext.Path),
				semconv.ServerAddress(event.RequestContext.DomainName),
			),
		)
		defer span.End()

		resp, err := next(ctx, event)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if resp.StatusCode >= 500 {
			span.SetStatus(codes.Error, strconv.Itoa(resp.StatusCode))
		}

		span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))

		attrs := metric.WithAttributes(
			attribute.Bool("faas.coldstart", isColdStart),
			semconv.HTTPResponseStatusCode(resp.StatusCode),
		)
		m.invocations.Add(ctx, 1, attrs)
		m.duration.Record(ctx, float64(time.Since(start).Milliseconds()), attrs)

		if m.metricFlush != nil {
			_ = m.metricFlush(ctx)
		}

		return resp, err
	}
}

// NewNoResponse returns a NoResponse middleware that wraps a custom-event Lambda handler
// in a root span and records invocation metrics. Use this for handlers that do not return
// an API Gateway response (e.g. SQS, SNS, scheduled events).
//
// spanName should describe the operation, e.g. "process-scheduled-task".
func NewNoResponse[E any](tracer trace.Tracer, meter metric.Meter, spanName string) awsmiddleware.NoResponse[E] {
	invocations, _ := meter.Int64Counter("faas.invocations",
		metric.WithDescription("Number of Lambda handler invocations"),
	)
	duration, _ := meter.Float64Histogram("faas.duration",
		metric.WithDescription("Duration of Lambda handler invocations in milliseconds"),
		metric.WithUnit("ms"),
	)
	var metricFlush func(context.Context) error
	if fp, ok := otel.GetMeterProvider().(interface{ ForceFlush(context.Context) error }); ok {
		metricFlush = fp.ForceFlush
	}
	return &noResponseSpanMiddleware[E]{
		tracer:      tracer,
		spanName:    spanName,
		invocations: invocations,
		duration:    duration,
		metricFlush: metricFlush,
	}
}

type noResponseSpanMiddleware[E any] struct {
	tracer      trace.Tracer
	spanName    string
	invocations metric.Int64Counter
	duration    metric.Float64Histogram
	metricFlush func(context.Context) error
}

func (m *noResponseSpanMiddleware[E]) Wrap(
	next func(context.Context, E) error,
) func(context.Context, E) error {
	return func(ctx context.Context, event E) error {
		start := time.Now()
		isColdStart := isColdStart()

		spanName := m.spanName
		if spanName == "" {
			spanName = lambdaFunctionName(ctx)
		}

		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindConsumer),
		)
		defer span.End()

		err := next(ctx, event)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		attrs := metric.WithAttributes(
			attribute.Bool("faas.coldstart", isColdStart),
			attribute.Bool("faas.error", err != nil),
		)
		m.invocations.Add(ctx, 1, attrs)
		m.duration.Record(ctx, float64(time.Since(start).Milliseconds()), attrs)

		if m.metricFlush != nil {
			_ = m.metricFlush(ctx)
		}

		return err
	}
}

// headersToHTTP converts an API Gateway header map to http.Header, normalising keys to
// canonical MIME form (e.g. "x-amzn-trace-id" → "X-Amzn-Trace-Id") so that
// propagation.HeaderCarrier lookups are case-insensitive regardless of what casing
// API Gateway or the client used.
func headersToHTTP(headers map[string]string) http.Header {
	h := make(http.Header, len(headers))
	for k, v := range headers {
		h.Set(k, v)
	}
	return h
}

// lambdaFunctionName returns the Lambda function name from context, or "lambda" as fallback.
func lambdaFunctionName(ctx context.Context) string {
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		// ARN format: arn:aws:lambda:<region>:<account>:function:<name>
		parts := strings.Split(lc.InvokedFunctionArn, ":")
		if len(parts) >= 7 {
			return parts[6]
		}
	}
	return "lambda"
}
