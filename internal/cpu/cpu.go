package cpu

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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
	data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
	if err != nil {
		return "", err
	}
	return Governor(strings.TrimSpace(string(data))), nil
}

func GetAvailableGovernors() ([]Governor, error) {
	data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors")
	if err != nil {
		return nil, err
	}

	var governors []Governor
	for _, g := range strings.Fields(string(data)) {
		governors = append(governors, Governor(g))
	}
	return governors, nil
}

func SetGovernor(gov Governor) error {
	cpuCount := GetCPUCount()
	for i := 0; i < cpuCount; i++ {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_governor", i)
		if err := os.WriteFile(path, []byte(gov), 0o644); err != nil {
			return fmt.Errorf("failed to set governor for cpu%d: %w", i, err)
		}
	}
	return nil
}

func GetSMTStatus() (bool, error) {
	data, err := os.ReadFile("/sys/devices/system/cpu/smt/active")
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
	return os.WriteFile("/sys/devices/system/cpu/smt/control", []byte(value), 0o644)
}

func LaunchWithAffinity(affinity string, args []string) *exec.Cmd {
	tasksetArgs := append([]string{"-c", affinity}, args...)
	return exec.Command("taskset", tasksetArgs...)
}

func GetCPUInfo() (map[string]string, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}

	info := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info["model"] = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	info["cores"] = fmt.Sprintf("%d", GetCPUCount())

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

func SCXStart(scheduler string) error {
	cmd := exec.Command("systemctl", "start", "scx.service")
	return cmd.Run()
}

func SCXStop() error {
	cmd := exec.Command("systemctl", "stop", "scx.service")
	return cmd.Run()
}

func SCXStatus() (bool, error) {
	cmd := exec.Command("systemctl", "is-active", "scx.service")
	err := cmd.Run()
	return err == nil, nil
}

func GetSchedulers() ([]string, error) {
	entries, err := filepath.Glob("/usr/bin/scx_*")
	if err != nil {
		return nil, err
	}

	var schedulers []string
	for _, e := range entries {
		name := filepath.Base(e)
		if strings.HasPrefix(name, "scx_") {
			schedulers = append(schedulers, strings.TrimPrefix(name, "scx_"))
		}
	}
	return schedulers, nil
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
	for i := 0; i < cpuCount; i++ {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i)
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
	if memData, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(memData), "\n")
		for _, line := range lines {
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
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0
	}

	// Parse first line: cpu user nice system idle iowait irq softirq
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0
	}

	var user, nice, system, idle int
	_, _ = fmt.Sscanf(fields[1], "%d", &user)
	_, _ = fmt.Sscanf(fields[2], "%d", &nice)
	_, _ = fmt.Sscanf(fields[3], "%d", &system)
	_, _ = fmt.Sscanf(fields[4], "%d", &idle)

	total := user + nice + system + idle
	if total == 0 {
		return 0
	}

	// This is cumulative since boot, so not very useful for instant reading
	// Use load average instead for a better approximation
	if loadData, err := os.ReadFile("/proc/loadavg"); err == nil {
		var load1 float64
		_, _ = fmt.Sscanf(string(loadData), "%f", &load1)
		// Normalize by CPU count for percentage approximation
		cpuCount := float64(GetCPUCount())
		util := (load1 / cpuCount) * 100
		if util > 100 {
			util = 100
		}
		return util
	}

	return 0
}
