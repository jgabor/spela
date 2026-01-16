package commands

import (
	"fmt"

	"github.com/jgabor/spela/internal/gpu"
	"github.com/spf13/cobra"
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

	fmt.Printf("GPU:         %s\n", info["name"])
	fmt.Printf("Driver:      %s\n", info["driver"])
	fmt.Printf("VRAM:        %s\n", info["memory"])
	fmt.Printf("Temperature: %s\n", info["temperature"])
	fmt.Printf("Power:       %s\n", info["power"])

	return nil
}

func runGPUReset(cmd *cobra.Command, args []string) error {
	if err := gpu.ResetClocks(); err != nil {
		return fmt.Errorf("failed to reset clocks: %w", err)
	}
	fmt.Println("GPU clocks reset to default")
	return nil
}
