package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

var (
	gpuSetClockOffset  int
	gpuSetMemoryOffset int
	gpuSetPowerMizer   string
)

var gpuSetCmd = &cobra.Command{
	Use:   "set <game>",
	Short: "Set GPU profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runGPUSet,
}

var gpuShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show GPU profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runGPUShow,
}

func init() {
	gpuSetCmd.Flags().IntVar(&gpuSetClockOffset, "clock-offset", 0, "GPU core clock offset in MHz")
	gpuSetCmd.Flags().IntVar(&gpuSetMemoryOffset, "memory-offset", 0, "GPU memory clock offset in MHz")
	gpuSetCmd.Flags().StringVar(&gpuSetPowerMizer, "power-mizer", "", "GPU power mode (adaptive, max)")

	GPUCmd.AddCommand(gpuSetCmd)
	GPUCmd.AddCommand(gpuShowCmd)
}

func runGPUSet(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		p = &profile.Profile{Name: g.Name}
	}

	changed := false

	if cmd.Flags().Changed("clock-offset") {
		p.GPU.ClockOffset = gpuSetClockOffset
		changed = true
	}

	if cmd.Flags().Changed("memory-offset") {
		p.GPU.MemoryOffset = gpuSetMemoryOffset
		changed = true
	}

	if gpuSetPowerMizer != "" {
		p.GPU.PowerMizer = gpuSetPowerMizer
		changed = true
	}

	if !changed {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := profile.Save(g.AppID, p); err != nil {
		return err
	}

	fmt.Printf("Updated GPU profile for %s\n", g.Name)
	return nil
}

func runGPUShow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		fmt.Printf("No profile for %s\n", g.Name)
		p = &profile.Profile{}
	}

	fmt.Printf("GPU profile for %s:\n\n", g.Name)
	fmt.Printf("%s  %s\n", tui.CLIDim("Clock offset:"), displayGPUInt(p.GPU.ClockOffset))
	fmt.Printf("%s  %s\n", tui.CLIDim("Memory offset:"), displayGPUInt(p.GPU.MemoryOffset))
	fmt.Printf("%s  %s\n", tui.CLIDim("Power mode:"), displayGPUString(p.GPU.PowerMizer))
	fmt.Printf("%s  %v\n", tui.CLIDim("Shader cache:"), p.GPU.ShaderCache)
	fmt.Printf("%s  %v\n", tui.CLIDim("Threaded opt:"), p.GPU.ThreadedOptimization)

	return nil
}

func displayGPUInt(v int) string {
	if v == 0 {
		return "(default)"
	}
	return fmt.Sprintf("%d", v)
}

func displayGPUString(v string) string {
	if v == "" {
		return "(default)"
	}
	return v
}
