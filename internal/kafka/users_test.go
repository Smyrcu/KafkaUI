package kafka

import "testing"

func TestScramMechanismToString(t *testing.T) {
	tests := []struct {
		input    int8
		expected string
	}{
		{1, "SCRAM-SHA-256"},
		{2, "SCRAM-SHA-512"},
		{0, "UNKNOWN"},
		{3, "UNKNOWN"},
		{-1, "UNKNOWN"},
	}

	for _, tt := range tests {
		got := scramMechanismToString(tt.input)
		if got != tt.expected {
			t.Errorf("scramMechanismToString(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestStringToScramMechanism(t *testing.T) {
	tests := []struct {
		input    string
		expected int8
	}{
		{"SCRAM-SHA-256", 1},
		{"SCRAM-SHA-512", 2},
		{"UNKNOWN", 0},
		{"", 0},
		{"PLAIN", 0},
	}

	for _, tt := range tests {
		got := stringToScramMechanism(tt.input)
		if got != tt.expected {
			t.Errorf("stringToScramMechanism(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestScramMechanismRoundTrip(t *testing.T) {
	mechanisms := []string{"SCRAM-SHA-256", "SCRAM-SHA-512"}
	for _, m := range mechanisms {
		code := stringToScramMechanism(m)
		if code == 0 {
			t.Errorf("stringToScramMechanism(%q) returned 0", m)
			continue
		}
		back := scramMechanismToString(code)
		if back != m {
			t.Errorf("round-trip failed: %q -> %d -> %q", m, code, back)
		}
	}
}
