package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/tui"
)

var GPUCmd = &cobra.Command{
	Use:   "gpu",
	Short: "GPU tuning and information",
	Long:  "View GPU information and configure NVIDIA GPU settings.",
}

var gpuInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show GPU information",
	RunE:  runGPUInfo,
}

var gpuResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset GPU clocks to default",
	RunE:  runGPUReset,
}

func init() {
	GPUCmd.AddCommand(gpuInfoCmd)
	GPUCmd.AddCommand(gpuResetCmd)
}

func runGPUInfo(cmd *cobra.Command, args []string) error {
	info, err := gpu.GetGPUInfo()
	if err != nil {
		return fmt.Errorf("failed to get GPU info: %w", err)
	}

	fmt.Printf("%s  %s\n", tui.CLIDim("GPU:"), tui.CLIPrimary(info["name"]))
	fmt.Printf("%s  %s\n", tui.CLIDim("Driver:"), info["driver"])
	fmt.Printf("%s  %s\n", tui.CLIDim("VRAM:"), tui.CLIAccent(info["memory"]))
	fmt.Printf("%s  %s\n", tui.CLIDim("Temp:"), tui.CLIAccent(info["temperature"]))

	// Power section — enhanced with NVML data when available
	if draw := info["power_draw"]; draw != "" {
		fmt.Printf("%s  %s", tui.CLIDim("Power:"), tui.CLIAccent(draw))
		if limit := info["power_limit"]; limit != "" {
			fmt.Printf(" / %s limit", limit)
		}
		if r := info["power_range"]; r != "" {
			fmt.Printf(" (%s range)", r)
		}
		fmt.Println()
	} else if p := info["power"]; p != "" {
		// nvidia-smi fallback
		fmt.Printf("%s  %s\n", tui.CLIDim("Power:"), tui.CLIAccent(p))
	}

	if clock := info["graphics_clock"]; clock != "" {
		fmt.Printf("%s  %s\n", tui.CLIDim("Core:"), tui.CLIAccent(clock))
	}
	if clock := info["memory_clock"]; clock != "" {
		fmt.Printf("%s  %s\n", tui.CLIDim("Mem:"), tui.CLIAccent(clock))
	}
	if fan := info["fan_speed"]; fan != "" {
		fmt.Printf("%s  %s\n", tui.CLIDim("Fan:"), tui.CLIAccent(fan))
	}
	if util := info["utilization"]; util != "" {
		fmt.Printf("%s  %s\n", tui.CLIDim("Load:"), tui.CLIAccent(util))
	}

	return nil
}

func runGPUReset(cmd *cobra.Command, args []string) error {
	if err := gpu.ResetClocks(); err != nil {
		return fmt.Errorf("failed to reset clocks: %w", err)
	}
	fmt.Println(tui.CLISuccess("GPU clocks reset to default"))
	return nil
}
