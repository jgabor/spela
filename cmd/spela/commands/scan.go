package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/steam"
	"github.com/jgabor/spela/internal/tui"
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	db, err := steam.Rescan(cfg)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if scanJSON {
		data, err := json.MarshalIndent(db.Games, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Found %s games\n", tui.CLIPrimary(strconv.Itoa(len(db.Games))))
	gamesWithDLSS := db.GamesWithDLSS()
	if len(gamesWithDLSS) > 0 {
		fmt.Printf("Games with DLSS/FSR/XeSS: %s\n", tui.CLIAccent(strconv.Itoa(len(gamesWithDLSS))))
		for _, g := range gamesWithDLSS {
			fmt.Printf("  - %s %s\n", tui.CLIPrimary(g.Name), tui.CLIDim("("+strconv.FormatUint(g.AppID, 10)+")"))
			for _, d := range g.DLLs {
				version := d.Version
				if version == "" {
					version = "unknown"
				}
				fmt.Printf("      %s: %s\n", tui.CLISecondary(d.Name), tui.CLIAccent(version))
			}
		}
	}

	return nil
}
