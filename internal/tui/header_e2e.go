//go:build e2e

package tui

import (
	"os"
	"sync/atomic"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/overlay"
)

func init() {
	if os.Getenv("SPELA_E2E_METRICS") != "successive-widths" {
		return
	}

	var sequence atomic.Uint64
	samples := []metricsMsg{
		{
			gpuMetrics: &gpu.GPUMetrics{Temperature: 100, PowerDraw: 307, PowerLimit: 300, Utilization: 100, MemoryUsed: 8192, MemoryTotal: 12288},
			cpuMetrics: &cpu.CPUMetrics{AverageFrequency: 5365, Utilization: 100, RAMUsedMB: 16384, RAMTotalMB: 32768},
			alerts:     []overlay.Alert{{Type: overlay.AlertPowerLimit, Severity: overlay.AlertWarning}},
		},
		{
			gpuMetrics: &gpu.GPUMetrics{Temperature: 9, PowerDraw: 48, PowerLimit: 300, Utilization: 9, MemoryUsed: 512, MemoryTotal: 12288},
			cpuMetrics: &cpu.CPUMetrics{AverageFrequency: 842, Utilization: 9, RAMUsedMB: 1024, RAMTotalMB: 32768},
		},
		{
			gpuMetrics: &gpu.GPUMetrics{Temperature: 105, PowerDraw: 1234, PowerLimit: 300, Utilization: 100, MemoryUsed: 10240, MemoryTotal: 12288},
			cpuMetrics: &cpu.CPUMetrics{AverageFrequency: 12345, Utilization: 100, RAMUsedMB: 24576, RAMTotalMB: 32768},
		},
	}
	readSystemMetrics = func() metricsMsg {
		return samples[(sequence.Add(1)-1)%uint64(len(samples))]
	}
}
