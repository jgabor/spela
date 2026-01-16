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
	GovernorPerformance Governor = "performance"
	GovernorPowersave   Governor = "powersave"
	GovernorOndemand    Governor = "ondemand"
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
		if err := os.WriteFile(path, []byte(gov), 0644); err != nil {
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
	return os.WriteFile("/sys/devices/system/cpu/smt/control", []byte(value), 0644)
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
