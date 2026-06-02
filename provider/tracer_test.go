package provider_test

import (
	"context"
	"github.com/ellogroup/ello-golang-otel/config"
	"github.com/ellogroup/ello-golang-otel/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
