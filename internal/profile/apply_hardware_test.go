package profile

import "testing"

func TestNeedsHardwareApply(t *testing.T) {
	tests := []struct {
		name     string
		profile  Profile
		expected bool
	}{
		{
			name:     "empty profile",
			profile:  Profile{},
			expected: false,
		},
		{
			name:     "only env settings",
			profile:  Profile{GPU: GPUSettings{ShaderCache: true, ThreadedOptimization: true}},
			expected: false,
		},
		{
			name:     "clock offset set",
			profile:  Profile{GPU: GPUSettings{ClockOffset: 100}},
			expected: true,
		},
		{
			name:     "memory offset set",
			profile:  Profile{GPU: GPUSettings{MemoryOffset: 8001}},
			expected: true,
		},
		{
			name:     "governor set",
			profile:  Profile{CPU: CPUSettings{Governor: "performance"}},
			expected: true,
		},
		{
			name:     "smt set",
			profile:  Profile{CPU: CPUSettings{SMT: boolPtr(false)}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.needsHardwareApply(); got != tt.expected {
				t.Errorf("needsHardwareApply() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
