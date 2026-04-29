package config_test

import (
	"testing"

	"github.com/ellogroup/ello-golang-otel/config"
	"github.com/stretchr/testify/assert"
)

func TestNewFromEnv_Defaults(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_SERVICE_VERSION", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("OTEL_SAMPLE_RATE", "")

	cfg := config.NewFromEnv()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "", cfg.Endpoint)
	assert.Equal(t, "unknown-service", cfg.ServiceName)
	assert.Equal(t, "", cfg.ServiceVersion)
	assert.Equal(t, "unknown", cfg.Environment)
	assert.Equal(t, 1.0, cfg.SampleRate)
}

func TestNewFromEnv_AllSet(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://jaeger:4318")
	t.Setenv("OTEL_SERVICE_NAME", "my-service")
	t.Setenv("OTEL_SERVICE_VERSION", "1.2.3")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("OTEL_SAMPLE_RATE", "0.5")

	cfg := config.NewFromEnv()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "http://jaeger:4318", cfg.Endpoint)
	assert.Equal(t, "my-service", cfg.ServiceName)
	assert.Equal(t, "1.2.3", cfg.ServiceVersion)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, 0.5, cfg.SampleRate)
}
