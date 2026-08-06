package provider_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ellogroup/ello-golang-otel/config"
	"github.com/ellogroup/ello-golang-otel/provider"
)

func TestNewTracerProvider_Disabled(t *testing.T) {
	tests := []struct {
		name string
		opts []provider.Option
	}{
		{name: "no options"},
		{name: "WithLambda", opts: []provider.Option{provider.WithLambda()}},
		{name: "WithXRay", opts: []provider.Option{provider.WithXRay()}},
		{name: "WithLambda and WithXRay", opts: []provider.Option{provider.WithLambda(), provider.WithXRay()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{Enabled: false, ServiceName: "test-service"}

			tracer, shutdown, err := provider.NewTracerProvider(context.Background(), cfg, tt.opts...)

			require.NoError(t, err)
			assert.NotNil(t, tracer)
			assert.NotNil(t, shutdown)
			assert.NoError(t, shutdown(context.Background()))
		})
	}
}

func TestNewLambdaTracerProvider_Disabled(t *testing.T) {
	cfg := config.Config{Enabled: false, ServiceName: "test-service"}

	tracer, shutdown, err := provider.NewLambdaTracerProvider(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, tracer)
	assert.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}

func TestNewTracerProvider_Enabled_PathLessEndpoint(t *testing.T) {
	// Regression test: a bare host:port endpoint (no path), like Aspire/ADOT collector
	// endpoints, must still land on /v1/traces rather than 404ing against the root.
	// Only the /v1/traces path is registered, so this would fail without
	// resolveSignalEndpoint filling in the default path.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{Enabled: true, ServiceName: "test-service", Endpoint: srv.URL, SampleRate: 1.0}

	tracer, shutdown, err := provider.NewTracerProvider(context.Background(), cfg, provider.WithLambda())
	require.NoError(t, err)
	require.NotNil(t, tracer)

	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	assert.NoError(t, shutdown(context.Background()))
	assert.Equal(t, "/v1/traces", gotPath)
}

func TestNewTracerProvider_Enabled_InvalidEndpoint(t *testing.T) {
	cfg := config.Config{Enabled: true, ServiceName: "test-service", Endpoint: "://not-a-url"}

	_, _, err := provider.NewTracerProvider(context.Background(), cfg)

	assert.Error(t, err)
}
