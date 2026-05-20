package config_test

import (
	"github.com/ellogroup/ello-golang-otel/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantEnabled bool
		wantConfig  config.Config
	}{
		{
			name: "no env vars set, returns defaults",
			env: map[string]string{
				"OTEL_ENABLED":                "",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "",
				"OTEL_SERVICE_NAME":           "",
				"OTEL_SERVICE_VERSION":        "",
				"ENVIRONMENT":                 "",
				"OTEL_SAMPLE_RATE":            "",
			},
			wantConfig: config.Config{
				Enabled:        false,
				Endpoint:       "",
				ServiceName:    "unknown-service",
				ServiceVersion: "",
				Environment:    "unknown",
				SampleRate:     1.0,
			},
		},
		{
			name: "all env vars set, returns configured values",
			env: map[string]string{
				"OTEL_ENABLED":                "true",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "http://jaeger:4318",
				"OTEL_SERVICE_NAME":           "my-service",
				"OTEL_SERVICE_VERSION":        "1.2.3",
				"ENVIRONMENT":                 "production",
				"OTEL_SAMPLE_RATE":            "0.5",
			},
			wantConfig: config.Config{
				Enabled:        true,
				Endpoint:       "http://jaeger:4318",
				ServiceName:    "my-service",
				ServiceVersion: "1.2.3",
				Environment:    "production",
				SampleRate:     0.5,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			assert.Equalf(t, tt.wantConfig, config.NewFromEnv(), "NewFromEnv()")
		})
	}
}
