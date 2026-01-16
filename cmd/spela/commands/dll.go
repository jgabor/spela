package commands

import (
	"fmt"
	"strconv"

	"github.com/jgabor/spela/internal/game"
	"github.com/spf13/cobra"
)

var DLLCmd = &cobra.Command{
	Use:   "dll",
	Short: "Manage game DLLs",
	Long:  "List, update, and restore DLSS/FSR/XeSS DLLs for games.",
}

var dllListCmd = &cobra.Command{
	Use:   "list [game]",
	Short: "List detected DLLs",
	RunE:  runDLLList,
}

var dllCheckCmd = &cobra.Command{
	Use:   "check-updates",
	Short: "Check for DLL updates",
	RunE:  runDLLCheckUpdates,
}

func init() {
	DLLCmd.AddCommand(dllListCmd)
	DLLCmd.AddCommand(dllCheckCmd)
}

func runDLLList(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var games []*game.Game

	if len(args) > 0 {
		var g *game.Game
		if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
			g = db.GetGame(appID)
		} else {
			g = db.GetGameByName(args[0])
		}
		if g == nil {
			return fmt.Errorf("game not found: %s", args[0])
		}
		games = []*game.Game{g}
	} else {
		games = db.GamesWithDLSS()
	}

	if len(games) == 0 {
		fmt.Println("No games with DLSS/FSR/XeSS DLLs found.")
		return nil
	}

	for _, g := range games {
		fmt.Printf("%s (%d)\n", g.Name, g.AppID)
		for _, d := range g.DLLs {
			version := d.Version
			if version == "" {
				version = "unknown"
			}
			fmt.Printf("  %s: %s\n", d.Name, version)
			fmt.Printf("    %s\n", d.Path)
		}
	}

	return nil
}

func runDLLCheckUpdates(cmd *cobra.Command, args []string) error {
	fmt.Println("DLL update checking not yet implemented.")
	fmt.Println("This feature requires the DLL repository manifest.")
	return nil
}
