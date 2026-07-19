package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

var gpuShowJSON bool

var (
	gpuSetClockOffset  int
	gpuSetMemoryOffset int
	gpuSetPowerLimit   int
	gpuSetFanSpeed     int
	gpuSetPowerMizer   string
	gpuSetShaderCache  string
	gpuSetCachePath    string
	gpuSetThreadedOpt  string
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
	gpuSetCmd.Flags().IntVar(&gpuSetPowerLimit, "power-limit", 0, "GPU power limit in watts")
	gpuSetCmd.Flags().IntVar(&gpuSetFanSpeed, "fan-speed", 0, "GPU fan speed percentage (0 to clear)")
	gpuSetCmd.Flags().StringVar(&gpuSetPowerMizer, "power-mizer", "", "GPU power mode (adaptive, max)")
	gpuSetCmd.Flags().StringVar(&gpuSetShaderCache, "shader-cache", "", "Enable shader caching (true/false)")
	gpuSetCmd.Flags().StringVar(&gpuSetCachePath, "shader-cache-path", "", "Custom shader cache path (use 'default' to clear)")
	gpuSetCmd.Flags().StringVar(&gpuSetThreadedOpt, "threaded-opt", "", "Enable threaded optimization (true/false)")

	gpuShowCmd.Flags().BoolVar(&gpuShowJSON, "json", false, "Output as JSON")

	GPUCmd.AddCommand(gpuSetCmd)
	GPUCmd.AddCommand(gpuShowCmd)
	GPUCmd.AddCommand(gpuProfileResetCmd)
}

var gpuProfileResetCmd = &cobra.Command{
	Use:   "profile-reset <game> <field>",
	Short: "Reset a GPU profile field to inherit from defaults",
	Long: `Reset a GPU profile field back to inherited. Valid fields:
  clock_offset, memory_offset, power_limit, fan_speed,
  power_mizer, shader_cache, shader_cache_path, threaded_optimization.`,
	Args: cobra.ExactArgs(2),
	RunE: runGPUProfileReset,
}

func runGPUProfileReset(_ *cobra.Command, args []string) error {
	return resetProfileField(args, "gpu", "GPU", "")
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

	changes := profileChanges{name: g.Name}

	if cmd.Flags().Changed("clock-offset") {
		changes.set(profile.FieldGPUClockOffset, gpuSetClockOffset)
	}

	if cmd.Flags().Changed("memory-offset") {
		changes.set(profile.FieldGPUMemoryOffset, gpuSetMemoryOffset)
	}

	if cmd.Flags().Changed("power-limit") {
		changes.set(profile.FieldGPUPowerLimit, gpuSetPowerLimit)
	}

	if cmd.Flags().Changed("fan-speed") {
		changes.set(profile.FieldGPUFanSpeed, gpuSetFanSpeed)
	}

	if gpuSetPowerMizer != "" {
		value := gpuSetPowerMizer
		if value == "default" {
			value = ""
		}
		changes.set(profile.FieldGPUPowerMizer, value)
	}

	if gpuSetShaderCache != "" {
		b, err := parseBoolFlag(gpuSetShaderCache)
		if err != nil {
			return fmt.Errorf("--shader-cache: %w", err)
		}
		changes.set(profile.FieldGPUShaderCache, b)
	}

	if gpuSetCachePath != "" {
		value := gpuSetCachePath
		if value == "default" {
			value = ""
		}
		changes.set(profile.FieldGPUShaderCachePath, value)
	}

	if gpuSetThreadedOpt != "" {
		b, err := parseBoolFlag(gpuSetThreadedOpt)
		if err != nil {
			return fmt.Errorf("--threaded-opt: %w", err)
		}
		changes.set(profile.FieldGPUThreadedOptimization, b)
	}

	if len(changes.mutations) == 0 {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := changes.save(g.AppID); err != nil {
		return err
	}

	fmt.Printf("Updated GPU profile for %s\n", g.Name)
	return nil
}

func runGPUShow(cmd *cobra.Command, args []string) error {
	g, p, resolved, err := resolvedProfileForShow(args[0])
	if err != nil {
		return err
	}

	if gpuShowJSON {
		data, err := json.MarshalIndent(resolved.GPU, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("GPU profile for %s:\n\n", g.Name)
	fmt.Println(renderField("Clock offset:", profile.FieldGPUClockOffset, p, displayGPUInt(resolved.GPU.ClockOffset)))
	fmt.Println(renderField("Memory offset:", profile.FieldGPUMemoryOffset, p, displayGPUInt(resolved.GPU.MemoryOffset)))
	fmt.Println(renderField("Power limit:", profile.FieldGPUPowerLimit, p, displayGPUInt(resolved.GPU.PowerLimit)))
	fmt.Println(renderField("Fan speed:", profile.FieldGPUFanSpeed, p, displayGPUPercent(resolved.GPU.FanSpeed)))
	fmt.Println(renderField("Power mode:", profile.FieldGPUPowerMizer, p, displayGPUString(resolved.GPU.PowerMizer)))
	fmt.Println(renderField("Shader cache:", profile.FieldGPUShaderCache, p, resolved.GPU.ShaderCache))
	fmt.Println(renderField("Threaded opt:", profile.FieldGPUThreadedOptimization, p, resolved.GPU.ThreadedOptimization))

	return nil
}

func displayGPUInt(v int) string {
	if v == 0 {
		return "(default)"
	}
	return fmt.Sprintf("%d", v)
}

func displayGPUPercent(v int) string {
	if v == 0 {
		return "(auto)"
	}
	return fmt.Sprintf("%d%%", v)
}

func displayGPUString(v string) string {
	if v == "" {
		return "(default)"
	}
	return v
}
