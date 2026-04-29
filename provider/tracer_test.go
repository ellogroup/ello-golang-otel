package provider_test

import (
	"context"
	"testing"

	"github.com/ellogroup/ello-golang-otel/config"
	"github.com/ellogroup/ello-golang-otel/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTracerProvider_Disabled(t *testing.T) {
	cfg := config.Config{Enabled: false, ServiceName: "test-service"}

	tracer, shutdown, err := provider.NewTracerProvider(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, tracer)
	assert.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}
