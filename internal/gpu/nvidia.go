package gpu

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	"github.com/jgabor/spela/internal/privilege"
)

func LockGraphicsClocks(offset int) error {
	maxMHz := 2100 + offset
	if nvmlAvailable && privilege.IsRoot() {
		return SetGpuLockedClocksNVML(0, uint32(maxMHz))
	}
	return runNvidiaSMIElevated("-lgc", fmt.Sprintf("%d,%d", 0, maxMHz))
}

func LockMemoryClocks(offset int) error {
	if nvmlAvailable && privilege.IsRoot() {
		return SetMemoryLockedClocksNVML(0, uint32(offset))
	}
	return runNvidiaSMIElevated("-lmc", fmt.Sprintf("%d,%d", 0, offset))
}

// GetCurrentPowerLimit returns the GPU power limit in watts via metrics.
func GetCurrentPowerLimit() (int, error) {
	m, err := GetGPUMetrics()
	if err != nil {
		return 0, err
	}
	return int(m.PowerLimit), nil
}

func SetPowerLimit(watts int) error {
	if nvmlAvailable && privilege.IsRoot() {
		return SetPowerManagementLimitNVML(uint32(watts))
	}
	return runNvidiaSMIElevated("-pl", strconv.Itoa(watts))
}

func ResetClocks() error {
	if nvmlAvailable && privilege.IsRoot() {
		if err := ResetGpuLockedClocksNVML(); err != nil {
			return err
		}
		return ResetMemoryLockedClocksNVML()
	}
	if err := runNvidiaSMIElevated("-rgc"); err != nil {
		return err
	}
	return runNvidiaSMIElevated("-rmc")
}

// SetFanSpeed sets the GPU fan speed as a percentage (0-100). Requires root.
// Sets all fans to the same speed.
func SetFanSpeed(speed int) error {
	if !nvmlAvailable || !privilege.IsRoot() {
		return fmt.Errorf("NVML fan speed control requires root and NVML")
	}
	numFans, ret := nvmlDevice.GetNumFans()
	if ret != nvml.SUCCESS || numFans == 0 {
		numFans = 1
	}
	for i := 0; i < numFans; i++ {
		if err := SetFanSpeedNVML(i, speed); err != nil {
			return err
		}
	}
	return nil
}

// ResetFanSpeed resets all GPU fans to default (automatic) control. Requires root.
func ResetFanSpeed() error {
	if !nvmlAvailable || !privilege.IsRoot() {
		return fmt.Errorf("NVML fan speed control requires root and NVML")
	}
	numFans, ret := nvmlDevice.GetNumFans()
	if ret != nvml.SUCCESS || numFans == 0 {
		numFans = 1
	}
	for i := 0; i < numFans; i++ {
		if err := ResetFanSpeedNVML(i); err != nil {
			return err
		}
	}
	return nil
}

// Init initializes the GPU subsystem. It tries NVML first for fast metric
// access, falling back to nvidia-smi if NVML is unavailable.
func Init() {
	initNVML()
}

// Shutdown releases GPU subsystem resources.
func Shutdown() {
	if nvmlAvailable {
		shutdownNVML()
	}
}

func GetGPUInfo() (map[string]string, error) {
	if nvmlAvailable {
		return getInfoNVML()
	}
	return getInfoSMI()
}

// DriverVersionString returns the raw NVIDIA driver version string (e.g.
// "580.94.16"), obtained via NVML when available or nvidia-smi otherwise.
//
// Returns ("", nil) when no NVIDIA driver/GPU is detected — callers that
// need the distinction between "non-NVIDIA" and "probe failed" should
// branch on the empty return plus nil error.
//
// Intended for feature-gating code paths that must compare the driver
// against a minimum required version (e.g. vkd3d-proton descriptor_heap
// support). The returned string may be two- or three-component and may
// contain trailing whitespace; callers should pass it through
// proton.ParseDriverVersion for structured handling.
func DriverVersionString() (string, error) {
	if nvmlAvailable {
		if driver, ret := nvml.SystemGetDriverVersion(); ret == nvml.SUCCESS {
			return driver, nil
		}
		// NVML is up but the driver query failed — fall through to smi.
	}
	out, err := runNvidiaSMI("--query-gpu=driver_version", "--format=csv,noheader,nounits")
	if err != nil {
		// nvidia-smi not present or failed — treat as "no NVIDIA driver".
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func getInfoSMI() (map[string]string, error) {
	out, err := runNvidiaSMI("--query-gpu=name,driver_version,memory.total,temperature.gpu,power.draw", "--format=csv,noheader,nounits")
	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(out), ", ")
	if len(parts) < 5 {
		return nil, fmt.Errorf("unexpected nvidia-smi output")
	}

	return map[string]string{
		"name":        parts[0],
		"driver":      parts[1],
		"memory":      parts[2] + " MB",
		"temperature": parts[3] + "°C",
		"power":       parts[4] + " W",
	}, nil
}

// ThrottleReasons reports why the GPU is currently throttling.
// Only populated when NVML is available.
type ThrottleReasons struct {
	ThermalHardware bool // HW thermal limit reached
	ThermalSoftware bool // SW thermal cap triggered
	PowerCap        bool // User-set power limit is constraining clocks
	PowerBrake      bool // HW power brake (emergency)
}

// Throttling returns true if any throttle reason is active.
func (r *ThrottleReasons) Throttling() bool {
	return r != nil && (r.ThermalHardware || r.ThermalSoftware || r.PowerCap || r.PowerBrake)
}

type GPUMetrics struct {
	Temperature     int
	PowerDraw       float64
	PowerLimit      float64
	Utilization     int
	MemoryUsed      int
	MemoryTotal     int
	GraphicsClock   int
	MemoryClock     int
	FanSpeed        int
	ThrottleReasons *ThrottleReasons // nil when NVML unavailable
}

func GetGPUMetrics() (*GPUMetrics, error) {
	if nvmlAvailable {
		return getMetricsNVML()
	}
	return getMetricsSMI()
}

func getMetricsSMI() (*GPUMetrics, error) {
	out, err := runNvidiaSMI(
		"--query-gpu=temperature.gpu,power.draw,power.limit,utilization.gpu,memory.used,memory.total,clocks.gr,clocks.mem",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(out), ", ")
	if len(parts) < 8 {
		return nil, fmt.Errorf("unexpected nvidia-smi output: got %d fields", len(parts))
	}

	metrics := &GPUMetrics{}
	metrics.Temperature, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	metrics.PowerDraw, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	metrics.PowerLimit, _ = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	metrics.Utilization, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
	metrics.MemoryUsed, _ = strconv.Atoi(strings.TrimSpace(parts[4]))
	metrics.MemoryTotal, _ = strconv.Atoi(strings.TrimSpace(parts[5]))
	metrics.GraphicsClock, _ = strconv.Atoi(strings.TrimSpace(parts[6]))
	metrics.MemoryClock, _ = strconv.Atoi(strings.TrimSpace(parts[7]))

	return metrics, nil
}

func runNvidiaSMI(args ...string) (string, error) {
	cmd := exec.Command("nvidia-smi", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func runNvidiaSMIElevated(args ...string) error {
	_, err := privilege.Exec("nvidia-smi", args...)
	return err
}

type GPUGeneration int

const (
	GPUGenerationUnknown GPUGeneration = iota
	GPUGenerationTuring
	GPUGenerationAmpere
	GPUGenerationAdaLovelace
	GPUGenerationBlackwell
)
