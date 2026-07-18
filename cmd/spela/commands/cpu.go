package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/tui"
)

var CPUCmd = &cobra.Command{
	Use:   "cpu",
	Short: "CPU tuning and information",
	Long:  "View CPU information and configure CPU governor, SMT, and scheduler settings.",
}

type cpuReadOperations struct {
	getInfo               func() (map[string]string, error)
	getAvailableGovernors func() ([]cpu.Governor, error)
	getMetrics            func() (*cpu.CPUMetrics, error)
	scxIsAvailable        func() bool
	scxStatus             func() (bool, error)
	getCurrentGovernor    func() (cpu.Governor, error)
	getSMTStatus          func() (bool, error)
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
	return runCPUInfoWithOperations(systemCPUReadOperations())
}

func systemCPUReadOperations() cpuReadOperations {
	return cpuReadOperations{
		getInfo:               cpu.GetCPUInfo,
		getAvailableGovernors: cpu.GetAvailableGovernors,
		getMetrics:            cpu.GetCPUMetrics,
		scxIsAvailable:        cpu.SCXIsAvailable,
		scxStatus:             cpu.SCXStatus,
		getCurrentGovernor:    cpu.GetCurrentGovernor,
		getSMTStatus:          cpu.GetSMTStatus,
	}
}

func runCPUInfoWithOperations(operations cpuReadOperations) error {
	info, err := operations.getInfo()
	if err != nil {
		return err
	}

	fmt.Printf("%s  %s\n", tui.CLIDim("CPU:"), tui.CLIPrimary(info["model"]))
	fmt.Printf("%s  %s\n", tui.CLIDim("Cores:"), tui.CLIAccent(info["cores"]))

	// Governor with available options
	fmt.Printf("%s  %s", tui.CLIDim("Governor:"), tui.CLIAccent(info["governor"]))
	if available, err := operations.getAvailableGovernors(); err == nil && len(available) > 0 {
		names := make([]string, len(available))
		for i, g := range available {
			names[i] = string(g)
		}
		fmt.Printf(" (%s available)", strings.Join(names, ", "))
	}
	fmt.Println()

	// SMT
	smt := info["smt"]
	switch smt {
	case "true":
		smt = "enabled"
	case "false":
		smt = "disabled"
	}
	fmt.Printf("%s  %s\n", tui.CLIDim("SMT:"), tui.CLIAccent(smt))

	// Live metrics from GetCPUMetrics
	metrics, metricsErr := operations.getMetrics()
	if metricsErr == nil && metrics != nil {
		if metrics.AverageFrequency > 0 {
			fmt.Printf("%s  %s\n", tui.CLIDim("Freq:"), tui.CLIAccent(fmt.Sprintf("%d MHz avg", metrics.AverageFrequency)))
		}
		if metrics.Utilization > 0 {
			fmt.Printf("%s  %s\n", tui.CLIDim("Load:"), tui.CLIAccent(fmt.Sprintf("%.0f%%", metrics.Utilization)))
		}
		if metrics.RAMTotalMB > 0 {
			fmt.Printf("%s  %s\n", tui.CLIDim("RAM:"), tui.CLIAccent(fmt.Sprintf("%d / %d MB", metrics.RAMUsedMB, metrics.RAMTotalMB)))
		}
	}

	if operations.scxIsAvailable() {
		active, _ := operations.scxStatus()
		status := "inactive"
		if active {
			status = "active"
		}
		fmt.Printf("%s  %s\n", tui.CLIDim("SCX:"), tui.CLIAccent(status))
	}

	return nil
}

func runCPUGovernor(cmd *cobra.Command, args []string) error {
	return runCPUGovernorWithOperations(args, systemCPUReadOperations())
}

func runCPUGovernorWithOperations(args []string, operations cpuReadOperations) error {
	if len(args) == 0 {
		gov, err := operations.getCurrentGovernor()
		if err != nil {
			return err
		}
		fmt.Printf("Current governor: %s\n", gov)

		available, err := operations.getAvailableGovernors()
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
	return runCPUSMTWithOperations(args, systemCPUReadOperations())
}

func runCPUSMTWithOperations(args []string, operations cpuReadOperations) error {
	if len(args) == 0 {
		active, err := operations.getSMTStatus()
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
