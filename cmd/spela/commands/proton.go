package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

var protonShowJSON bool

var ProtonCmd = &cobra.Command{
	Use:   "proton",
	Short: "Proton profile settings",
	Long:  "Configure per-game Proton settings (HDR, Wayland, NGX updater).",
}

var (
	protonSetHDR        string
	protonSetWayland    string
	protonSetNGXUpdater string
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

func init() {
	protonSetCmd.Flags().StringVar(&protonSetHDR, "hdr", "", "Enable HDR (true/false)")
	protonSetCmd.Flags().StringVar(&protonSetWayland, "wayland", "", "Enable native Wayland (true/false)")
	protonSetCmd.Flags().StringVar(&protonSetNGXUpdater, "ngx-updater", "", "Enable NGX auto-updater (true/false)")

	protonShowCmd.Flags().BoolVar(&protonShowJSON, "json", false, "Output as JSON")

	ProtonCmd.AddCommand(protonSetCmd)
	ProtonCmd.AddCommand(protonShowCmd)
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
		p.Proton.EnableHDR = protonSetHDR == "true" || protonSetHDR == "1"
		changed = true
	}

	if protonSetWayland != "" {
		p.Proton.EnableWayland = protonSetWayland == "true" || protonSetWayland == "1"
		changed = true
	}

	if protonSetNGXUpdater != "" {
		p.Proton.EnableNGXUpdater = protonSetNGXUpdater == "true" || protonSetNGXUpdater == "1"
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

	if protonShowJSON {
		data, err := json.MarshalIndent(p.Proton, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Proton profile for %s:\n\n", g.Name)
	fmt.Printf("%s  %v\n", tui.CLIDim("HDR:"), p.Proton.EnableHDR)
	fmt.Printf("%s  %v\n", tui.CLIDim("Wayland:"), p.Proton.EnableWayland)
	fmt.Printf("%s  %v\n", tui.CLIDim("NGX updater:"), p.Proton.EnableNGXUpdater)

	return nil
}
