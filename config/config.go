package config

import (
	dflt "github.com/ellogroup/ello-golang-otel/internal/default"
	"os"
)

// Config holds the OpenTelemetry configuration read from environment variables.
type Config struct {
	// Enabled controls whether OTEL tracing and metrics are active.
	// When false, no-op providers are used with zero overhead.
	Enabled bool

	// Endpoint is the OTLP HTTP exporter endpoint (e.g. "http://jaeger:4318").
	// Must not include a path — the exporter appends /v1/traces and /v1/metrics automatically.
	Endpoint string

	// ServiceName is the logical name of the service reported to the OTEL backend.
	ServiceName string

	// ServiceVersion is the version of the deployed service (e.g. "1.2.3" or a git SHA).
	ServiceVersion string

	// Environment is the deployment environment (e.g. "production", "internal-test").
	Environment string

	// SampleRate is the fraction of traces to sample (0.0–1.0). Defaults to 1.0 (sample all).
	SampleRate float64
}

// NewFromEnv reads OTEL configuration from environment variables.
//
// Environment variables:
//
//	OTEL_ENABLED                  — "true"/"false" (default: false)
//	OTEL_EXPORTER_OTLP_ENDPOINT   — OTLP HTTP base URL (e.g. "http://jaeger:4318")
//	OTEL_SERVICE_NAME             — service name reported to the backend
//	OTEL_SERVICE_VERSION          — service version (optional)
//	ENVIRONMENT                   — deployment environment (optional)
//	OTEL_SAMPLE_RATE              — sampling rate 0.0–1.0 (default: 1.0)
func NewFromEnv() Config {
	return Config{
		Enabled:        dflt.StrToBoolOrDefault(os.Getenv("OTEL_ENABLED"), false),
		Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:    dflt.NonEmptyOrDefault(os.Getenv("OTEL_SERVICE_NAME"), "unknown-service"),
		ServiceVersion: os.Getenv("OTEL_SERVICE_VERSION"),
		Environment:    dflt.NonEmptyOrDefault(os.Getenv("ENVIRONMENT"), "unknown"),
		SampleRate:     dflt.StrToFloat64OrDefault(os.Getenv("OTEL_SAMPLE_RATE"), 1.0),
	}
}
