# Agent Context — This Repository

> Read this file first, then follow the reading order below. The shared
> standards for all Ello repositories live in the `ai-context` submodule at
> `.ai-context/`. If `.ai-context/` is empty, run `make sync-ai-context`
> before continuing.

---

## Reading order

1. `.ai-context/AGENTS.md` — universal standards for all teams.
2. `.ai-context/standards/security.md` — absolute security constraints.
3. `.ai-context/teams/backend/AGENTS.md` — Go conventions for this stack. Note: the repository-structure and Lambda-handler sections of that document describe an *application* repository (`app/cmd/`, Terraform, API Gateway) — this repository is a library, not an app, so those sections do not apply here. See "This repository" below for what actually applies.
4. `.ai-context/skills/documentation/SKILL.md` — how to maintain `.agents/memory/`.
5. `.ai-context/skills/spec/SKILL.md` — spec-driven development workflow.
6. This repository's `README.md` — public API documentation, setup, commands.
7. `.agents/memory/progress.md`, `decisions.md`, `notes.md`, `techdebt.md` — session memory. If absent, run `make init-memory` to seed them from `.ai-context/skills/documentation/assets/`.

Load other documents under `.ai-context/` on demand — for example
`.ai-context/teams/backend/conventions/go.md` before writing Go.
`.ai-context/teams/backend/conventions/api-design.md` and the Lambda-handler
and Terraform sections of `teams/backend/AGENTS.md` do not apply to this
repository (see below) and can be skipped.

---

## This repository

`ello-golang-otel` (`github.com/ellogroup/ello-golang-otel`) is a **shared
Go library**, not an application. It provides OpenTelemetry (OTEL)
integration consumed by other Ello backend repositories — it is the
"provided by `ello-golang-otel` and `ello-golang-aws` directly" middleware
referenced in `.ai-context/teams/backend/AGENTS.md` §2 for event-driven
Lambda handlers. There is no Lambda runtime, API Gateway integration, or
Terraform of its own; consuming repositories wire this library into their
own handlers.

**Deliberate departures from the template/backend conventions** — this is
a pure library, so the assumptions in `teams/backend/AGENTS.md` about
application repository shape do not hold here:

- **Single Go module at the repository root** (`github.com/ellogroup/ello-golang-otel`),
  not `app/` + `test/` submodules. There is no `app/cmd/` Lambda entry
  point, no `infrastructure/` Terraform, and no separate Godog integration
  test module — this repo has no running application to integrate-test.
  Unit tests live alongside the code they test (`foo.go` → `foo_test.go`),
  same convention as an app repo, just without the second module.
- No `docker-compose.yml` / LocalStack — nothing here needs a running
  local environment, only `docker build` + `go test`/`golangci-lint` inside
  the built image (see `Makefile`).

### Package architecture

Six packages with a clear dependency hierarchy, plus one internal helper
package:

| Package | Purpose |
|---|---|
| `config/` | Reads all OTEL configuration from environment variables via `config.NewFromEnv()`. Key env vars: `OTEL_ENABLED`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OTEL_SERVICE_VERSION`, `ENVIRONMENT`, `OTEL_SAMPLE_RATE`. |
| `provider/` | Creates and globally registers OTEL providers: `NewTracerProvider(ctx, cfg, ...opts)` (with `WithLambda()` / `WithXRay()` options, plus the `NewLambdaTracerProvider(ctx, cfg)` convenience wrapper) and `NewMeterProvider(ctx, cfg)`, each returning a provider + shutdown func. When `OTEL_ENABLED=false`, both return no-op providers with zero overhead. Both always register W3C TraceContext and Baggage propagators globally. |
| `lambda/middleware/` | AWS Lambda middleware that wraps handlers and instruments them: `NewAPIGatewayV1(tracer, meter)` for API Gateway v1 HTTP handlers (extracts W3C trace context from request headers, creates server spans, sets Error on 5xx or handler errors); `NewNoResponse[E any](tracer, meter, spanName)` for event-driven handlers (SQS, SNS, scheduled events) with no response instrumentation. |
| `aws/middleware/` | `AppendToConfig(cfg *aws.Config)` instruments all AWS SDK v2 calls with OTel client spans (service, operation, region, request ID attributes) via the `otelaws` contrib package. Do not also wrap the AWS HTTP transport with `http/transport.New` — double-wrapping creates duplicate spans. |
| `slog/handler/` | `New(inner slog.Handler)` wraps any `slog.Handler` to inject `trace_id` and `span_id` from the active OTEL span into every log record. No-op when OTEL is disabled. |
| `http/transport/` | `New(base RoundTripper)` wraps an HTTP client's transport with OTEL instrumentation: creates child spans, injects W3C `traceparent`/`tracestate` headers for downstream propagation, and records HTTP attributes. |
| `internal/default/` | Private helpers for parsing env vars with defaults: `StrToBoolOrDefault`, `StrToFloat64OrDefault`, `NonEmptyOrDefault`. |

### Key design decisions

- **Zero overhead when disabled** — no-op providers ensure no performance
  cost when `OTEL_ENABLED=false`.
- **Global registration** — providers call `otel.SetTracerProvider()` /
  `otel.SetMeterProvider()` so callers use standard OTEL APIs without
  holding provider references.
- **Parent-based sampling** — uses `ParentBased(sampler)`; downstream
  services inherit the upstream sampling decision. Rate configurable via
  `OTEL_SAMPLE_RATE`.
- **Ello ecosystem dependency** — `ello-golang-aws` supplies the Lambda
  middleware interfaces this library wraps.

### Documentation

`README.md` is the public-facing documentation for this library. Keep it
up to date whenever packages are added, removed, or their behaviour
changes — update it as part of any PR that affects the public API or
package architecture.

| Concern | Where to look |
|---|---|
| Local commands | `Makefile`, `README.md` |
| Library code | `config/`, `provider/`, `lambda/middleware/`, `aws/middleware/`, `slog/handler/`, `http/transport/`, `internal/default/` |
| Unit tests | Alongside each package (`*_test.go`) — no separate test module |
| Session memory / handoff notes | `.agents/memory/` |

This section is a stable overview, not a change log: don't duplicate
`.agents/memory/decisions.md` or `progress.md` here, just keep the
high-level "what is this library and how is it shaped" summary current as
the package set evolves.

---

## Updating shared context

`.ai-context/` is a git submodule pinned to a specific commit. To pull the
latest shared context:

```bash
make sync-ai-context
```

Treat each update as a dependency bump — review the diff before committing
the new submodule pointer.
