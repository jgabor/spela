package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

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

	OverlayCmd.AddCommand(overlaySetCmd)
	OverlayCmd.AddCommand(overlayShowCmd)
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
		changed = true
	}

	if overlaySetPosition != "" {
		p.Overlay.Position = overlaySetPosition
		changed = true
	}

	if overlaySetShowFPS != "" {
		p.Overlay.ShowFPS = overlaySetShowFPS == "true" || overlaySetShowFPS == "1"
		changed = true
	}

	if overlaySetShowFrametime != "" {
		p.Overlay.ShowFrametime = overlaySetShowFrametime == "true" || overlaySetShowFrametime == "1"
		changed = true
	}

	if overlaySetShowCPU != "" {
		p.Overlay.ShowCPU = overlaySetShowCPU == "true" || overlaySetShowCPU == "1"
		changed = true
	}

	if overlaySetShowGPU != "" {
		p.Overlay.ShowGPU = overlaySetShowGPU == "true" || overlaySetShowGPU == "1"
		changed = true
	}

	if overlaySetShowVRAM != "" {
		p.Overlay.ShowVRAM = overlaySetShowVRAM == "true" || overlaySetShowVRAM == "1"
		changed = true
	}

	if overlaySetToggleKey != "" {
		p.Overlay.ToggleKey = overlaySetToggleKey
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

	fmt.Printf("Overlay profile for %s:\n\n", g.Name)
	fmt.Printf("%s  %v\n", tui.CLIDim("Enabled:"), p.Overlay.Enabled)
	fmt.Printf("%s  %s\n", tui.CLIDim("Position:"), displayOrDefault(p.Overlay.Position))
	fmt.Printf("%s  %v\n", tui.CLIDim("Show FPS:"), p.Overlay.ShowFPS)
	fmt.Printf("%s  %v\n", tui.CLIDim("Show frametime:"), p.Overlay.ShowFrametime)
	fmt.Printf("%s  %v\n", tui.CLIDim("Show CPU:"), p.Overlay.ShowCPU)
	fmt.Printf("%s  %v\n", tui.CLIDim("Show GPU:"), p.Overlay.ShowGPU)
	fmt.Printf("%s  %v\n", tui.CLIDim("Show VRAM:"), p.Overlay.ShowVRAM)
	fmt.Printf("%s  %s\n", tui.CLIDim("Toggle key:"), displayOrDefault(p.Overlay.ToggleKey))

	return nil
}

func displayOrDefault(s string) string {
	if s == "" {
		return "(default)"
	}
	return s
}
