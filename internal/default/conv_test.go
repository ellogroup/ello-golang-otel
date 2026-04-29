package dflt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrToBoolOrDefault(t *testing.T) {
	tests := []struct {
		input    string
		dflt     bool
		expected bool
	}{
		{"true", false, true},
		{"false", true, false},
		{"1", false, true},
		{"0", true, false},
		{"invalid", false, false},
		{"", true, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, StrToBoolOrDefault(tt.input, tt.dflt))
	}
}

func TestStrToFloat64OrDefault(t *testing.T) {
	tests := []struct {
		input    string
		dflt     float64
		expected float64
	}{
		{"0.5", 1.0, 0.5},
		{"1.0", 0.5, 1.0},
		{"invalid", 1.0, 1.0},
		{"", 0.5, 0.5},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, StrToFloat64OrDefault(tt.input, tt.dflt))
	}
}

func TestNonEmptyOrDefault(t *testing.T) {
	tests := []struct {
		input    string
		dflt     string
		expected string
	}{
		{"hello", "default", "hello"},
		{"", "default", "default"},
		{"  ", "default", "  "},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, NonEmptyOrDefault(tt.input, tt.dflt))
	}
}
