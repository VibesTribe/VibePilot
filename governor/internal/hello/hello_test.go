package hello

import "testing"

func TestGreet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "greet World",
			input:    "World",
			expected: "Hello, World!",
		},
		{
			name:     "empty name",
			input:    "",
			expected: "Hello, !",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Greet(tc.input)
			if got != tc.expected {
				t.Errorf("Greet(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}
