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

`OTEL_EXPORTER_OTLP_ENDPOINT` is normally left path-less — the trace and metric
exporters append `/v1/traces` and `/v1/metrics` respectively. To target the
literal root instead, add a trailing `/` (e.g. `http://jaeger:4318/`).

## Provider

### Tracer

`NewTracerProvider` accepts functional options to configure export and propagation behaviour:

| Option | Effect |
|---|---|
| `WithLambda()` | Synchronous export via `SimpleSpanProcessor` — spans are delivered before the Lambda container is frozen |
| `WithXRay()` | X-Ray ID generator (timestamp-prefixed trace IDs required by ADOT) + X-Ray propagator for `X-Amzn-Trace-Id` headers |

`WithLambda` and `WithXRay` are independent: a Lambda may send to a non-X-Ray backend, and a non-Lambda service could use X-Ray.

```go
cfg := config.NewFromEnv()

// Long-running service, no options needed
tracer, shutdown, err := provider.NewTracerProvider(ctx, cfg)

// Lambda sending to a non-X-Ray backend
tracer, shutdown, err := provider.NewTracerProvider(ctx, cfg, provider.WithLambda())

// Lambda with ADOT/X-Ray (convenience wrapper for the common Ello case)
tracer, shutdown, err := provider.NewLambdaTracerProvider(ctx, cfg)
// equivalent to: provider.NewTracerProvider(ctx, cfg, provider.WithLambda(), provider.WithXRay())

if err != nil {
    // handle error
}
defer shutdown(ctx)
```

W3C TraceContext and Baggage propagators are always registered globally; `WithXRay` additionally registers
the X-Ray propagator. All propagators are registered even when `OTEL_ENABLED=false` so context extraction
works in the disabled case.

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

OTEL middleware wraps Lambda handlers in a root span and records invocation metrics.

For API Gateway v1 handlers, use `NewAPIGatewayV1`. Prepend it to the middleware slice so the span covers
the full request lifecycle:

```go
allMiddlewares := append(
    middleware.APIGatewayV1{otelmiddleware.NewAPIGatewayV1(tracer, meter)},
    commonMiddlewares...,
)
```

The middleware extracts incoming W3C `traceparent` / `tracestate` headers to continue an upstream trace,
and sets the span status to Error on 5xx responses or returned errors.

For event-driven handlers (SQS, SNS, scheduled events) that do not return a response, use `NewNoResponse`:

```go
allMiddlewares := append(
    []awsmiddleware.NoResponse[events.SQSEvent]{otelmiddleware.NewNoResponse[events.SQSEvent](tracer, meter, "process-sqs")},
    commonMiddlewares...,
)
```

Both middlewares record the following metrics on every invocation:

| Metric | Type | Unit | Attributes |
|---|---|---|---|
| `faas.invocations` | Counter | — | `faas.coldstart` (bool), `http.response.status_code` (API Gateway only), `faas.error` (NoResponse only) |
| `faas.duration` | Histogram | ms | same as above |

`faas.coldstart` is `true` only on the first invocation of each Lambda execution environment, making cold starts distinguishable from warm invocations in your metrics backend.

## AWS

### Middleware

Instruments all AWS SDK v2 calls with OpenTelemetry client spans. Each SDK call records `aws.service`,
`aws.operation`, `aws.region`, and `aws.request_id` attributes, and propagates W3C trace context into
outgoing HTTP request headers.

Call `AppendToConfig` once after building your `aws.Config`:

```go
cfg, err := awsconfig.LoadDefaultConfig(ctx)
if err != nil {
    // handle error
}
awsmiddleware.AppendToConfig(&cfg)
```

The global TracerProvider (set by `provider.NewTracerProvider`) is used automatically. When OTEL is
disabled the global provider is a no-op so spans are zero-overhead.

Do **not** also wrap the AWS HTTP transport with `http/transport.New` — `otelaws` handles trace
propagation at the SDK middleware layer; double-wrapping creates duplicate spans.

## slog

### Handler

Wraps any `slog.Handler` to inject `trace_id` and `span_id` from the active OpenTelemetry span into every log record. When no span is active (or OTEL is disabled), no extra attributes are added.

```go
inner := slog.NewJSONHandler(os.Stdout, nil)
logger := slog.New(handler.New(inner))
```

Pass a context carrying an active span when logging and the fields are added automatically:

```go
logger.InfoContext(ctx, "order processed", slog.String("order_id", id))
// {"time":"...","level":"INFO","msg":"order processed","order_id":"...","trace_id":"...","span_id":"..."}
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

## Development

### AI-agent context

This repository carries shared AI-agent context and tooling used across Ello repositories:

- `AGENTS.md` — repository-specific agent context (read this first).
- `.ai-context/` — a git submodule of shared, cross-repo standards, conventions, and skills.
- `CLAUDE.md` — a thin Claude Code-specific pointer into the two files above.

`.ai-context/` is auto-initialised by `make build` when missing (skipped in CI, which does not need AI-agent
context to build or test). To pull the latest shared standards and regenerate Claude Code skill wrappers:

```bash
make sync-ai-context   # updates the .ai-context submodule pointer, then runs sync-skills
make init-memory       # seeds .agents/memory/{progress,decisions,notes,techdebt}.md (idempotent)
```

### Commands

Docker-based (matches CI exactly — requires Docker):

```bash
make build            # build Docker image, go mod tidy, ensure .ai-context is initialised
make format            # gofmt, go fix, goimports (local prefix github.com/ellogroup)
make test              # static-tests + unit-tests
make static-tests      # golangci-lint (config verify + run), gosec, govulncheck
make unit-tests        # go test -v -cover ./...
make build-format-test # build + format + test
```

Local (no Docker):

```bash
go test -v -cover ./...
go vet ./...
go mod tidy
```
