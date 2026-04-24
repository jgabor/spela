package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/privilege"
)

var (
	applyGPUClockOffset  int
	applyGPUMemoryOffset int
	applyGPUPowerLimit   int
	applyGPUFanSpeed     int
	applyCPUGovernor     string
	applyCPUSMT          string
	applyReset           bool
)

// ApplyProfileCmd is a hidden subcommand invoked via pkexec to apply
// privileged GPU/CPU settings in a single elevated process.
var ApplyProfileCmd = &cobra.Command{
	Use:    "apply-profile",
	Short:  "Apply privileged GPU/CPU profile settings",
	Hidden: true,
	RunE:   runApplyProfile,
}

func init() {
	ApplyProfileCmd.Flags().IntVar(&applyGPUClockOffset, "gpu-clock-offset", 0, "GPU graphics clock offset (MHz)")
	ApplyProfileCmd.Flags().IntVar(&applyGPUMemoryOffset, "gpu-memory-offset", 0, "GPU memory clock (MHz)")
	ApplyProfileCmd.Flags().IntVar(&applyGPUPowerLimit, "gpu-power-limit", 0, "GPU power limit (watts)")
	ApplyProfileCmd.Flags().IntVar(&applyGPUFanSpeed, "gpu-fan-speed", 0, "GPU fan speed percentage (0=auto)")
	ApplyProfileCmd.Flags().StringVar(&applyCPUGovernor, "cpu-governor", "", "CPU frequency governor")
	ApplyProfileCmd.Flags().StringVar(&applyCPUSMT, "cpu-smt", "", "CPU SMT control (on/off)")
	ApplyProfileCmd.Flags().BoolVar(&applyReset, "reset", false, "Reset GPU clocks to default")
}

func runApplyProfile(cmd *cobra.Command, args []string) error {
	if !privilege.IsRoot() {
		return fmt.Errorf("apply-profile must be run with root privileges")
	}

	if applyReset {
		return applyResetSettings(cmd)
	}

	return applySettings(cmd)
}

func applySettings(cmd *cobra.Command) error {
	if err := validateApplyInputs(cmd); err != nil {
		return err
	}

	if cmd.Flags().Changed("gpu-clock-offset") {
		if err := gpu.LockGraphicsClocks(applyGPUClockOffset); err != nil {
			return fmt.Errorf("set GPU clock offset: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-memory-offset") {
		if err := gpu.LockMemoryClocks(applyGPUMemoryOffset); err != nil {
			return fmt.Errorf("set GPU memory offset: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-power-limit") {
		if err := gpu.SetPowerLimit(applyGPUPowerLimit); err != nil {
			return fmt.Errorf("set GPU power limit: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-fan-speed") && applyGPUFanSpeed > 0 {
		if err := gpu.SetFanSpeed(applyGPUFanSpeed); err != nil {
			return fmt.Errorf("set GPU fan speed: %w", err)
		}
	}

	if applyCPUGovernor != "" {
		if err := cpu.SetGovernor(cpu.Governor(applyCPUGovernor)); err != nil {
			return fmt.Errorf("set CPU governor: %w", err)
		}
	}

	if applyCPUSMT == "on" || applyCPUSMT == "off" {
		if err := cpu.SetSMT(applyCPUSMT == "on"); err != nil {
			return fmt.Errorf("set CPU SMT: %w", err)
		}
	}

	return nil
}

func applyResetSettings(cmd *cobra.Command) error {
	if err := validateApplyInputs(cmd); err != nil {
		return err
	}

	if err := gpu.ResetClocks(); err != nil {
		return fmt.Errorf("reset GPU clocks: %w", err)
	}

	if cmd.Flags().Changed("gpu-power-limit") && applyGPUPowerLimit > 0 {
		if err := gpu.SetPowerLimit(applyGPUPowerLimit); err != nil {
			return fmt.Errorf("restore GPU power limit: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-fan-speed") {
		if applyGPUFanSpeed == 0 {
			if err := gpu.ResetFanSpeed(); err != nil {
				return fmt.Errorf("reset GPU fan speed: %w", err)
			}
		} else {
			if err := gpu.SetFanSpeed(applyGPUFanSpeed); err != nil {
				return fmt.Errorf("restore GPU fan speed: %w", err)
			}
		}
	}

	if applyCPUGovernor != "" {
		if err := cpu.SetGovernor(cpu.Governor(applyCPUGovernor)); err != nil {
			return fmt.Errorf("restore CPU governor: %w", err)
		}
	}

	if applyCPUSMT == "on" || applyCPUSMT == "off" {
		if err := cpu.SetSMT(applyCPUSMT == "on"); err != nil {
			return fmt.Errorf("restore CPU SMT: %w", err)
		}
	}

	return nil
}

func validateApplyInputs(cmd *cobra.Command) error {
	if applyCPUGovernor != "" {
		if err := validateCPUGovernorFlag("cpu-governor", applyCPUGovernor); err != nil {
			return err
		}
	}
	return nil
}
