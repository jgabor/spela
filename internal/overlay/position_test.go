package overlay

import "testing"

func TestParsePosition(t *testing.T) {
	tests := []struct {
		input string
		want  uint8
	}{
		{"top-left", 0},
		{"top-right", 1},
		{"bottom-left", 2},
		{"bottom-right", 3},
		{"", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		if got := ParsePosition(tt.input); got != tt.want {
			t.Errorf("ParsePosition(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
