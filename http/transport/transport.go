// Package transport provides an OpenTelemetry-instrumented HTTP transport.
// It wraps outgoing HTTP requests in a client span and propagates W3C trace context
// via the traceparent / tracestate headers, enabling distributed tracing across services.
package transport

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"
)

// New wraps base with an OpenTelemetry HTTP transport that:
//   - Creates a child client span for every outgoing request.
//   - Injects W3C traceparent and tracestate headers so downstream services can
//     continue the trace.
//   - Records HTTP attributes (method, URL, status code) on the span.
//
// The global TracerProvider (set by provider.NewTracerProvider) is used automatically.
// When OTEL is disabled the transport is still safe to use — the no-op provider produces
// zero-overhead spans and no headers are injected.
func New(base http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(base)
}
