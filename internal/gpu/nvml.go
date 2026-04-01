package gpu

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

var (
	nvmlAvailable bool
	nvmlDevice    nvml.Device
)

// initNVML initializes NVML and obtains a handle for GPU 0.
// Returns true if NVML is available and ready.
func initNVML() bool {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return false
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS || count == 0 {
		_ = nvml.Shutdown()
		return false
	}

	device, ret := nvml.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		_ = nvml.Shutdown()
		return false
	}

	nvmlDevice = device
	nvmlAvailable = true
	return true
}

func shutdownNVML() {
	_ = nvml.Shutdown()
	nvmlAvailable = false
}

func getMetricsNVML() (*GPUMetrics, error) {
	metrics := &GPUMetrics{}

	if temp, ret := nvmlDevice.GetTemperature(nvml.TEMPERATURE_GPU); ret == nvml.SUCCESS {
		metrics.Temperature = int(temp)
	}

	if power, ret := nvmlDevice.GetPowerUsage(); ret == nvml.SUCCESS {
		metrics.PowerDraw = float64(power) / 1000.0
	}

	if limit, ret := nvmlDevice.GetPowerManagementLimit(); ret == nvml.SUCCESS {
		metrics.PowerLimit = float64(limit) / 1000.0
	}

	if util, ret := nvmlDevice.GetUtilizationRates(); ret == nvml.SUCCESS {
		metrics.Utilization = int(util.Gpu)
	}

	if mem, ret := nvmlDevice.GetMemoryInfo(); ret == nvml.SUCCESS {
		metrics.MemoryUsed = int(mem.Used / (1024 * 1024))
		metrics.MemoryTotal = int(mem.Total / (1024 * 1024))
	}

	if clock, ret := nvmlDevice.GetClockInfo(nvml.CLOCK_GRAPHICS); ret == nvml.SUCCESS {
		metrics.GraphicsClock = int(clock)
	}

	if clock, ret := nvmlDevice.GetClockInfo(nvml.CLOCK_MEM); ret == nvml.SUCCESS {
		metrics.MemoryClock = int(clock)
	}

	if fan, ret := nvmlDevice.GetFanSpeed(); ret == nvml.SUCCESS {
		metrics.FanSpeed = int(fan)
	}

	if reasons, ret := nvmlDevice.GetCurrentClocksThrottleReasons(); ret == nvml.SUCCESS {
		metrics.ThrottleReasons = &ThrottleReasons{
			ThermalHardware: reasons&nvml.ClocksThrottleReasonHwThermalSlowdown != 0,
			ThermalSoftware: reasons&nvml.ClocksThrottleReasonSwThermalSlowdown != 0,
			PowerCap:        reasons&nvml.ClocksThrottleReasonSwPowerCap != 0,
			PowerBrake:      reasons&nvml.ClocksThrottleReasonHwPowerBrakeSlowdown != 0,
		}
	}

	return metrics, nil
}

// SetGpuLockedClocksNVML sets the GPU clock range via NVML. Requires root.
func SetGpuLockedClocksNVML(minMHz, maxMHz uint32) error {
	if !nvmlAvailable {
		return fmt.Errorf("NVML not available")
	}
	if ret := nvmlDevice.SetGpuLockedClocks(minMHz, maxMHz); ret != nvml.SUCCESS {
		return fmt.Errorf("NVML SetGpuLockedClocks: %v", nvml.ErrorString(ret))
	}
	return nil
}

// SetMemoryLockedClocksNVML sets the memory clock range via NVML. Requires root.
func SetMemoryLockedClocksNVML(minMHz, maxMHz uint32) error {
	if !nvmlAvailable {
		return fmt.Errorf("NVML not available")
	}
	if ret := nvmlDevice.SetMemoryLockedClocks(minMHz, maxMHz); ret != nvml.SUCCESS {
		return fmt.Errorf("NVML SetMemoryLockedClocks: %v", nvml.ErrorString(ret))
	}
	return nil
}

// SetPowerManagementLimitNVML sets the power limit in watts via NVML. Requires root.
func SetPowerManagementLimitNVML(watts uint32) error {
	if !nvmlAvailable {
		return fmt.Errorf("NVML not available")
	}
	milliwatts := watts * 1000
	if ret := nvmlDevice.SetPowerManagementLimit(milliwatts); ret != nvml.SUCCESS {
		return fmt.Errorf("NVML SetPowerManagementLimit: %v", nvml.ErrorString(ret))
	}
	return nil
}

// ResetGpuLockedClocksNVML resets GPU clocks to default via NVML. Requires root.
func ResetGpuLockedClocksNVML() error {
	if !nvmlAvailable {
		return fmt.Errorf("NVML not available")
	}
	if ret := nvmlDevice.ResetGpuLockedClocks(); ret != nvml.SUCCESS {
		return fmt.Errorf("NVML ResetGpuLockedClocks: %v", nvml.ErrorString(ret))
	}
	return nil
}

// ResetMemoryLockedClocksNVML resets memory clocks to default via NVML. Requires root.
func ResetMemoryLockedClocksNVML() error {
	if !nvmlAvailable {
		return fmt.Errorf("NVML not available")
	}
	if ret := nvmlDevice.ResetMemoryLockedClocks(); ret != nvml.SUCCESS {
		return fmt.Errorf("NVML ResetMemoryLockedClocks: %v", nvml.ErrorString(ret))
	}
	return nil
}

// NVMLAvailable reports whether NVML was initialized successfully.
func NVMLAvailable() bool {
	return nvmlAvailable
}

func getInfoNVML() (map[string]string, error) {
	info := make(map[string]string)

	if name, ret := nvmlDevice.GetName(); ret == nvml.SUCCESS {
		info["name"] = name
	}

	if driver, ret := nvml.SystemGetDriverVersion(); ret == nvml.SUCCESS {
		info["driver"] = driver
	}

	if mem, ret := nvmlDevice.GetMemoryInfo(); ret == nvml.SUCCESS {
		info["memory"] = fmt.Sprintf("%d MB", mem.Total/(1024*1024))
	}

	if temp, ret := nvmlDevice.GetTemperature(nvml.TEMPERATURE_GPU); ret == nvml.SUCCESS {
		info["temperature"] = fmt.Sprintf("%d°C", temp)
	}

	if power, ret := nvmlDevice.GetPowerUsage(); ret == nvml.SUCCESS {
		info["power_draw"] = fmt.Sprintf("%.1f W", float64(power)/1000.0)
	}

	if limit, ret := nvmlDevice.GetPowerManagementLimit(); ret == nvml.SUCCESS {
		info["power_limit"] = fmt.Sprintf("%d W", limit/1000)
	}

	if minLimit, maxLimit, ret := nvmlDevice.GetPowerManagementLimitConstraints(); ret == nvml.SUCCESS {
		info["power_range"] = fmt.Sprintf("%d–%d W", minLimit/1000, maxLimit/1000)
	}

	if clock, ret := nvmlDevice.GetClockInfo(nvml.CLOCK_GRAPHICS); ret == nvml.SUCCESS {
		info["graphics_clock"] = fmt.Sprintf("%d MHz", clock)
	}

	if clock, ret := nvmlDevice.GetClockInfo(nvml.CLOCK_MEM); ret == nvml.SUCCESS {
		info["memory_clock"] = fmt.Sprintf("%d MHz", clock)
	}

	if fan, ret := nvmlDevice.GetFanSpeed(); ret == nvml.SUCCESS {
		info["fan_speed"] = fmt.Sprintf("%d%%", fan)
	}

	if util, ret := nvmlDevice.GetUtilizationRates(); ret == nvml.SUCCESS {
		info["utilization"] = fmt.Sprintf("%d%%", util.Gpu)
	}

	return info, nil
}
