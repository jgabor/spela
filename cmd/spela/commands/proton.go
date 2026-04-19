package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
	"github.com/jgabor/spela/internal/steam"
	"github.com/jgabor/spela/internal/tui"
)

var protonShowJSON bool

// protonCompatibilityNotice is indirected so tests can swap in a deterministic
// stub. The default wires the production resolver + NVML driver probe.
var protonCompatibilityNotice = func(appID uint64) string {
	cfg, _ := config.Load()
	steamRoot := ""
	if cfg != nil {
		steamRoot = cfg.SteamPath
	}
	if steamRoot == "" {
		steamRoot = steam.FindSteamPath()
	}
	return proton.CompatibilityNotice(appID, proton.NoticeDeps{
		SteamRoot:         steamRoot,
		ResolveForAppID:   proton.ResolveForAppID,
		SupportsVKD3DHeap: proton.SupportsVKD3DHeap,
		DriverVersion:     gpu.DriverVersionString,
	})
}

var ProtonCmd = &cobra.Command{
	Use:   "proton",
	Short: "Proton profile settings",
	Long:  "Configure per-game Proton settings (HDR, Wayland, NGX updater, VKD3D heap).",
}

var (
	protonSetHDR        string
	protonSetWayland    string
	protonSetNGXUpdater string
	protonSetVKD3DHeap  string
)

var protonSetCmd = &cobra.Command{
	Use:   "set <game>",
	Short: "Set Proton profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runProtonSet,
}

var protonShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show Proton profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runProtonShow,
}

var protonResetCmd = &cobra.Command{
	Use:   "reset <game> <field>",
	Short: "Reset a Proton field to inherit from defaults",
	Long: `Reset a Proton profile field back to inherited. Valid fields:
  hdr, wayland, ngx_updater, vkd3d_heap.
Subsequent 'proton show' will mark the field as (inherited).`,
	Args: cobra.ExactArgs(2),
	RunE: runProtonReset,
}

func init() {
	protonSetCmd.Flags().StringVar(&protonSetHDR, "hdr", "", "Enable HDR (true/false)")
	protonSetCmd.Flags().StringVar(&protonSetWayland, "wayland", "", "Enable native Wayland (true/false)")
	protonSetCmd.Flags().StringVar(&protonSetNGXUpdater, "ngx-updater", "", "Enable NGX auto-updater (true/false)")
	protonSetCmd.Flags().StringVar(&protonSetVKD3DHeap, "vkd3d-heap", "", "Enable VKD3D descriptor heap path (true/false)")

	protonShowCmd.Flags().BoolVar(&protonShowJSON, "json", false, "Output as JSON")

	ProtonCmd.AddCommand(protonSetCmd)
	ProtonCmd.AddCommand(protonShowCmd)
	ProtonCmd.AddCommand(protonResetCmd)
}

// protonFieldAliases maps the short CLI names users type at the command line
// to the canonical dot-path keys used by the inheritance layer.
var protonFieldAliases = map[string]string{
	"hdr":                profile.FieldProtonEnableHDR,
	"wayland":            profile.FieldProtonEnableWayland,
	"ngx_updater":        profile.FieldProtonEnableNGXUpdater,
	"ngx-updater":        profile.FieldProtonEnableNGXUpdater,
	"vkd3d_heap":         profile.FieldProtonVKD3DHeap,
	"vkd3d-heap":         profile.FieldProtonVKD3DHeap,
	"enable_hdr":         profile.FieldProtonEnableHDR,
	"enable_wayland":     profile.FieldProtonEnableWayland,
	"enable_ngx_updater": profile.FieldProtonEnableNGXUpdater,
}

func runProtonReset(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	key, ok := protonFieldAliases[args[1]]
	if !ok {
		return fmt.Errorf("unknown proton field %q (valid: hdr, wayland, ngx_updater, vkd3d_heap)", args[1])
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

func runProtonSet(cmd *cobra.Command, args []string) error {
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

	if protonSetHDR != "" {
		b, err := parseBoolFlag(protonSetHDR)
		if err != nil {
			return fmt.Errorf("--hdr: %w", err)
		}
		p.Proton.EnableHDR = b
		p.MarkOverride(profile.FieldProtonEnableHDR)
		changed = true
	}

	if protonSetWayland != "" {
		b, err := parseBoolFlag(protonSetWayland)
		if err != nil {
			return fmt.Errorf("--wayland: %w", err)
		}
		p.Proton.EnableWayland = b
		p.MarkOverride(profile.FieldProtonEnableWayland)
		changed = true
	}

	if protonSetNGXUpdater != "" {
		b, err := parseBoolFlag(protonSetNGXUpdater)
		if err != nil {
			return fmt.Errorf("--ngx-updater: %w", err)
		}
		p.Proton.EnableNGXUpdater = b
		p.MarkOverride(profile.FieldProtonEnableNGXUpdater)
		changed = true
	}

	if protonSetVKD3DHeap != "" {
		b, err := parseBoolFlag(protonSetVKD3DHeap)
		if err != nil {
			return fmt.Errorf("--vkd3d-heap: %w", err)
		}
		p.Proton.VKD3DHeap = b
		p.MarkOverride(profile.FieldProtonVKD3DHeap)
		changed = true
	}

	if !changed {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := profile.Save(g.AppID, p); err != nil {
		return err
	}

	fmt.Printf("Updated Proton profile for %s\n", g.Name)
	return nil
}

func runProtonShow(cmd *cobra.Command, args []string) error {
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

	if protonShowJSON {
		data, err := json.MarshalIndent(resolved.Proton, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Proton profile for %s:\n\n", g.Name)
	fmt.Println(renderField("HDR:", profile.FieldProtonEnableHDR, p, resolved.Proton.EnableHDR))
	fmt.Println(renderField("Wayland:", profile.FieldProtonEnableWayland, p, resolved.Proton.EnableWayland))
	fmt.Println(renderField("NGX updater:", profile.FieldProtonEnableNGXUpdater, p, resolved.Proton.EnableNGXUpdater))
	fmt.Println(renderField("VKD3D heap:", profile.FieldProtonVKD3DHeap, p, resolved.Proton.VKD3DHeap))
	if resolved.Proton.VKD3DHeap {
		if notice := protonCompatibilityNotice(g.AppID); notice != "" {
			fmt.Printf("    %s\n", tui.CLIDim(notice))
		}
	}

	return nil
}

// parseBoolFlag parses the human-facing bool forms accepted by `proton set`
// (true/false/1/0, case-insensitive). Unknown values return an error so
// callers can surface a usage hint rather than silently defaulting.
func parseBoolFlag(raw string) (bool, error) {
	switch raw {
	case "true", "True", "TRUE", "1", "yes", "on":
		return true, nil
	case "false", "False", "FALSE", "0", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("invalid bool %q (use true/false)", raw)
}
