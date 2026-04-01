package overlay

import "github.com/jgabor/spela/internal/gpu"

// BuildGPUCollector creates a CollectFunc that reads GPU metrics via NVML
// and evaluates alerts. The returned function is safe to call repeatedly
// from a collector goroutine.
func BuildGPUCollector(position string) CollectFunc {
	pos := ParsePosition(position)
	thresholds := DefaultThresholds()

	return func() SharedState {
		state := SharedState{
			Visible:  true,
			Position: pos,
		}

		metrics, err := gpu.GetGPUMetrics()
		if err != nil {
			return state
		}

		state.Temperature = metrics.Temperature
		state.PowerDraw = metrics.PowerDraw
		state.PowerLimit = metrics.PowerLimit
		state.Utilization = metrics.Utilization
		state.VRAMUsedMB = metrics.MemoryUsed
		state.VRAMTotalMB = metrics.MemoryTotal
		state.GraphicsMHz = metrics.GraphicsClock
		state.MemoryMHz = metrics.MemoryClock
		state.FanSpeed = metrics.FanSpeed

		input := AlertInput{
			Temperature:   metrics.Temperature,
			PowerDraw:     metrics.PowerDraw,
			PowerLimit:    metrics.PowerLimit,
			GraphicsClock: metrics.GraphicsClock,
			FanSpeed:      metrics.FanSpeed,
		}
		if metrics.ThrottleReasons != nil {
			input.ThrottleThermal = metrics.ThrottleReasons.ThermalHardware || metrics.ThrottleReasons.ThermalSoftware
			input.ThrottlePower = metrics.ThrottleReasons.PowerCap || metrics.ThrottleReasons.PowerBrake
		}

		alerts := Evaluate(input, thresholds)
		if len(alerts) > 0 {
			state.AlertActive = true
			highest := AlertInfo
			for _, a := range alerts {
				if a.Severity > highest {
					highest = a.Severity
				}
			}
			state.AlertSeverity = highest
		}

		return state
	}
}
