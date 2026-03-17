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

	return metrics, nil
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
		info["power"] = fmt.Sprintf("%.1f W", float64(power)/1000.0)
	}

	return info, nil
}
