// Package middleware provides AWS SDK v2 OpenTelemetry instrumentation.
// It wraps every AWS SDK call in a client span using the otelaws contrib package,
// recording service name, operation, region, and request ID as span attributes.
package middleware

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

// AppendToConfig attaches OTel instrumentation middleware to an existing aws.Config.
// It mutates cfg.APIOptions in-place, adding SDK middleware steps that:
//   - Start a client span for each AWS API call
//   - Record aws.service, aws.operation, aws.region, aws.request_id attributes
//   - Propagate W3C trace context into outgoing HTTP request headers
//   - Set span error status on failed calls
//
// The global TracerProvider is used automatically (registered by provider.NewTracerProvider).
// When OTEL is disabled the global provider is a no-op so spans are zero-overhead.
//
// Do NOT also wrap the AWS HTTP transport with oteltransport.New — otelaws handles
// trace propagation at the SDK middleware layer; double-wrapping creates duplicate spans.
func AppendToConfig(cfg *aws.Config) {
	otelaws.AppendMiddlewares(&cfg.APIOptions)
}
