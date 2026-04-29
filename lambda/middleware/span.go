// Package middleware provides OpenTelemetry Lambda middleware that wraps each invocation
// in a root trace span and injects the trace ID and span ID into logctx so they appear
// automatically on every log line produced via logctx.Zap(ctx).
package middleware

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	awsmiddleware "github.com/ellogroup/ello-golang-aws/lambda/middleware"
	"github.com/ellogroup/ello-golang-ctx/logctx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"strconv"
	"strings"
)

// NewAPIGatewayV1 returns a WithResponse middleware that:
//  1. Extracts W3C traceparent / tracestate from incoming API Gateway headers.
//  2. Starts a root server span named "{METHOD} {path}".
//  3. Injects trace_id and span_id into logctx so every logctx.Zap(ctx) call
//     automatically includes them.
//  4. Sets the span status to Error on 5xx responses or returned errors.
//  5. Ends the span when the handler returns.
//
// Prepend this to the middleware slice before middleware.CommonAPIGatewayV1 so the
// span covers the full request lifecycle:
//
//	allMiddlewares := append(middleware.APIGatewayV1{otelmiddleware.NewAPIGatewayV1(tracer)}, commonMiddlewares...)
func NewAPIGatewayV1(tracer trace.Tracer) awsmiddleware.WithResponse[events.APIGatewayProxyRequest, events.APIGatewayProxyResponse] {
	return &apiGatewayV1SpanMiddleware{tracer: tracer}
}

type apiGatewayV1SpanMiddleware struct {
	tracer trace.Tracer
}

func (m *apiGatewayV1SpanMiddleware) Wrap(
	next func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error),
) func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return func(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		// Extract W3C trace context from incoming API Gateway headers.
		// Headers are normalised to lowercase because the W3C spec and HTTP/2 use lowercase.
		carrier := propagation.MapCarrier(lowercaseHeaders(event.Headers))
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

		// Inject trace_id and span_id into logctx so all subsequent log calls
		// via logctx.Zap(ctx) automatically carry them.
		ctx = injectSpanContext(ctx, span)

		resp, err := next(ctx, event)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if resp.StatusCode >= 500 {
			span.SetStatus(codes.Error, strconv.Itoa(resp.StatusCode))
		}

		span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))

		return resp, err
	}
}

// NewNoResponse returns a NoResponse middleware that wraps a custom-event Lambda handler
// in a root span. Use this for handlers that do not return an API Gateway response
// (e.g. SQS, SNS, scheduled events).
//
// spanName should describe the operation, e.g. "process-scheduled-task".
func NewNoResponse[E any](tracer trace.Tracer, spanName string) awsmiddleware.NoResponse[E] {
	return &noResponseSpanMiddleware[E]{tracer: tracer, spanName: spanName}
}

type noResponseSpanMiddleware[E any] struct {
	tracer   trace.Tracer
	spanName string
}

func (m *noResponseSpanMiddleware[E]) Wrap(
	next func(context.Context, E) error,
) func(context.Context, E) error {
	return func(ctx context.Context, event E) error {
		spanName := m.spanName
		if spanName == "" {
			spanName = lambdaFunctionName(ctx)
		}

		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindConsumer),
		)
		defer span.End()

		ctx = injectSpanContext(ctx, span)

		err := next(ctx, event)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return err
	}
}

// injectSpanContext adds trace_id and span_id from span into logctx.
// Only injects when the span context is valid (i.e. OTEL is enabled).
func injectSpanContext(ctx context.Context, span trace.Span) context.Context {
	sc := span.SpanContext()
	if !sc.IsValid() {
		return ctx
	}
	return logctx.Add(ctx,
		logctx.String("trace_id", sc.TraceID().String()),
		logctx.String("span_id", sc.SpanID().String()),
	)
}

// lowercaseHeaders returns a copy of headers with all keys lowercased.
// API Gateway may preserve the original casing from the client; W3C headers are case-insensitive.
func lowercaseHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[strings.ToLower(k)] = v
	}
	return out
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
