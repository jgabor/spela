package cpu

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func setupMockSysfs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sysRoot = dir
	t.Cleanup(func() { sysRoot = "" })
	return dir
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGetCPUCount(t *testing.T) {
	count := GetCPUCount()
	if count < 1 {
		t.Errorf("GetCPUCount() = %d, want >= 1", count)
	}
}

func TestGetCurrentGovernor(t *testing.T) {
	root := setupMockSysfs(t)
	writeFile(t, root, "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor", "performance\n")

	gov, err := GetCurrentGovernor()
	if err != nil {
		t.Fatalf("GetCurrentGovernor() error = %v", err)
	}
	if gov != GovernorPerformance {
		t.Errorf("GetCurrentGovernor() = %q, want %q", gov, GovernorPerformance)
	}
}

func TestGetCurrentGovernorMissing(t *testing.T) {
	root := setupMockSysfs(t)
	_ = root // no file written

	_, err := GetCurrentGovernor()
	if err == nil {
		t.Fatal("GetCurrentGovernor() error = nil, want file-not-found error")
	}
}

func TestGetAvailableGovernors(t *testing.T) {
	root := setupMockSysfs(t)
	writeFile(t, root, "/sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors",
		"performance powersave ondemand conservative\n")

	govs, err := GetAvailableGovernors()
	if err != nil {
		t.Fatalf("GetAvailableGovernors() error = %v", err)
	}
	if len(govs) != 4 {
		t.Fatalf("GetAvailableGovernors() returned %d governors, want 4", len(govs))
	}
	if govs[0] != GovernorPerformance {
		t.Errorf("govs[0] = %q, want %q", govs[0], GovernorPerformance)
	}
}

func TestSetGovernorDirect(t *testing.T) {
	root := setupMockSysfs(t)

	// Create sysfs files for each CPU (use actual CPU count)
	cpuCount := GetCPUCount()
	for i := range cpuCount {
		writeFile(t, root, fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_governor", i), "powersave")
	}

	if err := setGovernorDirect(GovernorPerformance); err != nil {
		t.Fatalf("setGovernorDirect() error = %v", err)
	}

	for i := range cpuCount {
		got := readFile(t, root, fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_governor", i))
		if got != string(GovernorPerformance) {
			t.Errorf("cpu%d governor = %q, want %q", i, got, GovernorPerformance)
		}
	}
}

func TestGetSMTStatus(t *testing.T) {
	root := setupMockSysfs(t)

	tests := []struct {
		content string
		want    bool
	}{
		{"1\n", true},
		{"0\n", false},
	}

	for _, tt := range tests {
		writeFile(t, root, "/sys/devices/system/cpu/smt/active", tt.content)

		got, err := GetSMTStatus()
		if err != nil {
			t.Fatalf("GetSMTStatus() error = %v", err)
		}
		if got != tt.want {
			t.Errorf("GetSMTStatus() with %q = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestSetSMTDirect(t *testing.T) {
	root := setupMockSysfs(t)
	writeFile(t, root, "/sys/devices/system/cpu/smt/control", "on")

	if err := setSMTDirect("off"); err != nil {
		t.Fatalf("setSMTDirect() error = %v", err)
	}

	got := readFile(t, root, "/sys/devices/system/cpu/smt/control")
	if got != "off" {
		t.Errorf("SMT control = %q, want %q", got, "off")
	}

	if err := setSMTDirect("on"); err != nil {
		t.Fatalf("setSMTDirect() error = %v", err)
	}

	got = readFile(t, root, "/sys/devices/system/cpu/smt/control")
	if got != "on" {
		t.Errorf("SMT control = %q, want %q", got, "on")
	}
}

func TestGetCPUInfo(t *testing.T) {
	root := setupMockSysfs(t)

	writeFile(t, root, "/proc/cpuinfo", `processor	: 0
vendor_id	: GenuineIntel
model name	: Intel(R) Core(TM) i9-14900K
`)
	writeFile(t, root, "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor", "performance\n")
	writeFile(t, root, "/sys/devices/system/cpu/smt/active", "1\n")

	info, err := GetCPUInfo()
	if err != nil {
		t.Fatalf("GetCPUInfo() error = %v", err)
	}
	if info["model"] != "Intel(R) Core(TM) i9-14900K" {
		t.Errorf("model = %q, want Intel(R) Core(TM) i9-14900K", info["model"])
	}
	if info["governor"] != "performance" {
		t.Errorf("governor = %q, want performance", info["governor"])
	}
	if info["smt"] != "true" {
		t.Errorf("smt = %q, want true", info["smt"])
	}
}

func TestGetCPUMetrics(t *testing.T) {
	root := setupMockSysfs(t)

	// Set up mock frequency files for actual CPU count
	cpuCount := GetCPUCount()
	for i := range cpuCount {
		writeFile(t, root, fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i), "4500000\n")
	}

	writeFile(t, root, "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor", "performance\n")
	writeFile(t, root, "/sys/devices/system/cpu/smt/active", "1\n")
	writeFile(t, root, "/proc/meminfo", "MemTotal:       32768000 kB\nMemAvailable:   16384000 kB\n")
	writeFile(t, root, "/proc/loadavg", "2.50 1.80 1.20 3/500 12345\n")

	metrics, err := GetCPUMetrics()
	if err != nil {
		t.Fatalf("GetCPUMetrics() error = %v", err)
	}
	if len(metrics.Frequencies) != cpuCount {
		t.Errorf("Frequencies count = %d, want %d", len(metrics.Frequencies), cpuCount)
	}
	if metrics.AverageFrequency != 4500 {
		t.Errorf("AverageFrequency = %d, want 4500", metrics.AverageFrequency)
	}
	if metrics.Governor != GovernorPerformance {
		t.Errorf("Governor = %q, want %q", metrics.Governor, GovernorPerformance)
	}
	if !metrics.SMTEnabled {
		t.Error("SMTEnabled = false, want true")
	}
	if metrics.RAMTotalMB != 32000 {
		t.Errorf("RAMTotalMB = %d, want 32000", metrics.RAMTotalMB)
	}
	if metrics.RAMUsedMB != 16000 {
		t.Errorf("RAMUsedMB = %d, want 16000", metrics.RAMUsedMB)
	}
}

func TestLaunchWithAffinity(t *testing.T) {
	cmd := LaunchWithAffinity("0-3", []string{"echo", "hello"})
	args := cmd.Args
	if len(args) < 4 {
		t.Fatalf("args = %v, want at least 4 elements", args)
	}
	if args[0] != "taskset" {
		t.Errorf("args[0] = %q, want taskset", args[0])
	}
	if args[1] != "-c" {
		t.Errorf("args[1] = %q, want -c", args[1])
	}
	if args[2] != "0-3" {
		t.Errorf("args[2] = %q, want 0-3", args[2])
	}
	if args[3] != "echo" {
		t.Errorf("args[3] = %q, want echo", args[3])
	}
}
