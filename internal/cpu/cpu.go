package cpu

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/jgabor/spela/internal/privilege"
)

// sysRoot overrides the filesystem root for sysfs/proc paths.
// Empty uses real paths. Set in tests to use a temporary directory.
var sysRoot string

func sysPath(path string) string {
	if sysRoot != "" {
		return filepath.Join(sysRoot, path)
	}
	return path
}

type Governor string

const (
	GovernorPerformance  Governor = "performance"
	GovernorPowersave    Governor = "powersave"
	GovernorOndemand     Governor = "ondemand"
	GovernorConservative Governor = "conservative"
)

func GetCPUCount() int {
	return runtime.NumCPU()
}

func GetCurrentGovernor() (Governor, error) {
	data, err := os.ReadFile(sysPath("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"))
	if err != nil {
		return "", err
	}
	return Governor(strings.TrimSpace(string(data))), nil
}

func GetAvailableGovernors() ([]Governor, error) {
	data, err := os.ReadFile(sysPath("/sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors"))
	if err != nil {
		return nil, err
	}

	var governors []Governor
	for g := range strings.FieldsSeq(string(data)) {
		governors = append(governors, Governor(g))
	}
	return governors, nil
}

func SetGovernor(gov Governor) error {
	if privilege.IsRoot() {
		return setGovernorDirect(gov)
	}
	cpuCount := GetCPUCount()
	for i := range cpuCount {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_governor", i)
		_, err := privilege.ExecWithInput(string(gov), "tee", path)
		if err != nil {
			return fmt.Errorf("failed to set governor for cpu%d: %w", i, err)
		}
	}
	return nil
}

func setGovernorDirect(gov Governor) error {
	cpuCount := GetCPUCount()
	for i := range cpuCount {
		path := sysPath(fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_governor", i))
		if err := os.WriteFile(path, []byte(string(gov)), 0o644); err != nil {
			return fmt.Errorf("failed to set governor for cpu%d: %w", i, err)
		}
	}
	return nil
}

func GetSMTStatus() (bool, error) {
	data, err := os.ReadFile(sysPath("/sys/devices/system/cpu/smt/active"))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

func SetSMT(enabled bool) error {
	value := "off"
	if enabled {
		value = "on"
	}
	if privilege.IsRoot() {
		return setSMTDirect(value)
	}
	_, err := privilege.ExecWithInput(value, "tee", sysPath("/sys/devices/system/cpu/smt/control"))
	if err != nil {
		return fmt.Errorf("failed to set SMT: %w", err)
	}
	return nil
}

func setSMTDirect(value string) error {
	path := sysPath("/sys/devices/system/cpu/smt/control")
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		return fmt.Errorf("failed to set SMT: %w", err)
	}
	return nil
}

func LaunchWithAffinity(affinity string, args []string) *exec.Cmd {
	tasksetArgs := append([]string{"-c", affinity}, args...)
	return exec.Command("taskset", tasksetArgs...)
}

func GetCPUInfo() (map[string]string, error) {
	data, err := os.ReadFile(sysPath("/proc/cpuinfo"))
	if err != nil {
		return nil, err
	}

	info := make(map[string]string)
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info["model"] = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	info["cores"] = strconv.Itoa(GetCPUCount())

	gov, err := GetCurrentGovernor()
	if err == nil {
		info["governor"] = string(gov)
	}

	smt, err := GetSMTStatus()
	if err == nil {
		info["smt"] = fmt.Sprintf("%v", smt)
	}

	return info, nil
}

func SCXIsAvailable() bool {
	_, err := exec.LookPath("scx_loader")
	return err == nil
}

func SCXStatus() (bool, error) {
	cmd := exec.Command("systemctl", "is-active", "scx.service")
	err := cmd.Run()
	return err == nil, nil
}

type CPUMetrics struct {
	Frequencies      []int
	AverageFrequency int
	Utilization      float64
	Governor         Governor
	SMTEnabled       bool
	RAMUsedMB        int
	RAMTotalMB       int
}

func GetCPUMetrics() (*CPUMetrics, error) {
	metrics := &CPUMetrics{}
	cpuCount := GetCPUCount()

	var total int
	for i := range cpuCount {
		path := sysPath(fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		freq := 0
		_, _ = fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &freq)
		metrics.Frequencies = append(metrics.Frequencies, freq/1000)
		total += freq / 1000
	}

	if len(metrics.Frequencies) > 0 {
		metrics.AverageFrequency = total / len(metrics.Frequencies)
	}

	metrics.Governor, _ = GetCurrentGovernor()
	metrics.SMTEnabled, _ = GetSMTStatus()

	// Get RAM info from /proc/meminfo
	if memData, err := os.ReadFile(sysPath("/proc/meminfo")); err == nil {
		for line := range strings.SplitSeq(string(memData), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				_, _ = fmt.Sscanf(line, "MemTotal: %d kB", &metrics.RAMTotalMB)
				metrics.RAMTotalMB /= 1024
			} else if strings.HasPrefix(line, "MemAvailable:") {
				var available int
				_, _ = fmt.Sscanf(line, "MemAvailable: %d kB", &available)
				metrics.RAMUsedMB = metrics.RAMTotalMB - (available / 1024)
			}
		}
	}

	// Get CPU utilization from /proc/stat (simplified: use instant calculation)
	metrics.Utilization = getCPUUtilization()

	return metrics, nil
}

func getCPUUtilization() float64 {
	data, err := os.ReadFile(sysPath("/proc/loadavg"))
	if err != nil {
		return 0
	}

	var load1 float64
	_, _ = fmt.Sscanf(string(data), "%f", &load1)

	// Normalize by CPU count for percentage approximation
	cpuCount := float64(GetCPUCount())
	util := (load1 / cpuCount) * 100
	if util > 100 {
		util = 100
	}
	return util
}
