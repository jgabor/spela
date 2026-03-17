package gpu

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jgabor/spela/internal/privilege"
)

type NVIDIASettings struct {
	ThreadedOptimization bool
	DigitalVibrance      int
	PowerMizerMode       int
}

type NVIDIASMISettings struct {
	TargetTemp          int
	GraphicsClockOffset int
	MemoryClockOffset   int
}

func GetNVIDIASettings() (*NVIDIASettings, error) {
	settings := &NVIDIASettings{}

	out, err := runNvidiaSettings("-q", "[gpu:0]/GPUPowerMizerMode")
	if err == nil {
		if val := parseNvidiaSettingsValue(out); val != "" {
			settings.PowerMizerMode, _ = strconv.Atoi(val)
		}
	}

	out, err = runNvidiaSettings("-q", "[gpu:0]/DigitalVibrance")
	if err == nil {
		if val := parseNvidiaSettingsValue(out); val != "" {
			settings.DigitalVibrance, _ = strconv.Atoi(val)
		}
	}

	return settings, nil
}

func SetPowerMizerMode(mode int) error {
	_, err := runNvidiaSettings("-a", fmt.Sprintf("[gpu:0]/GPUPowerMizerMode=%d", mode))
	return err
}

func SetDigitalVibrance(level int) error {
	_, err := runNvidiaSettings("-a", fmt.Sprintf("[gpu:0]/DigitalVibrance=%d", level))
	return err
}

func SetGraphicsClockOffset(offset int) error {
	return runNvidiaSMIElevated("-lgc", fmt.Sprintf("%d,%d", 0, 2100+offset))
}

func SetMemoryClockOffset(offset int) error {
	return runNvidiaSMIElevated("-lmc", strconv.Itoa(offset))
}

func SetPowerLimit(watts int) error {
	return runNvidiaSMIElevated("-pl", strconv.Itoa(watts))
}

func ResetClocks() error {
	if err := runNvidiaSMIElevated("-rgc"); err != nil {
		return err
	}
	return runNvidiaSMIElevated("-rmc")
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

func runNvidiaSettings(args ...string) (string, error) {
	cmd := exec.Command("nvidia-settings", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
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

func parseNvidiaSettingsValue(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "):") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(strings.TrimSuffix(parts[len(parts)-1], "."))
			}
		}
	}
	return ""
}

type GPUGeneration int

const (
	GPUGenerationUnknown GPUGeneration = iota
	GPUGenerationTuring
	GPUGenerationAmpere
	GPUGenerationAdaLovelace
	GPUGenerationBlackwell
)

func (g GPUGeneration) SupportsFP8() bool {
	return g >= GPUGenerationAdaLovelace
}

func (g GPUGeneration) String() string {
	switch g {
	case GPUGenerationTuring:
		return "Turing (RTX 20)"
	case GPUGenerationAmpere:
		return "Ampere (RTX 30)"
	case GPUGenerationAdaLovelace:
		return "Ada Lovelace (RTX 40)"
	case GPUGenerationBlackwell:
		return "Blackwell (RTX 50)"
	default:
		return "Unknown"
	}
}

func GetGPUGeneration() (GPUGeneration, error) {
	info, err := GetGPUInfo()
	if err != nil {
		return GPUGenerationUnknown, err
	}
	return parseGPUGeneration(info["name"]), nil
}

func parseGPUGeneration(name string) GPUGeneration {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "rtx 20"), strings.Contains(name, "rtx20"):
		return GPUGenerationTuring
	case strings.Contains(name, "rtx 30"), strings.Contains(name, "rtx30"):
		return GPUGenerationAmpere
	case strings.Contains(name, "rtx 40"), strings.Contains(name, "rtx40"):
		return GPUGenerationAdaLovelace
	case strings.Contains(name, "rtx 50"), strings.Contains(name, "rtx50"):
		return GPUGenerationBlackwell
	default:
		return GPUGenerationUnknown
	}
}
