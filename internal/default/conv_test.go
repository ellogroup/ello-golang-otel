package dflt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrToBoolOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dflt     bool
		expected bool
	}{
		{"non-empty true string, returns true", "true", false, true},
		{"non-empty false string, returns false", "false", true, false},
		{"1, returns true", "1", false, true},
		{"0, returns false", "0", true, false},
		{"invalid string, returns default", "invalid", false, false},
		{"empty string, returns default", "", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expected, StrToBoolOrDefault(tt.input, tt.dflt), "StrToBoolOrDefault(%q, %v)", tt.input, tt.dflt)
		})
	}
}

func TestStrToFloat64OrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dflt     float64
		expected float64
	}{
		{"valid decimal, returns parsed value", "0.5", 1.0, 0.5},
		{"valid decimal whole number, returns parsed value", "1.0", 0.5, 1.0},
		{"zero decimal, returns 0.0", "0.0", 1.0, 0.0},
		{"zero integer, returns 0.0", "0", 1.0, 0.0},
		{"negative decimal, returns parsed value", "-1.0", 0.0, -1.0},
		{"negative small decimal, returns parsed value", "-0.5", 0.0, -0.5},
		{"integer without decimal, returns parsed value", "1", 0.0, 1.0},
		{"large integer, returns parsed value", "42", 0.0, 42.0},
		{"invalid string, returns default", "invalid", 1.0, 1.0},
		{"empty string, returns default", "", 0.5, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expected, StrToFloat64OrDefault(tt.input, tt.dflt), "StrToFloat64OrDefault(%q, %v)", tt.input, tt.dflt)
		})
	}
}

func TestNonEmptyOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dflt     string
		expected string
	}{
		{"non-empty string, returns input", "hello", "default", "hello"},
		{"empty string, returns default", "", "default", "default"},
		{"whitespace string, returns input", "  ", "default", "  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expected, NonEmptyOrDefault(tt.input, tt.dflt), "NonEmptyOrDefault(%q, %q)", tt.input, tt.dflt)
		})
	}
}
