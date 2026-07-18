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

type applyProfileOperations struct {
	lockGraphicsClocks func(int) error
	lockMemoryClocks   func(int) error
	setPowerLimit      func(int) error
	setFanSpeed        func(int) error
	resetClocks        func() error
	resetFanSpeed      func() error
	setGovernor        func(cpu.Governor) error
	setSMT             func(bool) error
}

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
		return applyResetSettings(cmd, systemApplyProfileOperations())
	}

	return applySettings(cmd, systemApplyProfileOperations())
}

func systemApplyProfileOperations() applyProfileOperations {
	return applyProfileOperations{
		lockGraphicsClocks: gpu.LockGraphicsClocks,
		lockMemoryClocks:   gpu.LockMemoryClocks,
		setPowerLimit:      gpu.SetPowerLimit,
		setFanSpeed:        gpu.SetFanSpeed,
		resetClocks:        gpu.ResetClocks,
		resetFanSpeed:      gpu.ResetFanSpeed,
		setGovernor:        cpu.SetGovernor,
		setSMT:             cpu.SetSMT,
	}
}

func applySettings(cmd *cobra.Command, operations applyProfileOperations) error {
	if err := validateApplyInputs(cmd); err != nil {
		return err
	}

	if cmd.Flags().Changed("gpu-clock-offset") {
		if err := operations.lockGraphicsClocks(applyGPUClockOffset); err != nil {
			return fmt.Errorf("set GPU clock offset: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-memory-offset") {
		if err := operations.lockMemoryClocks(applyGPUMemoryOffset); err != nil {
			return fmt.Errorf("set GPU memory offset: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-power-limit") {
		if err := operations.setPowerLimit(applyGPUPowerLimit); err != nil {
			return fmt.Errorf("set GPU power limit: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-fan-speed") && applyGPUFanSpeed > 0 {
		if err := operations.setFanSpeed(applyGPUFanSpeed); err != nil {
			return fmt.Errorf("set GPU fan speed: %w", err)
		}
	}

	if applyCPUGovernor != "" {
		if err := operations.setGovernor(cpu.Governor(applyCPUGovernor)); err != nil {
			return fmt.Errorf("set CPU governor: %w", err)
		}
	}

	if applyCPUSMT == "on" || applyCPUSMT == "off" {
		if err := operations.setSMT(applyCPUSMT == "on"); err != nil {
			return fmt.Errorf("set CPU SMT: %w", err)
		}
	}

	return nil
}

func applyResetSettings(cmd *cobra.Command, operations applyProfileOperations) error {
	if err := validateApplyInputs(cmd); err != nil {
		return err
	}

	if err := operations.resetClocks(); err != nil {
		return fmt.Errorf("reset GPU clocks: %w", err)
	}

	if cmd.Flags().Changed("gpu-power-limit") && applyGPUPowerLimit > 0 {
		if err := operations.setPowerLimit(applyGPUPowerLimit); err != nil {
			return fmt.Errorf("restore GPU power limit: %w", err)
		}
	}

	if cmd.Flags().Changed("gpu-fan-speed") {
		if applyGPUFanSpeed == 0 {
			if err := operations.resetFanSpeed(); err != nil {
				return fmt.Errorf("reset GPU fan speed: %w", err)
			}
		} else {
			if err := operations.setFanSpeed(applyGPUFanSpeed); err != nil {
				return fmt.Errorf("restore GPU fan speed: %w", err)
			}
		}
	}

	if applyCPUGovernor != "" {
		if err := operations.setGovernor(cpu.Governor(applyCPUGovernor)); err != nil {
			return fmt.Errorf("restore CPU governor: %w", err)
		}
	}

	if applyCPUSMT == "on" || applyCPUSMT == "off" {
		if err := operations.setSMT(applyCPUSMT == "on"); err != nil {
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
