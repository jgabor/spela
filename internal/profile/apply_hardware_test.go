package profile

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/privilege"
)

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

func TestApplyHardwareBatchesMutationAndRestoration(t *testing.T) {
	var calls [][]string
	operations := profileHardwareOperations{
		currentGovernor: func() (cpu.Governor, error) { return cpu.Governor("powersave"), nil },
		smtStatus:       func() (bool, error) { return true, nil },
		powerLimit:      func() (int, error) { return 250, nil },
		execSelf: func(args ...string) (*privilege.ExecResult, error) {
			calls = append(calls, append([]string(nil), args...))
			return &privilege.ExecResult{}, nil
		},
	}
	profile := Profile{
		GPU: GPUSettings{ClockOffset: 150, MemoryOffset: 500, PowerLimit: 300, FanSpeed: 70},
		CPU: CPUSettings{Governor: "performance", SMT: boolPtr(false)},
	}
	cleanup, err := profile.applyHardwareWithOperations(operations)
	if err != nil || cleanup.Area != "hardware" || cleanup.Run == nil {
		t.Fatalf("applyHardware = cleanup %+v, error %v", cleanup, err)
	}
	wantApply := []string{"apply-profile", "--gpu-clock-offset=150", "--gpu-memory-offset=500", "--gpu-power-limit=300", "--gpu-fan-speed=70", "--cpu-governor=performance", "--cpu-smt=off"}
	if !reflect.DeepEqual(calls, [][]string{wantApply}) {
		t.Fatalf("apply batch = %v, want %v", calls, wantApply)
	}
	if err := cleanup.Run(); err != nil {
		t.Fatal(err)
	}
	wantRestore := []string{"apply-profile", "--reset", "--gpu-power-limit=250", "--gpu-fan-speed=0", "--cpu-governor=powersave", "--cpu-smt=on"}
	if !reflect.DeepEqual(calls[1], wantRestore) {
		t.Fatalf("restore batch = %v, want %v", calls[1], wantRestore)
	}
}

func TestApplyHardwareSurfacesApplyAndRestoreFailures(t *testing.T) {
	failure := errors.New("pkexec unavailable")
	operations := profileHardwareOperations{
		currentGovernor: func() (cpu.Governor, error) { return "", nil },
		smtStatus:       func() (bool, error) { return false, nil },
		powerLimit:      func() (int, error) { return 0, nil },
		execSelf:        func(...string) (*privilege.ExecResult, error) { return nil, failure },
	}
	profile := Profile{GPU: GPUSettings{ClockOffset: 1}}
	if _, err := profile.applyHardwareWithOperations(operations); !errors.Is(err, failure) || !strings.Contains(err.Error(), "apply hardware settings") {
		t.Fatalf("apply failure = %v", err)
	}
	calls := 0
	operations.execSelf = func(...string) (*privilege.ExecResult, error) {
		calls++
		if calls == 1 {
			return &privilege.ExecResult{}, nil
		}
		return nil, failure
	}
	cleanup, err := profile.applyHardwareWithOperations(operations)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup.Run(); !errors.Is(err, failure) || !strings.Contains(err.Error(), "restore hardware settings") {
		t.Fatalf("restore failure = %v", err)
	}
}
