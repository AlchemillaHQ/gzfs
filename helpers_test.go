package gzfs

import (
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{"empty string", "", 0},
		{"dash", "-", 0},
		{"spaces", "   ", 0},
		{"bytes", "1024", 1024},
		{"bytes with B", "1024B", 1024},
		{"kilobytes", "1K", 1024},
		{"kilobytes with B", "1KB", 1024},
		{"megabytes", "2M", 2 * 1024 * 1024},
		{"megabytes with B", "2MB", 2 * 1024 * 1024},
		{"gigabytes", "3G", 3 * 1024 * 1024 * 1024},
		{"gigabytes with B", "3GB", 3 * 1024 * 1024 * 1024},
		{"terabytes", "1T", 1024 * 1024 * 1024 * 1024},
		{"terabytes with B", "1TB", 1024 * 1024 * 1024 * 1024},
		{"petabytes", "1P", 1024 * 1024 * 1024 * 1024 * 1024},
		{"petabytes with B", "1PB", 1024 * 1024 * 1024 * 1024 * 1024},
		{"decimal value", "1.5G", uint64(1.5 * 1024 * 1024 * 1024)},
		{"percentage (should be 0)", "75%", 0},
		{"ratio (should be 0)", "2.5x", 0},
		{"invalid unit", "100Z", 0},
		{"invalid number", "abc", 0},
		{"lowercase units", "1g", 1024 * 1024 * 1024},
		{"mixed case", "1gb", 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSize(tt.input)
			if result != tt.expected {
				t.Errorf("ParseSize(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseRatio(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"empty string", "", 0},
		{"dash", "-", 0},
		{"spaces", "   ", 0},
		{"simple ratio", "1.50x", 1.50},
		{"ratio without x", "2.25", 2.25},
		{"integer ratio", "3x", 3.0},
		{"zero ratio", "0x", 0.0},
		{"invalid ratio", "abc", 0},
		{"negative ratio", "-1.5x", -1.5},
		{"high precision", "1.123456x", 1.123456},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseRatio(tt.input)
			if result != tt.expected {
				t.Errorf("ParseRatio(%q) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"dash", "-", ""},
		{"spaces", "   ", ""},
		{"normal string", "hello", "hello"},
		{"string with spaces", "  hello  ", "hello"},
		{"dash with spaces", "  -  ", ""},
		{"path", "/tank/data", "/tank/data"},
		{"none value", "none", "none"},
		{"off value", "off", "off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseString(tt.input)
			if result != tt.expected {
				t.Errorf("ParseString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseUint64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{"empty string", "", 0},
		{"spaces", "   ", 0},
		{"zero", "0", 0},
		{"positive number", "12345", 12345},
		{"large number", "18446744073709551615", 18446744073709551615}, // max uint64
		{"invalid number", "abc", 0},
		{"negative number", "-123", 0}, // ParseUint should fail on negative
		{"number with spaces", "  123  ", 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseUint64(tt.input)
			if result != tt.expected {
				t.Errorf("ParseUint64(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateDeterministicUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"simple string", "test"},
		{"complex string", "tank-myencryptionkey"},
		{"path string", "/tank/data"},
		{"special characters", "test@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result1 := GenerateDeterministicUUID(tt.input)
			result2 := GenerateDeterministicUUID(tt.input)

			// Should be deterministic (same input = same output)
			if result1 != result2 {
				t.Errorf("GenerateDeterministicUUID is not deterministic for input %q: got %q and %q", tt.input, result1, result2)
			}

			// Should be valid UUID format (36 characters with dashes)
			if len(result1) != 36 {
				t.Errorf("GenerateDeterministicUUID(%q) = %q, expected 36 character UUID", tt.input, result1)
			}

			// Should not be empty
			if result1 == "" {
				t.Errorf("GenerateDeterministicUUID(%q) returned empty string", tt.input)
			}
		})
	}

	// Test that different inputs produce different UUIDs
	uuid1 := GenerateDeterministicUUID("input1")
	uuid2 := GenerateDeterministicUUID("input2")
	if uuid1 == uuid2 {
		t.Errorf("GenerateDeterministicUUID should produce different UUIDs for different inputs, but got same: %q", uuid1)
	}
}
