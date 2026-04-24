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
	DLSSCmd.AddCommand(dlssResetCmd)
}

var dlssResetCmd = &cobra.Command{
	Use:   "reset <game> <field>",
	Short: "Reset a DLSS field to inherit from defaults",
	Long: `Reset a DLSS profile field back to inherited. Valid fields:
  sr_mode, sr_preset, sr_model_preset, sr_override,
  rr_mode, rr_preset, rr_override,
  fg_enabled, fg_override, multi_frame, indicator, fg_indicator.`,
	Args: cobra.ExactArgs(2),
	RunE: runDLSSReset,
}

// dlssFieldAliases accepts both dash and underscore forms so
// 'sr-mode' and 'sr_mode' both work.
var dlssFieldAliases = map[string]string{
	"sr_mode":         profile.FieldDLSSSRMode,
	"sr-mode":         profile.FieldDLSSSRMode,
	"sr_preset":       profile.FieldDLSSSRPreset,
	"sr-preset":       profile.FieldDLSSSRPreset,
	"sr_model_preset": profile.FieldDLSSSRModelPreset,
	"sr-model-preset": profile.FieldDLSSSRModelPreset,
	"sr_override":     profile.FieldDLSSSROverride,
	"sr-override":     profile.FieldDLSSSROverride,
	"rr_mode":         profile.FieldDLSSRRMode,
	"rr-mode":         profile.FieldDLSSRRMode,
	"rr_preset":       profile.FieldDLSSRRPreset,
	"rr-preset":       profile.FieldDLSSRRPreset,
	"rr_override":     profile.FieldDLSSRROverride,
	"rr-override":     profile.FieldDLSSRROverride,
	"fg_enabled":      profile.FieldDLSSFGEnabled,
	"fg-enabled":      profile.FieldDLSSFGEnabled,
	"fg":              profile.FieldDLSSFGEnabled,
	"fg_override":     profile.FieldDLSSFGOverride,
	"fg-override":     profile.FieldDLSSFGOverride,
	"multi_frame":     profile.FieldDLSSMultiFrame,
	"multi-frame":     profile.FieldDLSSMultiFrame,
	"indicator":       profile.FieldDLSSIndicator,
	"fg_indicator":    profile.FieldDLSSFGIndicator,
	"fg-indicator":    profile.FieldDLSSFGIndicator,
}

func runDLSSReset(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}
	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}
	key, ok := dlssFieldAliases[args[1]]
	if !ok {
		return fmt.Errorf("unknown DLSS field %q", args[1])
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

	defaults, err := profile.LoadDefault()
	if err != nil {
		return fmt.Errorf("load default profile: %w", err)
	}
	resolved := p.ResolveForApply(defaults)

	if dlssShowJSON {
		data, err := json.MarshalIndent(resolved.DLSS, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("DLSS configuration for %s:\n\n", g.Name)
	fmt.Printf("Super Resolution (SR):\n")
	fmt.Printf("  %s\n", renderField("Mode:", profile.FieldDLSSSRMode, p, resolved.DLSS.SRMode))
	fmt.Printf("  %s\n", renderField("Preset:", profile.FieldDLSSSRPreset, p, resolved.DLSS.SRPreset))
	fmt.Printf("  %s\n", renderField("Model Preset:", profile.FieldDLSSSRModelPreset, p, resolved.DLSS.SRModelPreset))
	fmt.Printf("  %s\n", renderField("Override:", profile.FieldDLSSSROverride, p, resolved.DLSS.SROverride))

	fmt.Printf("\nRay Reconstruction (RR):\n")
	fmt.Printf("  %s\n", renderField("Mode:", profile.FieldDLSSRRMode, p, resolved.DLSS.RRMode))
	fmt.Printf("  %s\n", renderField("Preset:", profile.FieldDLSSRRPreset, p, resolved.DLSS.RRPreset))
	fmt.Printf("  %s\n", renderField("Override:", profile.FieldDLSSRROverride, p, resolved.DLSS.RROverride))

	fmt.Printf("\nFrame Generation (FG):\n")
	fmt.Printf("  %s\n", renderField("Enabled:", profile.FieldDLSSFGEnabled, p, resolved.DLSS.FGEnabled))
	fmt.Printf("  %s\n", renderField("Multi-frame:", profile.FieldDLSSMultiFrame, p, resolved.DLSS.MultiFrame))
	fmt.Printf("  %s\n", renderField("Override:", profile.FieldDLSSFGOverride, p, resolved.DLSS.FGOverride))

	fmt.Printf("\nDebug:\n")
	fmt.Printf("  %s\n", renderField("Indicator:", profile.FieldDLSSIndicator, p, resolved.DLSS.Indicator))
	fmt.Printf("  %s\n", renderField("FG Indicator:", profile.FieldDLSSFGIndicator, p, resolved.DLSS.FGIndicator))

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
		p.MarkOverride(profile.FieldDLSSSRMode)
		p.MarkOverride(profile.FieldDLSSSROverride)
		changed = true
	}

	if dlssSetSRPreset != "" {
		p.DLSS.SRPreset = profile.DLSSPreset(dlssSetSRPreset)
		p.DLSS.SROverride = true
		p.MarkOverride(profile.FieldDLSSSRPreset)
		p.MarkOverride(profile.FieldDLSSSROverride)
		changed = true
	}

	if dlssSetSRModelPreset != "" {
		p.DLSS.SRModelPreset = profile.DLSSModelPreset(dlssSetSRModelPreset)
		p.MarkOverride(profile.FieldDLSSSRModelPreset)
		changed = true
	}

	if dlssSetRRMode != "" {
		p.DLSS.RRMode = profile.DLSSMode(dlssSetRRMode)
		p.DLSS.RROverride = true
		p.MarkOverride(profile.FieldDLSSRRMode)
		p.MarkOverride(profile.FieldDLSSRROverride)
		changed = true
	}

	if dlssSetRRPreset != "" {
		p.DLSS.RRPreset = profile.DLSSPreset(dlssSetRRPreset)
		p.DLSS.RROverride = true
		p.MarkOverride(profile.FieldDLSSRRPreset)
		p.MarkOverride(profile.FieldDLSSRROverride)
		changed = true
	}

	if dlssSetRROverride != "" {
		b, err := parseBoolFlag(dlssSetRROverride)
		if err != nil {
			return fmt.Errorf("--rr-override: %w", err)
		}
		p.DLSS.RROverride = b
		p.MarkOverride(profile.FieldDLSSRROverride)
		changed = true
	}

	if dlssSetFGEnabled != "" {
		b, err := parseBoolFlag(dlssSetFGEnabled)
		if err != nil {
			return fmt.Errorf("--fg: %w", err)
		}
		p.DLSS.FGEnabled = b
		p.DLSS.FGOverride = true
		p.MarkOverride(profile.FieldDLSSFGEnabled)
		p.MarkOverride(profile.FieldDLSSFGOverride)
		changed = true
	}

	if dlssSetFGOverride != "" {
		b, err := parseBoolFlag(dlssSetFGOverride)
		if err != nil {
			return fmt.Errorf("--fg-override: %w", err)
		}
		p.DLSS.FGOverride = b
		p.MarkOverride(profile.FieldDLSSFGOverride)
		changed = true
	}

	if dlssSetMultiFrame >= 0 {
		p.DLSS.MultiFrame = dlssSetMultiFrame
		p.DLSS.FGOverride = true
		p.MarkOverride(profile.FieldDLSSMultiFrame)
		p.MarkOverride(profile.FieldDLSSFGOverride)
		changed = true
	}

	if cmd.Flags().Changed("fg-indicator") {
		p.DLSS.FGIndicator = dlssSetFGIndicator
		p.MarkOverride(profile.FieldDLSSFGIndicator)
		changed = true
	}

	if cmd.Flags().Changed("indicator") {
		p.DLSS.Indicator = dlssSetIndicator
		p.MarkOverride(profile.FieldDLSSIndicator)
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
