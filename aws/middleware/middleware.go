// Package middleware provides AWS SDK v2 OpenTelemetry instrumentation.
//
// It wraps every AWS SDK call in a client span, records service/operation/region/request-ID and
// HTTP status attributes, and injects trace context (via the global propagator) into outgoing
// request headers.
//
// This is a port of the core of go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws
// (v0.72.0, Apache-2.0), which is deprecated upstream as abandoned (no code owners). Differences from
// upstream: the per-service attribute builders (DynamoDB, S3, SQS, SNS) and configuration options were
// not ported; the global TracerProvider and TextMapPropagator are always used.
//
// TODO(agent/maintainer): periodically check whether upstream otelaws has been un-deprecated and, if
// so, replace this package's implementation with it again. See AGENTS.md "Vendored code — revisit
// periodically" and .agents/memory/techdebt.md TD-003.
package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	// ScopeName is the instrumentation scope name for spans created by this package.
	ScopeName = "github.com/ellogroup/ello-golang-otel/aws/middleware"

	systemKey     attribute.Key = "rpc.system.name"
	methodKey     attribute.Key = "rpc.method"
	regionKey     attribute.Key = "aws.region"
	requestIDKey  attribute.Key = "aws.request_id"
	statusCodeKey attribute.Key = "http.response.status_code"
	errorTypeKey  attribute.Key = "error.type"
	awsSystemVal                = "aws-api"
)

type startTimeKey struct{}

type otelMiddlewares struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// AppendToConfig attaches OTel instrumentation middleware to an existing aws.Config.
// It mutates cfg.APIOptions in-place, adding SDK middleware steps that:
//   - Start a client span for each AWS API call
//   - Record rpc.system.name, rpc.method, aws.region, aws.request_id and http.response.status_code attributes
//   - Inject trace context into outgoing HTTP request headers
//   - Set span error status on failed calls
//
// The global TracerProvider and TextMapPropagator are used (registered by provider.NewTracerProvider).
// When OTEL is disabled the global provider is a no-op so spans are zero-overhead.
//
// Do NOT also wrap the AWS HTTP transport with oteltransport.New — this middleware already
// injects trace context; double-wrapping creates duplicate spans.
func AppendToConfig(cfg *aws.Config) {
	m := otelMiddlewares{
		tracer:     otel.GetTracerProvider().Tracer(ScopeName),
		propagator: otel.GetTextMapPropagator(),
	}
	cfg.APIOptions = append(cfg.APIOptions, m.initializeBefore, m.initializeAfter, m.finalize, m.deserialize)
}

func (otelMiddlewares) initializeBefore(stack *middleware.Stack) error {
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("OTelInitializeMiddlewareBefore", func(
		ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
	) (middleware.InitializeOutput, middleware.Metadata, error) {
		ctx = context.WithValue(ctx, startTimeKey{}, time.Now())
		return next.HandleInitialize(ctx, in)
	}), middleware.Before)
}

func (m otelMiddlewares) initializeAfter(stack *middleware.Stack) error {
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("OTelInitializeMiddlewareAfter", func(
		ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
	) (middleware.InitializeOutput, middleware.Metadata, error) {
		serviceID := awsmiddleware.GetServiceID(ctx)
		operation := awsmiddleware.GetOperationName(ctx)

		startOpts := []trace.SpanStartOption{
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				systemKey.String(awsSystemVal),
				methodKey.String(methodName(serviceID, operation)),
				regionKey.String(awsmiddleware.GetRegion(ctx)),
			),
		}
		if t, ok := ctx.Value(startTimeKey{}).(time.Time); ok {
			startOpts = append(startOpts, trace.WithTimestamp(t))
		}

		ctx, span := m.tracer.Start(ctx, spanName(serviceID, operation), startOpts...)
		defer span.End()

		out, md, err := next.HandleInitialize(ctx, in)
		if err != nil {
			span.SetAttributes(errorTypeKey.String(errorType(err)))
			span.SetStatus(codes.Error, err.Error())
		}
		return out, md, err
	}), middleware.After)
}

func (m otelMiddlewares) finalize(stack *middleware.Stack) error {
	return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("OTelFinalizeMiddleware", func(
		ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
	) (middleware.FinalizeOutput, middleware.Metadata, error) {
		if req, ok := in.Request.(*smithyhttp.Request); ok {
			m.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
		}
		return next.HandleFinalize(ctx, in)
	}), middleware.After)
}

func (otelMiddlewares) deserialize(stack *middleware.Stack) error {
	return stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("OTelDeserializeMiddleware", func(
		ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler,
	) (middleware.DeserializeOutput, middleware.Metadata, error) {
		out, md, err := next.HandleDeserialize(ctx, in)
		resp, ok := out.RawResponse.(*smithyhttp.Response)
		if !ok {
			return out, md, err
		}

		span := trace.SpanFromContext(ctx)
		span.SetAttributes(statusCodeKey.Int(resp.StatusCode))
		if id, ok := awsmiddleware.GetRequestIDMetadata(md); ok {
			span.SetAttributes(requestIDKey.String(id))
		}
		return out, md, err
	}), middleware.Before)
}

func spanName(serviceID, operation string) string {
	if operation == "" {
		return serviceID
	}
	return serviceID + "." + operation
}

func methodName(serviceID, operation string) string {
	switch {
	case serviceID == "":
		return operation
	case operation == "":
		return serviceID
	default:
		return serviceID + "/" + operation
	}
}

func errorType(err error) string {
	type coder interface{ ErrorCode() string }
	var c coder
	if errors.As(err, &c) {
		return c.ErrorCode()
	}
	return "_OTHER"
}
