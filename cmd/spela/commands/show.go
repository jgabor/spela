package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

var showJSON bool

var ShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show game details",
	Long:  "Show detailed information about a game including DLLs and profile.",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	ShowCmd.Flags().BoolVar(&showJSON, "json", false, "Output in JSON format")
}

func runShow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game

	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	}

	if g == nil {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	if showJSON {
		data, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("%s  %s\n", tui.CLIDim("Name:"), tui.CLIPrimary(g.Name))
	fmt.Printf("%s %d\n", tui.CLIDim("App ID:"), g.AppID)
	fmt.Printf("%s  %s\n", tui.CLIDim("Install:"), g.InstallDir)
	if g.PrefixPath != "" {
		fmt.Printf("%s  %s\n", tui.CLIDim("Prefix:"), g.PrefixPath)
	}

	if len(g.DLLs) > 0 {
		fmt.Println("\n" + tui.CLISecondary("Detected DLLs:"))
		for _, d := range g.DLLs {
			version := d.Version
			if version == "" {
				version = "unknown"
			}
			fmt.Printf("  %s: %s\n", tui.CLIPrimary(d.Name), tui.CLIAccent(version))
			fmt.Printf("    %s\n", tui.CLIDim(d.Path))
		}
	}

	hasProfile := profile.Exists(g.AppID)
	profileStatus := tui.CLIDim("no")
	if hasProfile {
		profileStatus = tui.CLISuccess("yes")
	}
	fmt.Printf("\n%s %s\n", tui.CLIDim("Profile:"), profileStatus)

	return nil
}
