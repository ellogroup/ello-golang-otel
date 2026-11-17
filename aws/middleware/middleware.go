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

const (
	initializeBeforeID = "OTelInitializeMiddlewareBefore"
	initializeAfterID  = "OTelInitializeMiddlewareAfter"
	finalizeID         = "OTelFinalizeMiddleware"
	deserializeID      = "OTelDeserializeMiddleware"
)

// AppendToConfig attaches OTel instrumentation middleware to an existing aws.Config.
// It mutates cfg.APIOptions in-place, adding SDK middleware steps that:
//   - Start a client span for each AWS API call
//   - Record rpc.system.name, rpc.method, aws.region, aws.request_id and http.response.status_code attributes
//   - Inject trace context into outgoing HTTP request headers
//   - Set span error status on failed calls
//
// The global TracerProvider and TextMapPropagator are resolved on each call, so it does not matter
// whether AppendToConfig runs before or after provider.NewTracerProvider. When OTEL is disabled the
// global provider is a no-op so spans are zero-overhead.
//
// AppendToConfig is idempotent: calling it more than once on the same config does not add the
// middleware twice. Note that all headers produced by the global propagator (including W3C baggage,
// if set) are sent to AWS endpoints, so do not put sensitive data in baggage.
//
// Do NOT also wrap the AWS HTTP transport with oteltransport.New — this middleware already
// injects trace context; double-wrapping creates duplicate spans.
func AppendToConfig(cfg *aws.Config) {
	cfg.APIOptions = append(cfg.APIOptions, initializeBefore, initializeAfter, finalize, deserialize)
}

func initializeBefore(stack *middleware.Stack) error {
	if _, ok := stack.Initialize.Get(initializeBeforeID); ok {
		return nil
	}
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc(initializeBeforeID, func(
		ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
	) (middleware.InitializeOutput, middleware.Metadata, error) {
		ctx = context.WithValue(ctx, startTimeKey{}, time.Now())
		return next.HandleInitialize(ctx, in)
	}), middleware.Before)
}

func initializeAfter(stack *middleware.Stack) error {
	if _, ok := stack.Initialize.Get(initializeAfterID); ok {
		return nil
	}
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc(initializeAfterID, func(
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

		ctx, span := otel.GetTracerProvider().Tracer(ScopeName).Start(ctx, spanName(serviceID, operation), startOpts...)
		defer span.End()

		out, md, err := next.HandleInitialize(ctx, in)
		if err != nil {
			span.SetAttributes(errorTypeKey.String(errorType(err)))
			span.SetStatus(codes.Error, err.Error())
		}
		return out, md, err
	}), middleware.After)
}

func finalize(stack *middleware.Stack) error {
	if _, ok := stack.Finalize.Get(finalizeID); ok {
		return nil
	}
	return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc(finalizeID, func(
		ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
	) (middleware.FinalizeOutput, middleware.Metadata, error) {
		if req, ok := in.Request.(*smithyhttp.Request); ok {
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
		}
		return next.HandleFinalize(ctx, in)
	}), middleware.After)
}

func deserialize(stack *middleware.Stack) error {
	if _, ok := stack.Deserialize.Get(deserializeID); ok {
		return nil
	}
	return stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc(deserializeID, func(
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
