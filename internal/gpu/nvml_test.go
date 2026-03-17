package gpu

import (
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
