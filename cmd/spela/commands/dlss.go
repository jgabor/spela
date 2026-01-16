package commands

import (
	"fmt"
	"strconv"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/spf13/cobra"
)

var DLSSCmd = &cobra.Command{
	Use:   "dlss",
	Short: "Configure DLSS settings",
	Long:  "Configure DLSS Super Resolution, Ray Reconstruction, and Frame Generation settings.",
}

var dlssShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show DLSS configuration for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runDLSSShow,
}

var dlssSetSRMode string
var dlssSetSRPreset string
var dlssSetRRMode string
var dlssSetFGEnabled string
var dlssSetMultiFrame int
var dlssSetIndicator bool

var dlssSetCmd = &cobra.Command{
	Use:   "set <game>",
	Short: "Set DLSS configuration for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runDLSSSet,
}

func init() {
	dlssSetCmd.Flags().StringVar(&dlssSetSRMode, "sr-mode", "", "DLSS-SR mode (off, ultra_performance, performance, balanced, quality, dlaa)")
	dlssSetCmd.Flags().StringVar(&dlssSetSRPreset, "sr-preset", "", "DLSS-SR preset (default, A, B, C, D, latest)")
	dlssSetCmd.Flags().StringVar(&dlssSetRRMode, "rr-mode", "", "DLSS-RR mode")
	dlssSetCmd.Flags().StringVar(&dlssSetFGEnabled, "fg", "", "Frame generation (true/false)")
	dlssSetCmd.Flags().IntVar(&dlssSetMultiFrame, "multi-frame", -1, "Multi-frame count (0-3)")
	dlssSetCmd.Flags().BoolVar(&dlssSetIndicator, "indicator", false, "Enable DLSS indicator")

	DLSSCmd.AddCommand(dlssShowCmd)
	DLSSCmd.AddCommand(dlssSetCmd)
}

func runDLSSShow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		fmt.Printf("No profile for %s, using defaults\n", g.Name)
		p = profile.FromPreset(profile.PresetBalanced)
	}

	fmt.Printf("DLSS configuration for %s:\n\n", g.Name)
	fmt.Printf("Super Resolution (SR):\n")
	fmt.Printf("  Mode:     %s\n", p.DLSS.SRMode)
	fmt.Printf("  Preset:   %s\n", p.DLSS.SRPreset)
	fmt.Printf("  Override: %v\n", p.DLSS.SROverride)

	fmt.Printf("\nRay Reconstruction (RR):\n")
	fmt.Printf("  Mode:     %s\n", p.DLSS.RRMode)
	fmt.Printf("  Preset:   %s\n", p.DLSS.RRPreset)
	fmt.Printf("  Override: %v\n", p.DLSS.RROverride)

	fmt.Printf("\nFrame Generation (FG):\n")
	fmt.Printf("  Enabled:     %v\n", p.DLSS.FGEnabled)
	fmt.Printf("  Multi-frame: %d\n", p.DLSS.MultiFrame)
	fmt.Printf("  Override:    %v\n", p.DLSS.FGOverride)

	fmt.Printf("\nDebug:\n")
	fmt.Printf("  Indicator:    %v\n", p.DLSS.Indicator)
	fmt.Printf("  FG Indicator: %v\n", p.DLSS.FGIndicator)

	return nil
}

func runDLSSSet(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		p = profile.FromPreset(profile.PresetBalanced)
		p.Name = g.Name
	}

	changed := false

	if dlssSetSRMode != "" {
		p.DLSS.SRMode = profile.DLSSMode(dlssSetSRMode)
		p.DLSS.SROverride = true
		changed = true
	}

	if dlssSetSRPreset != "" {
		p.DLSS.SRPreset = profile.DLSSPreset(dlssSetSRPreset)
		p.DLSS.SROverride = true
		changed = true
	}

	if dlssSetRRMode != "" {
		p.DLSS.RRMode = profile.DLSSMode(dlssSetRRMode)
		p.DLSS.RROverride = true
		changed = true
	}

	if dlssSetFGEnabled != "" {
		p.DLSS.FGEnabled = dlssSetFGEnabled == "true" || dlssSetFGEnabled == "1"
		p.DLSS.FGOverride = true
		changed = true
	}

	if dlssSetMultiFrame >= 0 {
		p.DLSS.MultiFrame = dlssSetMultiFrame
		p.DLSS.FGOverride = true
		changed = true
	}

	if cmd.Flags().Changed("indicator") {
		p.DLSS.Indicator = dlssSetIndicator
		changed = true
	}

	if !changed {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := profile.Save(g.AppID, p); err != nil {
		return err
	}

	fmt.Printf("Updated DLSS configuration for %s\n", g.Name)
	return nil
}
