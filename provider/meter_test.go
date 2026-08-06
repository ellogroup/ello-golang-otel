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

func TestNewMeterProvider_Disabled(t *testing.T) {
	cfg := config.Config{Enabled: false, ServiceName: "test-service"}

	meter, shutdown, err := provider.NewMeterProvider(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, meter)
	assert.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}

func TestNewMeterProvider_Enabled_PathLessEndpoint(t *testing.T) {
	// Regression test: a bare host:port endpoint (no path), like Aspire/ADOT collector
	// endpoints, must still land on /v1/metrics rather than 404ing against the root.
	// Only the /v1/metrics path is registered, so this would fail without
	// resolveSignalEndpoint filling in the default path.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{Enabled: true, ServiceName: "test-service", Endpoint: srv.URL}

	meter, shutdown, err := provider.NewMeterProvider(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, meter)

	assert.NoError(t, shutdown(context.Background()))
	assert.Equal(t, "/v1/metrics", gotPath)
}

func TestNewMeterProvider_Enabled_InvalidEndpoint(t *testing.T) {
	cfg := config.Config{Enabled: true, ServiceName: "test-service", Endpoint: "://not-a-url"}

	_, _, err := provider.NewMeterProvider(context.Background(), cfg)

	assert.Error(t, err)
}
