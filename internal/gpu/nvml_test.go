package gpu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

func TestNVMLMetrics(t *testing.T) {
	if nvml.Init() != nvml.SUCCESS {
		t.Skip("NVML not available")
	}
	_ = nvml.Shutdown()

	Init()
	defer Shutdown()

	if !nvmlAvailable {
		t.Skip("NVML initialization failed")
	}

	metrics, err := GetGPUMetrics()
	if err != nil {
		t.Fatalf("GetGPUMetrics() error: %v", err)
	}

	if metrics.Temperature <= 0 || metrics.Temperature > 120 {
		t.Errorf("unexpected temperature: %d", metrics.Temperature)
	}
	if metrics.PowerDraw <= 0 {
		t.Errorf("unexpected power draw: %f", metrics.PowerDraw)
	}
	if metrics.MemoryTotal <= 0 {
		t.Errorf("unexpected memory total: %d", metrics.MemoryTotal)
	}
	if metrics.GraphicsClock <= 0 {
		t.Errorf("unexpected graphics clock: %d", metrics.GraphicsClock)
	}
	if metrics.ThrottleReasons == nil {
		t.Error("expected ThrottleReasons to be populated via NVML")
	}
}

func TestNvidiaSMIFallbackAndUnavailableControls(t *testing.T) {
	bin := t.TempDir()
	writeExecutable := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable("pkexec", "exit 0")
	writeExecutable("nvidia-smi", "printf '%s\\n' 'RTX 5090, 590.1, 32768, 55, 250.5'")
	t.Setenv("PATH", bin)
	savedAvailable := nvmlAvailable
	nvmlAvailable = false
	t.Cleanup(func() { nvmlAvailable = savedAvailable })

	info, err := GetGPUInfo()
	if err != nil || info["name"] != "RTX 5090" || info["power"] != "250.5 W" {
		t.Fatalf("GetGPUInfo fallback = %#v, %v", info, err)
	}
	writeExecutable("nvidia-smi", "printf '%s\\n' '55, 250.5, 450.0, 98, 4096, 32768, 2800, 14000'")
	metrics, err := GetGPUMetrics()
	if err != nil || metrics.PowerLimit != 450 || metrics.MemoryTotal != 32768 {
		t.Fatalf("GetGPUMetrics fallback = %#v, %v", metrics, err)
	}
	limit, err := GetCurrentPowerLimit()
	if err != nil || limit != 450 {
		t.Fatalf("GetCurrentPowerLimit = %d, %v", limit, err)
	}

	for name, operation := range map[string]func() error{
		"graphics clocks": func() error { return LockGraphicsClocks(100) },
		"memory clocks":   func() error { return LockMemoryClocks(1000) },
		"power limit":     func() error { return SetPowerLimit(350) },
		"reset clocks":    ResetClocks,
	} {
		if err := operation(); err != nil {
			t.Errorf("%s fallback: %v", name, err)
		}
	}
	if err := SetFanSpeed(50); err == nil {
		t.Fatal("SetFanSpeed without NVML unexpectedly succeeded")
	}
	if err := ResetFanSpeed(); err == nil {
		t.Fatal("ResetFanSpeed without NVML unexpectedly succeeded")
	}
}

func TestNvidiaSMIFallbackErrorsAndNVMLUnavailableSetters(t *testing.T) {
	bin := t.TempDir()
	path := filepath.Join(bin, "nvidia-smi")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'short\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	savedAvailable := nvmlAvailable
	nvmlAvailable = false
	t.Cleanup(func() { nvmlAvailable = savedAvailable })
	if _, err := GetGPUInfo(); err == nil {
		t.Fatal("short info output unexpectedly accepted")
	}
	if _, err := GetGPUMetrics(); err == nil {
		t.Fatal("short metrics output unexpectedly accepted")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := GetGPUInfo(); err == nil {
		t.Fatal("failed nvidia-smi info unexpectedly accepted")
	}
	if _, err := GetGPUMetrics(); err == nil {
		t.Fatal("failed nvidia-smi metrics unexpectedly accepted")
	}

	for name, operation := range map[string]func() error{
		"GPU clocks":    func() error { return SetGpuLockedClocksNVML(0, 1) },
		"memory clocks": func() error { return SetMemoryLockedClocksNVML(0, 1) },
		"power":         func() error { return SetPowerManagementLimitNVML(1) },
		"fan":           func() error { return SetFanSpeedNVML(0, 1) },
		"reset GPU":     ResetGpuLockedClocksNVML,
		"reset memory":  ResetMemoryLockedClocksNVML,
		"reset fan":     func() error { return ResetFanSpeedNVML(0) },
	} {
		if err := operation(); err == nil {
			t.Errorf("%s without NVML unexpectedly succeeded", name)
		}
	}
}

func TestThrottleReasonsThrottling(t *testing.T) {
	tests := []struct {
		name     string
		reasons  *ThrottleReasons
		expected bool
	}{
		{"nil", nil, false},
		{"none active", &ThrottleReasons{}, false},
		{"thermal hardware", &ThrottleReasons{ThermalHardware: true}, true},
		{"thermal software", &ThrottleReasons{ThermalSoftware: true}, true},
		{"power cap", &ThrottleReasons{PowerCap: true}, true},
		{"power brake", &ThrottleReasons{PowerBrake: true}, true},
		{"multiple", &ThrottleReasons{ThermalHardware: true, PowerCap: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.reasons.Throttling(); got != tt.expected {
				t.Errorf("Throttling() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNVMLInfo(t *testing.T) {
	if nvml.Init() != nvml.SUCCESS {
		t.Skip("NVML not available")
	}
	_ = nvml.Shutdown()

	Init()
	defer Shutdown()

	if !nvmlAvailable {
		t.Skip("NVML initialization failed")
	}

	info, err := GetGPUInfo()
	if err != nil {
		t.Fatalf("GetGPUInfo() error: %v", err)
	}

	if info["name"] == "" {
		t.Error("expected non-empty GPU name")
	}
	if info["driver"] == "" {
		t.Error("expected non-empty driver version")
	}
}

func TestFallbackWhenNVMLUnavailable(t *testing.T) {
	saved := nvmlAvailable
	nvmlAvailable = false
	defer func() { nvmlAvailable = saved }()

	// With nvmlAvailable=false, GetGPUMetrics uses nvidia-smi fallback.
	// This may fail without nvidia-smi, but must not panic.
	_, _ = GetGPUMetrics()
	_, _ = GetGPUInfo()
}
