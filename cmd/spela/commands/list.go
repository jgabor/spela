package commands

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jgabor/spela/internal/game"
	"github.com/spf13/cobra"
)

var (
	listWithDLLs bool
	listJSON     bool
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List detected games",
	Long:  "List all games detected in the game database.",
	RunE:  runList,
}

func init() {
	ListCmd.Flags().BoolVar(&listWithDLLs, "with-dlls", false, "Show DLL information")
	ListCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
}

func runList(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	games := db.List()
	if len(games) == 0 {
		fmt.Println("No games found. Run 'spela scan' first.")
		return nil
	}

	sort.Slice(games, func(i, j int) bool {
		return games[i].Name < games[j].Name
	})

	if listJSON {
		data, err := json.MarshalIndent(games, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	for _, g := range games {
		if listWithDLLs && len(g.DLLs) > 0 {
			fmt.Printf("%s (%d)\n", g.Name, g.AppID)
			for _, d := range g.DLLs {
				version := d.Version
				if version == "" {
					version = "unknown"
				}
				fmt.Printf("  %s: %s\n", d.Name, version)
			}
		} else {
			fmt.Printf("%s (%d)\n", g.Name, g.AppID)
		}
	}

	return nil
}
