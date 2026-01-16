package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/cpu"
)

var CPUCmd = &cobra.Command{
	Use:   "cpu",
	Short: "CPU tuning and information",
	Long:  "View CPU information and configure CPU governor, SMT, and scheduler settings.",
}

var cpuInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show CPU information",
	RunE:  runCPUInfo,
}

var cpuGovernorCmd = &cobra.Command{
	Use:   "governor [governor]",
	Short: "Get or set CPU governor",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCPUGovernor,
}

var cpuSMTCmd = &cobra.Command{
	Use:   "smt [on|off]",
	Short: "Get or set SMT status",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCPUSMT,
}

func init() {
	CPUCmd.AddCommand(cpuInfoCmd)
	CPUCmd.AddCommand(cpuGovernorCmd)
	CPUCmd.AddCommand(cpuSMTCmd)
}

func runCPUInfo(cmd *cobra.Command, args []string) error {
	info, err := cpu.GetCPUInfo()
	if err != nil {
		return err
	}

	fmt.Printf("Model:    %s\n", info["model"])
	fmt.Printf("Cores:    %s\n", info["cores"])
	fmt.Printf("Governor: %s\n", info["governor"])
	fmt.Printf("SMT:      %s\n", info["smt"])

	if cpu.SCXIsAvailable() {
		active, _ := cpu.SCXStatus()
		fmt.Printf("SCX:      %v\n", active)
	}

	return nil
}

func runCPUGovernor(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		gov, err := cpu.GetCurrentGovernor()
		if err != nil {
			return err
		}
		fmt.Printf("Current governor: %s\n", gov)

		available, err := cpu.GetAvailableGovernors()
		if err == nil {
			fmt.Printf("Available: %v\n", available)
		}
		return nil
	}

	if err := cpu.SetGovernor(cpu.Governor(args[0])); err != nil {
		return fmt.Errorf("failed to set governor (may need root): %w", err)
	}
	fmt.Printf("Governor set to %s\n", args[0])
	return nil
}

func runCPUSMT(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		active, err := cpu.GetSMTStatus()
		if err != nil {
			return err
		}
		fmt.Printf("SMT: %v\n", active)
		return nil
	}

	enabled := args[0] == "on" || args[0] == "true" || args[0] == "1"
	if err := cpu.SetSMT(enabled); err != nil {
		return fmt.Errorf("failed to set SMT (may need root): %w", err)
	}

	status := "disabled"
	if enabled {
		status = "enabled"
	}
	fmt.Printf("SMT %s\n", status)
	return nil
}
