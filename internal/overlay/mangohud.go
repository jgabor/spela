package overlay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jgabor/spela/internal/xdg"
)

type MangoHudConfig struct {
	Enabled        bool
	Position       string
	FontSize       int
	ShowFPS        bool
	ShowFrametime  bool
	ShowCPU        bool
	ShowGPU        bool
	ShowRAM        bool
	ShowVRAM       bool
	ShowTemp       bool
	ShowPower      bool
	ShowClock      bool
	Frametime      bool
	Histogram      bool
	ToggleKey      string
	LoggingEnabled bool
	LogDuration    int
}

func DefaultConfig() *MangoHudConfig {
	return &MangoHudConfig{
		Enabled:       true,
		Position:      "top-left",
		FontSize:      24,
		ShowFPS:       true,
		ShowFrametime: true,
		ShowCPU:       true,
		ShowGPU:       true,
		ShowRAM:       false,
		ShowVRAM:      true,
		ShowTemp:      true,
		ShowPower:     true,
		ShowClock:     true,
		Frametime:     true,
		Histogram:     false,
		ToggleKey:     "F12",
	}
}

func IsInstalled() bool {
	_, err := exec.LookPath("mangohud")
	return err == nil
}

func GetVersion() (string, error) {
	cmd := exec.Command("mangohud", "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *MangoHudConfig) ToConfigString() string {
	var lines []string

	if c.Position != "" {
		lines = append(lines, fmt.Sprintf("position=%s", c.Position))
	}

	if c.FontSize > 0 {
		lines = append(lines, fmt.Sprintf("font_size=%d", c.FontSize))
	}

	if c.ShowFPS {
		lines = append(lines, "fps")
	}

	if c.ShowFrametime {
		lines = append(lines, "frame_timing")
	}

	if c.ShowCPU {
		lines = append(lines, "cpu_stats")
		lines = append(lines, "cpu_temp")
		lines = append(lines, "cpu_power")
	}

	if c.ShowGPU {
		lines = append(lines, "gpu_stats")
		lines = append(lines, "gpu_temp")
		lines = append(lines, "gpu_power")
		lines = append(lines, "gpu_mem_clock")
		lines = append(lines, "gpu_core_clock")
	}

	if c.ShowVRAM {
		lines = append(lines, "vram")
	}

	if c.ShowRAM {
		lines = append(lines, "ram")
	}

	if c.Frametime {
		lines = append(lines, "frametime")
	}

	if c.Histogram {
		lines = append(lines, "histogram")
	}

	if c.ToggleKey != "" {
		lines = append(lines, fmt.Sprintf("toggle_hud=%s", c.ToggleKey))
	}

	if c.LoggingEnabled {
		lines = append(lines, "output_folder=~/.local/share/spela/logs")
		if c.LogDuration > 0 {
			lines = append(lines, fmt.Sprintf("log_duration=%d", c.LogDuration))
		}
	}

	return strings.Join(lines, "\n")
}

func (c *MangoHudConfig) WriteConfig(appID uint64) (string, error) {
	configDir := xdg.ConfigPath("mangohud")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, fmt.Sprintf("%d.conf", appID))
	content := c.ToConfigString()

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return "", err
	}

	return configPath, nil
}

func GetEnvironment(config *MangoHudConfig, appID uint64) (map[string]string, error) {
	env := make(map[string]string)

	if !config.Enabled {
		return env, nil
	}

	if !IsInstalled() {
		return env, nil
	}

	env["MANGOHUD"] = "1"

	if appID > 0 {
		configPath, err := config.WriteConfig(appID)
		if err == nil {
			env["MANGOHUD_CONFIGFILE"] = configPath
		}
	} else {
		env["MANGOHUD_CONFIG"] = config.ToConfigString()
	}

	return env, nil
}
