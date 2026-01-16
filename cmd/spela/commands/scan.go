package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jgabor/spela/internal/steam"
	"github.com/spf13/cobra"
)

var scanJSON bool

var ScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan Steam libraries for games",
	Long:  "Scan all Steam library folders for installed games and detect DLSS/FSR/XeSS DLLs.",
	RunE:  runScan,
}

func init() {
	ScanCmd.Flags().BoolVar(&scanJSON, "json", false, "Output in JSON format")
}

func runScan(cmd *cobra.Command, args []string) error {
	steamPath := steam.FindSteamPath()
	if steamPath == "" {
		return fmt.Errorf("could not find Steam installation")
	}

	db, err := steam.ScanAllLibraries()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if err := db.Save(); err != nil {
		return fmt.Errorf("failed to save game database: %w", err)
	}

	if scanJSON {
		data, err := json.MarshalIndent(db.Games, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Found %d games\n", len(db.Games))
	gamesWithDLSS := db.GamesWithDLSS()
	if len(gamesWithDLSS) > 0 {
		fmt.Printf("Games with DLSS/FSR/XeSS: %d\n", len(gamesWithDLSS))
		for _, g := range gamesWithDLSS {
			fmt.Printf("  - %s (%d)\n", g.Name, g.AppID)
			for _, d := range g.DLLs {
				version := d.Version
				if version == "" {
					version = "unknown"
				}
				fmt.Printf("      %s: %s\n", d.Name, version)
			}
		}
	}

	return nil
}
