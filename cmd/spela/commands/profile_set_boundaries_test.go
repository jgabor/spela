package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCPUAndGPUProfileSetDefaultAndValidationContracts(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	seedGame(t, "Cyberpunk 2077", 1091500)

	cpuCommand := &cobra.Command{}
	cpuSetGovernor, cpuSetSMT = "", ""
	if output := captureStdout(t, func() {
		if err := runCPUSet(cpuCommand, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "No changes specified") {
		t.Fatalf("CPU no-change output = %q", output)
	}
	for _, value := range []string{"off", "false", "default"} {
		cpuSetGovernor, cpuSetSMT = "default", value
		if err := runCPUSet(cpuCommand, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatalf("CPU SMT %s: %v", value, err)
		}
	}
	cpuSetGovernor, cpuSetSMT = "", "invalid"
	if err := runCPUSet(cpuCommand, []string{"Cyberpunk 2077"}); err == nil || !strings.Contains(err.Error(), "invalid SMT") {
		t.Fatalf("CPU invalid SMT error = %v", err)
	}
	if err := runCPUSet(cpuCommand, []string{"missing"}); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("CPU missing game error = %v", err)
	}

	gpuCommand := &cobra.Command{}
	gpuCommand.Flags().Int("clock-offset", 0, "")
	gpuCommand.Flags().Int("memory-offset", 0, "")
	gpuCommand.Flags().Int("power-limit", 0, "")
	gpuCommand.Flags().Int("fan-speed", 0, "")
	gpuSetPowerMizer, gpuSetShaderCache, gpuSetCachePath, gpuSetThreadedOpt = "", "", "", ""
	if output := captureStdout(t, func() {
		if err := runGPUSet(gpuCommand, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "No changes specified") {
		t.Fatalf("GPU no-change output = %q", output)
	}
	gpuSetPowerMizer, gpuSetShaderCache, gpuSetCachePath, gpuSetThreadedOpt = "default", "false", "default", "false"
	if err := runGPUSet(gpuCommand, []string{"Cyberpunk 2077"}); err != nil {
		t.Fatal(err)
	}
	for name, configure := range map[string]func(){
		"shader cache":          func() { gpuSetShaderCache, gpuSetThreadedOpt = "invalid", "" },
		"threaded optimization": func() { gpuSetShaderCache, gpuSetThreadedOpt = "", "invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			gpuSetPowerMizer, gpuSetCachePath = "", ""
			configure()
			if err := runGPUSet(gpuCommand, []string{"Cyberpunk 2077"}); err == nil || !strings.Contains(err.Error(), "invalid bool") {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
}

func TestProtonSetNoChangeValidationAndMissingProfileContracts(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	seedGame(t, "Cyberpunk 2077", 1091500)
	command := &cobra.Command{}
	protonSetHDR, protonSetWayland, protonSetNGXUpdater, protonSetVKD3DHeap = "", "", "", ""
	if output := captureStdout(t, func() {
		if err := runProtonSet(command, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "No changes specified") {
		t.Fatalf("Proton no-change output = %q", output)
	}
	for name, configure := range map[string]func(){
		"HDR":         func() { protonSetHDR = "invalid" },
		"Wayland":     func() { protonSetWayland = "invalid" },
		"NGX updater": func() { protonSetNGXUpdater = "invalid" },
		"VKD3D heap":  func() { protonSetVKD3DHeap = "invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			protonSetHDR, protonSetWayland, protonSetNGXUpdater, protonSetVKD3DHeap = "", "", "", ""
			configure()
			if err := runProtonSet(command, []string{"Cyberpunk 2077"}); err == nil || !strings.Contains(err.Error(), "invalid bool") {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
	protonSetHDR, protonSetWayland, protonSetNGXUpdater, protonSetVKD3DHeap = "", "", "", ""
	if output := captureStdout(t, func() {
		if err := runProtonShow(command, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "No profile") {
		t.Fatalf("missing Proton profile output = %q", output)
	}
	if output := captureStdout(t, func() {
		if err := runProtonReset(command, []string{"Cyberpunk 2077", "hdr"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "already inherited") {
		t.Fatalf("missing Proton reset output = %q", output)
	}
}

func TestOverlaySetNoChangeAndBooleanValidationContracts(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	seedGame(t, "Cyberpunk 2077", 1091500)
	command := &cobra.Command{}
	clear := func() {
		overlaySetEnabled, overlaySetPosition, overlaySetShowFPS, overlaySetShowFrametime = "", "", "", ""
		overlaySetShowCPU, overlaySetShowGPU, overlaySetShowVRAM, overlaySetToggleKey = "", "", "", ""
	}
	clear()
	if output := captureStdout(t, func() {
		if err := runOverlaySet(command, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "No changes specified") {
		t.Fatalf("overlay no-change output = %q", output)
	}
	tests := []struct {
		name string
		set  func()
	}{
		{"enabled", func() { overlaySetEnabled = "invalid" }},
		{"FPS", func() { overlaySetShowFPS = "invalid" }},
		{"frametime", func() { overlaySetShowFrametime = "invalid" }},
		{"CPU", func() { overlaySetShowCPU = "invalid" }},
		{"GPU", func() { overlaySetShowGPU = "invalid" }},
		{"VRAM", func() { overlaySetShowVRAM = "invalid" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clear()
			test.set()
			if err := runOverlaySet(command, []string{"Cyberpunk 2077"}); err == nil || !strings.Contains(err.Error(), "invalid bool") {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
	clear()
	if output := captureStdout(t, func() {
		if err := runOverlayReset(command, []string{"Cyberpunk 2077", "enabled"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "already inherited") {
		t.Fatalf("missing overlay reset output = %q", output)
	}
}
