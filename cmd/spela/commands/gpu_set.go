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

var gpuFieldAliases = map[string]string{
	"clock_offset":          profile.FieldGPUClockOffset,
	"clock-offset":          profile.FieldGPUClockOffset,
	"memory_offset":         profile.FieldGPUMemoryOffset,
	"memory-offset":         profile.FieldGPUMemoryOffset,
	"power_limit":           profile.FieldGPUPowerLimit,
	"power-limit":           profile.FieldGPUPowerLimit,
	"fan_speed":             profile.FieldGPUFanSpeed,
	"fan-speed":             profile.FieldGPUFanSpeed,
	"power_mizer":           profile.FieldGPUPowerMizer,
	"power-mizer":           profile.FieldGPUPowerMizer,
	"shader_cache":          profile.FieldGPUShaderCache,
	"shader-cache":          profile.FieldGPUShaderCache,
	"shader_cache_path":     profile.FieldGPUShaderCachePath,
	"shader-cache-path":     profile.FieldGPUShaderCachePath,
	"threaded_optimization": profile.FieldGPUThreadedOptimization,
	"threaded-optimization": profile.FieldGPUThreadedOptimization,
	"threaded_opt":          profile.FieldGPUThreadedOptimization,
	"threaded-opt":          profile.FieldGPUThreadedOptimization,
}

func runGPUProfileReset(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}
	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}
	key, ok := gpuFieldAliases[args[1]]
	if !ok {
		return fmt.Errorf("unknown GPU field %q", args[1])
	}
	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		fmt.Printf("No profile for %s; field is already inherited.\n", g.Name)
		return nil
	}
	if !p.IsOverridden(key) {
		fmt.Printf("%s: %s is already inherited.\n", g.Name, args[1])
		return nil
	}
	if err := p.Reset(key); err != nil {
		return err
	}
	if err := profile.Save(g.AppID, p); err != nil {
		return err
	}
	fmt.Printf("Reset %s on %s to inherited.\n", args[1], g.Name)
	return nil
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
		p.MarkOverride(profile.FieldGPUClockOffset)
		changed = true
	}

	if cmd.Flags().Changed("memory-offset") {
		p.GPU.MemoryOffset = gpuSetMemoryOffset
		p.MarkOverride(profile.FieldGPUMemoryOffset)
		changed = true
	}

	if cmd.Flags().Changed("power-limit") {
		p.GPU.PowerLimit = gpuSetPowerLimit
		p.MarkOverride(profile.FieldGPUPowerLimit)
		changed = true
	}

	if cmd.Flags().Changed("fan-speed") {
		p.GPU.FanSpeed = gpuSetFanSpeed
		p.MarkOverride(profile.FieldGPUFanSpeed)
		changed = true
	}

	if gpuSetPowerMizer != "" {
		if gpuSetPowerMizer == "default" {
			p.GPU.PowerMizer = ""
		} else {
			p.GPU.PowerMizer = gpuSetPowerMizer
		}
		p.MarkOverride(profile.FieldGPUPowerMizer)
		changed = true
	}

	if gpuSetShaderCache != "" {
		b, err := parseBoolFlag(gpuSetShaderCache)
		if err != nil {
			return fmt.Errorf("--shader-cache: %w", err)
		}
		p.GPU.ShaderCache = b
		p.MarkOverride(profile.FieldGPUShaderCache)
		changed = true
	}

	if gpuSetCachePath != "" {
		if gpuSetCachePath == "default" {
			p.GPU.ShaderCachePath = ""
		} else {
			p.GPU.ShaderCachePath = gpuSetCachePath
		}
		p.MarkOverride(profile.FieldGPUShaderCachePath)
		changed = true
	}

	if gpuSetThreadedOpt != "" {
		b, err := parseBoolFlag(gpuSetThreadedOpt)
		if err != nil {
			return fmt.Errorf("--threaded-opt: %w", err)
		}
		p.GPU.ThreadedOptimization = b
		p.MarkOverride(profile.FieldGPUThreadedOptimization)
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

	defaults, err := profile.LoadDefault()
	if err != nil {
		return fmt.Errorf("load default profile: %w", err)
	}
	resolved := p.ResolveForApply(defaults)

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
