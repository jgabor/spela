package dll

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "3.7.20", "3.7.20", 0},
		{"a less", "3.7.19", "3.7.20", -1},
		{"a greater", "3.7.21", "3.7.20", 1},
		{"major differs", "2.0.0", "3.0.0", -1},
		{"minor differs", "3.6.0", "3.7.0", -1},
		{"different lengths a shorter", "3.7", "3.7.1", -1},
		{"different lengths b shorter", "3.7.1", "3.7", 1},
		{"equal different lengths", "3.7.0", "3.7", 0},
		{"v prefix stripped", "v3.7.20", "3.7.20", 0},
		{"both v prefixed", "v1.0", "v2.0", -1},
		{"four components", "3.7.20.1", "3.7.20.2", -1},
		{"empty strings", "", "", 0},
		{"non-numeric parts", "abc.1", "0.2", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name                 string
		installed, available string
		want                 bool
	}{
		{"newer available", "3.7.10", "3.7.20", true},
		{"same version", "3.7.20", "3.7.20", false},
		{"older available", "3.7.20", "3.7.10", false},
		{"major upgrade", "2.0.0", "3.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewer(tt.installed, tt.available)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.installed, tt.available, got, tt.want)
			}
		})
	}
}
