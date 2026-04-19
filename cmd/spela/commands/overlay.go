package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

var overlayShowJSON bool

var OverlayCmd = &cobra.Command{
	Use:   "overlay",
	Short: "Overlay profile settings",
	Long:  "Configure per-game overlay settings (position, metrics display, toggle key).",
}

var (
	overlaySetEnabled       string
	overlaySetPosition      string
	overlaySetShowFPS       string
	overlaySetShowFrametime string
	overlaySetShowCPU       string
	overlaySetShowGPU       string
	overlaySetShowVRAM      string
	overlaySetToggleKey     string
)

var overlaySetCmd = &cobra.Command{
	Use:   "set <game>",
	Short: "Set overlay profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runOverlaySet,
}

var overlayShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show overlay profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runOverlayShow,
}

func init() {
	overlaySetCmd.Flags().StringVar(&overlaySetEnabled, "enabled", "", "Enable performance overlay (true/false)")
	overlaySetCmd.Flags().StringVar(&overlaySetPosition, "position", "", "Overlay screen position (top-left/top-right/bottom-left/bottom-right)")
	overlaySetCmd.Flags().StringVar(&overlaySetShowFPS, "show-fps", "", "Show frames per second (true/false)")
	overlaySetCmd.Flags().StringVar(&overlaySetShowFrametime, "show-frametime", "", "Show frame time (true/false)")
	overlaySetCmd.Flags().StringVar(&overlaySetShowCPU, "show-cpu", "", "Show CPU usage (true/false)")
	overlaySetCmd.Flags().StringVar(&overlaySetShowGPU, "show-gpu", "", "Show GPU usage (true/false)")
	overlaySetCmd.Flags().StringVar(&overlaySetShowVRAM, "show-vram", "", "Show VRAM usage (true/false)")
	overlaySetCmd.Flags().StringVar(&overlaySetToggleKey, "toggle-key", "", "Key to toggle overlay visibility")

	overlayShowCmd.Flags().BoolVar(&overlayShowJSON, "json", false, "Output as JSON")

	OverlayCmd.AddCommand(overlaySetCmd)
	OverlayCmd.AddCommand(overlayShowCmd)
	OverlayCmd.AddCommand(overlayResetCmd)
}

var overlayResetCmd = &cobra.Command{
	Use:   "reset <game> <field>",
	Short: "Reset an overlay field to inherit from defaults",
	Long: `Reset an overlay profile field back to inherited. Valid fields:
  enabled, position, show_fps, show_frametime, show_cpu, show_gpu, show_vram, toggle_key.`,
	Args: cobra.ExactArgs(2),
	RunE: runOverlayReset,
}

var overlayFieldAliases = map[string]string{
	"enabled":        profile.FieldOverlayEnabled,
	"position":       profile.FieldOverlayPosition,
	"show_fps":       profile.FieldOverlayShowFPS,
	"show-fps":       profile.FieldOverlayShowFPS,
	"show_frametime": profile.FieldOverlayShowFrametime,
	"show-frametime": profile.FieldOverlayShowFrametime,
	"show_cpu":       profile.FieldOverlayShowCPU,
	"show-cpu":       profile.FieldOverlayShowCPU,
	"show_gpu":       profile.FieldOverlayShowGPU,
	"show-gpu":       profile.FieldOverlayShowGPU,
	"show_vram":      profile.FieldOverlayShowVRAM,
	"show-vram":      profile.FieldOverlayShowVRAM,
	"toggle_key":     profile.FieldOverlayToggleKey,
	"toggle-key":     profile.FieldOverlayToggleKey,
}

func runOverlayReset(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}
	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}
	key, ok := overlayFieldAliases[args[1]]
	if !ok {
		return fmt.Errorf("unknown overlay field %q", args[1])
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

func runOverlaySet(cmd *cobra.Command, args []string) error {
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

	if overlaySetEnabled != "" {
		p.Overlay.Enabled = overlaySetEnabled == "true" || overlaySetEnabled == "1"
		p.MarkOverride(profile.FieldOverlayEnabled)
		changed = true
	}

	if overlaySetPosition != "" {
		p.Overlay.Position = overlaySetPosition
		p.MarkOverride(profile.FieldOverlayPosition)
		changed = true
	}

	if overlaySetShowFPS != "" {
		p.Overlay.ShowFPS = overlaySetShowFPS == "true" || overlaySetShowFPS == "1"
		p.MarkOverride(profile.FieldOverlayShowFPS)
		changed = true
	}

	if overlaySetShowFrametime != "" {
		p.Overlay.ShowFrametime = overlaySetShowFrametime == "true" || overlaySetShowFrametime == "1"
		p.MarkOverride(profile.FieldOverlayShowFrametime)
		changed = true
	}

	if overlaySetShowCPU != "" {
		p.Overlay.ShowCPU = overlaySetShowCPU == "true" || overlaySetShowCPU == "1"
		p.MarkOverride(profile.FieldOverlayShowCPU)
		changed = true
	}

	if overlaySetShowGPU != "" {
		p.Overlay.ShowGPU = overlaySetShowGPU == "true" || overlaySetShowGPU == "1"
		p.MarkOverride(profile.FieldOverlayShowGPU)
		changed = true
	}

	if overlaySetShowVRAM != "" {
		p.Overlay.ShowVRAM = overlaySetShowVRAM == "true" || overlaySetShowVRAM == "1"
		p.MarkOverride(profile.FieldOverlayShowVRAM)
		changed = true
	}

	if overlaySetToggleKey != "" {
		p.Overlay.ToggleKey = overlaySetToggleKey
		p.MarkOverride(profile.FieldOverlayToggleKey)
		changed = true
	}

	if !changed {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := profile.Save(g.AppID, p); err != nil {
		return err
	}

	fmt.Printf("Updated overlay profile for %s\n", g.Name)
	return nil
}

func runOverlayShow(cmd *cobra.Command, args []string) error {
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

	defaults, _ := profile.LoadDefault()
	resolved := p.ResolveForApply(defaults)

	if overlayShowJSON {
		data, err := json.MarshalIndent(resolved.Overlay, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Overlay profile for %s:\n\n", g.Name)
	fmt.Println(renderField("Enabled:", profile.FieldOverlayEnabled, p, resolved.Overlay.Enabled))
	fmt.Println(renderField("Position:", profile.FieldOverlayPosition, p, displayOrDefault(resolved.Overlay.Position)))
	fmt.Println(renderField("Show FPS:", profile.FieldOverlayShowFPS, p, resolved.Overlay.ShowFPS))
	fmt.Println(renderField("Show frametime:", profile.FieldOverlayShowFrametime, p, resolved.Overlay.ShowFrametime))
	fmt.Println(renderField("Show CPU:", profile.FieldOverlayShowCPU, p, resolved.Overlay.ShowCPU))
	fmt.Println(renderField("Show GPU:", profile.FieldOverlayShowGPU, p, resolved.Overlay.ShowGPU))
	fmt.Println(renderField("Show VRAM:", profile.FieldOverlayShowVRAM, p, resolved.Overlay.ShowVRAM))
	fmt.Println(renderField("Toggle key:", profile.FieldOverlayToggleKey, p, displayOrDefault(resolved.Overlay.ToggleKey)))

	return nil
}

func displayOrDefault(s string) string {
	if s == "" {
		return "(default)"
	}
	return s
}
