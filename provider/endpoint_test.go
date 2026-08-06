package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSignalEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{"bare host:port, appends default path", "http://localhost:4318", "http://localhost:4318/v1/traces"},
		{"bare host:port no scheme path, appends default path", "http://aspire:18890", "http://aspire:18890/v1/traces"},
		{"trailing slash targets root explicitly, left untouched", "http://localhost:4318/", "http://localhost:4318/"},
		{"explicit custom path, left untouched", "http://localhost:4318/custom", "http://localhost:4318/custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSignalEndpoint(tt.endpoint, "v1/traces")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveSignalEndpoint_InvalidURL(t *testing.T) {
	_, err := resolveSignalEndpoint("://not-a-url", "v1/traces")
	assert.Error(t, err)
}
