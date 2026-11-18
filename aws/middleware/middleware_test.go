package middleware_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	awsmiddleware "github.com/ellogroup/ello-golang-otel/aws/middleware"
)

type stubHTTP struct{ got *http.Request }

func (s *stubHTTP) Do(r *http.Request) (*http.Response, error) {
	s.got = r
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/x-amz-json-1.0"}, "X-Amzn-Requestid": {"req-123"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		Request:    r,
	}, nil
}

func setup(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(prevTP); otel.SetTextMapPropagator(prevProp) })
	return rec
}

func TestAppendToConfig_AddsMiddlewares(t *testing.T) {
	cfg := aws.Config{}
	awsmiddleware.AppendToConfig(&cfg)
	assert.NotEmpty(t, cfg.APIOptions)
}

func TestAppendToConfig_CreatesSpanAndInjectsTraceContext(t *testing.T) {
	rec := setup(t)
	stub := &stubHTTP{}
	cfg := aws.Config{Region: "eu-west-2", Credentials: aws.AnonymousCredentials{}, HTTPClient: stub}
	awsmiddleware.AppendToConfig(&cfg)

	_, err := sqs.NewFromConfig(cfg).ListQueues(context.Background(), &sqs.ListQueuesInput{})
	require.NoError(t, err)

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "SQS.ListQueues", spans[0].Name())
	attrs := map[string]any{}
	for _, kv := range spans[0].Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsInterface()
	}
	assert.Equal(t, "aws-api", attrs["rpc.system.name"])
	assert.Equal(t, "SQS/ListQueues", attrs["rpc.method"])
	assert.Equal(t, "eu-west-2", attrs["aws.region"])
	assert.Equal(t, int64(200), attrs["http.response.status_code"])
	assert.Equal(t, "req-123", attrs["aws.request_id"])
	assert.Equal(t, "aws_sqs", attrs["messaging.system"])

	require.NotNil(t, stub.got)
	assert.Contains(t, stub.got.Header.Get("traceparent"), spans[0].SpanContext().TraceID().String())
}

func TestAppendToConfig_Idempotent(t *testing.T) {
	rec := setup(t)
	cfg := aws.Config{Region: "eu-west-2", Credentials: aws.AnonymousCredentials{}, HTTPClient: &stubHTTP{}}
	awsmiddleware.AppendToConfig(&cfg)
	awsmiddleware.AppendToConfig(&cfg)

	_, err := sqs.NewFromConfig(cfg).ListQueues(context.Background(), &sqs.ListQueuesInput{})
	require.NoError(t, err)
	assert.Len(t, rec.Ended(), 1)
}

func TestAppendToConfig_ResolvesGlobalsLazily(t *testing.T) {
	cfg := aws.Config{Region: "eu-west-2", Credentials: aws.AnonymousCredentials{}, HTTPClient: &stubHTTP{}}
	awsmiddleware.AppendToConfig(&cfg) // before the provider is registered

	rec := setup(t)
	_, err := sqs.NewFromConfig(cfg).ListQueues(context.Background(), &sqs.ListQueuesInput{})
	require.NoError(t, err)
	assert.Len(t, rec.Ended(), 1)
}
