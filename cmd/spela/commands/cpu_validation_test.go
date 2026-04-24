package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/profile"
)

func withGovernorValidator(t *testing.T, fn func(cpu.Governor) error) {
	t.Helper()
	original := validateCPUGovernorAvailable
	validateCPUGovernorAvailable = fn
	t.Cleanup(func() { validateCPUGovernorAvailable = original })
}

func TestRunCPUSet_GovernorAvailablePersists(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	withGovernorValidator(t, func(gov cpu.Governor) error { return nil })

	cpuSetGovernor = "performance"
	cpuSetSMT = ""
	t.Cleanup(func() { cpuSetGovernor = "" })

	if err := runCPUSet(cpuSetCmd, []string{"1091500"}); err != nil {
		t.Fatalf("runCPUSet: %v", err)
	}
	p, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("profile.Load: %v", err)
	}
	if p.CPU.Governor != "performance" {
		t.Fatalf("CPU governor = %q, want performance", p.CPU.Governor)
	}
}

func TestRunCPUSet_GovernorUnavailableRejectsBeforeSave(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	withGovernorValidator(t, func(gov cpu.Governor) error {
		return fmt.Errorf("CPU governor %q is not available", gov)
	})

	cpuSetGovernor = "performance"
	cpuSetSMT = ""
	t.Cleanup(func() { cpuSetGovernor = "" })

	err := runCPUSet(cpuSetCmd, []string{"1091500"})
	if err == nil {
		t.Fatal("runCPUSet: expected unavailable governor error, got nil")
	}
	if p, _ := profile.Load(1091500); p != nil {
		t.Fatal("profile should not be saved when governor validation fails")
	}
}

func TestApplyProfile_GovernorUnavailableRejectsBeforePrivilegedChanges(t *testing.T) {
	withGovernorValidator(t, func(gov cpu.Governor) error {
		return fmt.Errorf("CPU governor %q is not available", gov)
	})

	applyCPUGovernor = "performance"
	applyGPUClockOffset = 100
	t.Cleanup(func() {
		applyCPUGovernor = ""
		applyGPUClockOffset = 0
	})

	cmd := &cobra.Command{}
	cmd.Flags().String("cpu-governor", "", "")
	cmd.Flags().Int("gpu-clock-offset", 0, "")
	if err := cmd.Flags().Set("cpu-governor", "performance"); err != nil {
		t.Fatalf("set cpu-governor: %v", err)
	}
	if err := cmd.Flags().Set("gpu-clock-offset", "100"); err != nil {
		t.Fatalf("set gpu-clock-offset: %v", err)
	}

	err := applySettings(cmd)
	if err == nil {
		t.Fatal("applySettings: expected governor validation error, got nil")
	}
	if !strings.Contains(err.Error(), "cpu-governor") {
		t.Fatalf("applySettings error = %q, want cpu-governor context", err.Error())
	}
}
