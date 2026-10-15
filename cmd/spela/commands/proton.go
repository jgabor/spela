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

func protonCompatibilityNotice(appID uint64) string {
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
	Short: "Make a Proton field follow All games",
	Long: `Make a Proton profile field follow All games again. Valid fields:
  hdr, wayland, ngx_updater, vkd3d_heap.
Subsequent 'proton show' will show the field as coming from All games.`,
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

func runProtonReset(_ *cobra.Command, args []string) error {
	return resetProfileField(args, "proton", "proton", " (valid: hdr, wayland, ngx_updater, vkd3d_heap)")
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

	changes := profileChanges{name: g.Name}

	if protonSetHDR != "" {
		b, err := parseBoolFlag(protonSetHDR)
		if err != nil {
			return fmt.Errorf("--hdr: %w", err)
		}
		changes.set(profile.FieldProtonEnableHDR, b)
	}

	if protonSetWayland != "" {
		b, err := parseBoolFlag(protonSetWayland)
		if err != nil {
			return fmt.Errorf("--wayland: %w", err)
		}
		changes.set(profile.FieldProtonEnableWayland, b)
	}

	if protonSetNGXUpdater != "" {
		b, err := parseBoolFlag(protonSetNGXUpdater)
		if err != nil {
			return fmt.Errorf("--ngx-updater: %w", err)
		}
		changes.set(profile.FieldProtonEnableNGXUpdater, b)
	}

	if protonSetVKD3DHeap != "" {
		b, err := parseBoolFlag(protonSetVKD3DHeap)
		if err != nil {
			return fmt.Errorf("--vkd3d-heap: %w", err)
		}
		changes.set(profile.FieldProtonVKD3DHeap, b)
	}

	if len(changes.mutations) == 0 {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := changes.save(g.AppID); err != nil {
		return err
	}

	fmt.Printf("Updated Proton profile for %s\n", g.Name)
	return nil
}

func runProtonShow(cmd *cobra.Command, args []string) error {
	return runProtonShowWithNotice(args, protonCompatibilityNotice)
}

func runProtonShowWithNotice(args []string, compatibilityNotice func(uint64) string) error {
	snapshot, err := loadProfileSnapshot(args[0])
	if err != nil {
		return err
	}

	if protonShowJSON {
		data, err := json.MarshalIndent(snapshot.resolved.Proton, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Proton profile for %s:\n\n", snapshot.game.Name)
	snapshot.printNoProfileNote()
	if err := printSubsystemFields("proton", snapshot, ""); err != nil {
		return err
	}
	if snapshot.resolved.Proton.VKD3DHeap {
		if notice := compatibilityNotice(snapshot.game.AppID); notice != "" {
			fmt.Printf("    %s\n", tui.CLIDim(notice))
		}
	}

	return nil
}
