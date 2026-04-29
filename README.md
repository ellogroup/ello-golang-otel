# Ello Go OpenTelemetry

Common packages for integrating OpenTelemetry distributed tracing and metrics.

## Configuration

Configuration is read from environment variables using `config.NewFromEnv()`.

| Variable | Description | Default |
|---|---|---|
| `OTEL_ENABLED` | Enable OTEL tracing and metrics | `false` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP HTTP exporter base URL (e.g. `http://jaeger:4318`) | — |
| `OTEL_SERVICE_NAME` | Service name reported to the OTEL backend | `unknown-service` |
| `OTEL_SERVICE_VERSION` | Service version (e.g. a semver or git SHA) | — |
| `ENVIRONMENT` | Deployment environment (e.g. `production`) | `unknown` |
| `OTEL_SAMPLE_RATE` | Fraction of traces to sample, `0.0`–`1.0` | `1.0` |

When `OTEL_ENABLED` is `false`, all providers return no-op implementations with zero overhead.

## Provider

### Tracer

Creates and globally registers a TracerProvider. Returns a `trace.Tracer` scoped to the service and a shutdown
function to call on Lambda shutdown.

```go
cfg := config.NewFromEnv()
tracer, shutdown, err := provider.NewTracerProvider(ctx, cfg)
if err != nil {
    // handle error
}
defer shutdown(ctx)
```

W3C TraceContext and Baggage propagators are always registered globally, enabling trace context extraction
even when OTEL is disabled.

Sampling uses a parent-based strategy: if an upstream service sampled the trace, the decision is inherited.
The local rate is controlled by `OTEL_SAMPLE_RATE`.

### Meter

Creates and globally registers a MeterProvider. Returns a `metric.Meter` scoped to the service and a shutdown
function to flush pending metrics on Lambda shutdown.

```go
cfg := config.NewFromEnv()
meter, shutdown, err := provider.NewMeterProvider(ctx, cfg)
if err != nil {
    // handle error
}
defer shutdown(ctx)
```

## Lambda

### Middleware

OTEL middleware wraps Lambda handlers in a root span and injects `trace_id` and `span_id` into
[logctx](https://github.com/ellogroup/ello-golang-ctx) so they appear automatically on every structured log line.

For API Gateway v1 handlers, use `NewAPIGatewayV1`. Prepend it to the middleware slice so the span covers
the full request lifecycle:

```go
allMiddlewares := append(
    middleware.APIGatewayV1{otelmiddleware.NewAPIGatewayV1(tracer)},
    commonMiddlewares...,
)
```

The middleware extracts incoming W3C `traceparent` / `tracestate` headers to continue an upstream trace,
and sets the span status to Error on 5xx responses or returned errors.

For event-driven handlers (SQS, SNS, scheduled events) that do not return a response, use `NewNoResponse`:

```go
allMiddlewares := append(
    []awsmiddleware.NoResponse[events.SQSEvent]{otelmiddleware.NewNoResponse[events.SQSEvent](tracer, "process-sqs")},
    commonMiddlewares...,
)
```

## HTTP

### Transport

Wraps an `http.RoundTripper` with OpenTelemetry instrumentation. Creates a child client span for each outgoing
request and injects W3C `traceparent` / `tracestate` headers so downstream services can continue the trace.

```go
client := &http.Client{
    Transport: transport.New(http.DefaultTransport),
}
```

The global TracerProvider (set by `provider.NewTracerProvider`) is used automatically. When OTEL is disabled
the transport is safe to use with zero overhead.
