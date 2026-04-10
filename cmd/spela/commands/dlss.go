package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

var dlssShowJSON bool

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

var (
	dlssSetSRMode        string
	dlssSetSRPreset      string
	dlssSetSRModelPreset string
	dlssSetRRMode        string
	dlssSetRRPreset      string
	dlssSetRROverride    string
	dlssSetFGEnabled     string
	dlssSetFGOverride    string
	dlssSetFGIndicator   bool
	dlssSetMultiFrame    int
	dlssSetIndicator     bool
)

var dlssSetCmd = &cobra.Command{
	Use:   "set <game>",
	Short: "Set DLSS configuration for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runDLSSSet,
}

func init() {
	dlssSetCmd.Flags().StringVar(&dlssSetSRMode, "sr-mode", "", "DLSS-SR mode (off, ultra_performance, performance, balanced, quality, dlaa)")
	dlssSetCmd.Flags().StringVar(&dlssSetSRPreset, "sr-preset", "", "DLSS-SR preset (default, A, B, C, D, E, F, J, K, L, M)")
	dlssSetCmd.Flags().StringVar(&dlssSetSRModelPreset, "sr-model-preset", "", "DLSS-SR model preset (auto, k, l, m)")
	dlssSetCmd.Flags().StringVar(&dlssSetRRMode, "rr-mode", "", "DLSS-RR mode")
	dlssSetCmd.Flags().StringVar(&dlssSetRRPreset, "rr-preset", "", "DLSS-RR preset (default, A, B, C, D, E, F, J, K, L, M)")
	dlssSetCmd.Flags().StringVar(&dlssSetRROverride, "rr-override", "", "Force ray reconstruction override (true/false)")
	dlssSetCmd.Flags().StringVar(&dlssSetFGEnabled, "fg", "", "Frame generation (true/false)")
	dlssSetCmd.Flags().StringVar(&dlssSetFGOverride, "fg-override", "", "Force frame generation override (true/false)")
	dlssSetCmd.Flags().BoolVar(&dlssSetFGIndicator, "fg-indicator", false, "Enable frame generation indicator")
	dlssSetCmd.Flags().IntVar(&dlssSetMultiFrame, "multi-frame", -1, "Multi-frame count (0-3)")
	dlssSetCmd.Flags().BoolVar(&dlssSetIndicator, "indicator", false, "Enable DLSS indicator")

	dlssShowCmd.Flags().BoolVar(&dlssShowJSON, "json", false, "Output as JSON")

	DLSSCmd.AddCommand(dlssShowCmd)
	DLSSCmd.AddCommand(dlssSetCmd)
}

func runDLSSShow(cmd *cobra.Command, args []string) error {
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

	if dlssShowJSON {
		data, err := json.MarshalIndent(p.DLSS, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("DLSS configuration for %s:\n\n", g.Name)
	fmt.Printf("Super Resolution (SR):\n")
	fmt.Printf("  Mode:          %s\n", p.DLSS.SRMode)
	fmt.Printf("  Preset:        %s\n", p.DLSS.SRPreset)
	fmt.Printf("  Model Preset:  %s\n", p.DLSS.SRModelPreset)
	fmt.Printf("  Override:      %v\n", p.DLSS.SROverride)

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

	if dlssSetSRModelPreset != "" {
		p.DLSS.SRModelPreset = profile.DLSSModelPreset(dlssSetSRModelPreset)
		changed = true
	}

	if dlssSetRRMode != "" {
		p.DLSS.RRMode = profile.DLSSMode(dlssSetRRMode)
		p.DLSS.RROverride = true
		changed = true
	}

	if dlssSetRRPreset != "" {
		p.DLSS.RRPreset = profile.DLSSPreset(dlssSetRRPreset)
		p.DLSS.RROverride = true
		changed = true
	}

	if dlssSetRROverride != "" {
		p.DLSS.RROverride = dlssSetRROverride == "true" || dlssSetRROverride == "1"
		changed = true
	}

	if dlssSetFGEnabled != "" {
		p.DLSS.FGEnabled = dlssSetFGEnabled == "true" || dlssSetFGEnabled == "1"
		p.DLSS.FGOverride = true
		changed = true
	}

	if dlssSetFGOverride != "" {
		p.DLSS.FGOverride = dlssSetFGOverride == "true" || dlssSetFGOverride == "1"
		changed = true
	}

	if dlssSetMultiFrame >= 0 {
		p.DLSS.MultiFrame = dlssSetMultiFrame
		p.DLSS.FGOverride = true
		changed = true
	}

	if cmd.Flags().Changed("fg-indicator") {
		p.DLSS.FGIndicator = dlssSetFGIndicator
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
