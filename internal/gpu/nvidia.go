package gpu

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type NVIDIASettings struct {
	ThreadedOptimization bool
	DigitalVibrance      int
	PowerMizerMode       int
}

type NVIDIASMISettings struct {
	TargetTemp        int
	GraphicsClockOffset int
	MemoryClockOffset  int
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
	_, err := runNvidiaSMI("-lgc", fmt.Sprintf("%d,%d", 0, 2100+offset))
	return err
}

func SetMemoryClockOffset(offset int) error {
	_, err := runNvidiaSMI("-lmc", fmt.Sprintf("%d", offset))
	return err
}

func SetPowerLimit(watts int) error {
	_, err := runNvidiaSMI("-pl", fmt.Sprintf("%d", watts))
	return err
}

func ResetClocks() error {
	runNvidiaSMI("-rgc")
	runNvidiaSMI("-rmc")
	return nil
}

func GetGPUInfo() (map[string]string, error) {
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

func parseNvidiaSettingsValue(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "):") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(strings.TrimSuffix(parts[len(parts)-1], "."))
			}
		}
	}
	return ""
}
