package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
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

	fmt.Printf("Name:        %s\n", g.Name)
	fmt.Printf("App ID:      %d\n", g.AppID)
	fmt.Printf("Install Dir: %s\n", g.InstallDir)
	if g.PrefixPath != "" {
		fmt.Printf("Prefix:      %s\n", g.PrefixPath)
	}

	if len(g.DLLs) > 0 {
		fmt.Println("\nDetected DLLs:")
		for _, d := range g.DLLs {
			version := d.Version
			if version == "" {
				version = "unknown"
			}
			fmt.Printf("  %s: %s\n", d.Name, version)
			fmt.Printf("    Path: %s\n", d.Path)
		}
	}

	hasProfile := profile.Exists(g.AppID)
	fmt.Printf("\nProfile:     %v\n", hasProfile)

	return nil
}
