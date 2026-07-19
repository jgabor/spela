package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
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

var dllUpdateCmd = &cobra.Command{
	Use:   "update <game> <dll-type>",
	Short: "Update a DLL to the latest version",
	Long:  "Download and install the latest version of a DLL for a game.",
	Args:  cobra.ExactArgs(2),
	RunE:  runDLLUpdate,
}

var dllRestoreCmd = &cobra.Command{
	Use:   "restore <game>",
	Short: "Restore original DLLs from backup",
	Args:  cobra.ExactArgs(1),
	RunE:  runDLLRestore,
}

func init() {
	DLLCmd.AddCommand(dllListCmd)
	DLLCmd.AddCommand(dllCheckCmd)
	DLLCmd.AddCommand(dllUpdateCmd)
	DLLCmd.AddCommand(dllRestoreCmd)
}

func runDLLList(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var games []*game.Game

	if len(args) > 0 {
		g := db.FindGame(args[0])
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

func runDLLCheckUpdates(cmd *cobra.Command, _ []string) error {
	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	games := db.GamesWithDLSS()
	if len(games) == 0 {
		fmt.Println("No games with DLSS/FSR/XeSS DLLs found.")
		return nil
	}

	hasUpdates := false
	for _, g := range games {
		gameHasUpdates := false
		var updates []string

		for _, d := range g.DLLs {
			dllType := strings.ToLower(string(d.Type))
			latest := manifest.GetLatestDLL(dllType)
			if latest == nil {
				continue
			}

			if d.Version == "" || dll.IsNewer(d.Version, latest.Version) {
				gameHasUpdates = true
				hasUpdates = true
				current := d.Version
				if current == "" {
					current = "unknown"
				}
				updates = append(updates, fmt.Sprintf("  %s: %s -> %s", d.Name, current, latest.Version))
			}
		}

		if gameHasUpdates {
			fmt.Printf("%s (%d)\n", g.Name, g.AppID)
			for _, u := range updates {
				fmt.Println(u)
			}
		}
	}

	if !hasUpdates {
		fmt.Println("All DLLs are up to date.")
	}

	return nil
}

func runDLLUpdate(cmd *cobra.Command, args []string) error {
	gameArg := args[0]
	dllType := strings.ToLower(args[1])

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	g := db.FindGame(gameArg)
	if g == nil {
		return fmt.Errorf("game not found: %s", gameArg)
	}

	version := ""
	downloadStarted := false
	batch := dll.UpdateGame(g.AppID, dllType, func(event dll.ProgressEvent) {
		if event.Version != "" {
			version = event.Version
		}
		if event.Stage != dll.StageDownloading {
			return
		}
		if !downloadStarted {
			fmt.Printf("Downloading %s %s...\n", dllType, event.Version)
			downloadStarted = true
		}
		if event.Downloaded > 0 && event.Total > 0 {
			fmt.Printf("\rDownloading: %.1f%%", float64(event.Downloaded)/float64(event.Total)*100)
		} else if event.Downloaded > 0 {
			fmt.Printf("\rDownloading: %d bytes", event.Downloaded)
		}
	})
	if downloadStarted {
		fmt.Println()
	}
	if len(batch.Items) == 0 {
		return fmt.Errorf("game does not have a %s DLL", dllType)
	}
	var failures []error
	for _, item := range batch.Items {
		if item.Err != nil {
			fmt.Printf("Failed %s: %v\n", item.Path, item.Err)
			failures = append(failures, item.Err)
		}
	}
	if batch.Failed > 0 {
		fmt.Printf("DLL update result: %d updated, %d unchanged, %d failed\n", batch.Updated, batch.Unchanged, batch.Failed)
		return fmt.Errorf("%d DLL update(s) failed: %w", batch.Failed, errors.Join(failures...))
	}
	if batch.Updated == 0 {
		fmt.Printf("%s DLLs are already at the latest version\n", dllType)
		fmt.Printf("DLL update result: 0 updated, %d unchanged, 0 failed\n", batch.Unchanged)
		return nil
	}
	if batch.Updated > 1 {
		fmt.Printf("Updated %d %s DLLs to version %s\n", batch.Updated, dllType, version)
		fmt.Printf("DLL update result: %d updated, %d unchanged, 0 failed\n", batch.Updated, batch.Unchanged)
		return nil
	}
	fmt.Printf("Updated %s DLL to version %s\n", dllType, version)
	fmt.Printf("DLL update result: 1 updated, %d unchanged, 0 failed\n", batch.Unchanged)
	return nil
}

func runDLLRestore(cmd *cobra.Command, args []string) error {
	gameArg := args[0]

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	g := db.FindGame(gameArg)
	if g == nil {
		return fmt.Errorf("game not found: %s", gameArg)
	}

	if _, err := dll.Restore(g.AppID, nil); err != nil {
		return err
	}

	fmt.Printf("Restored original DLLs for %s\n", g.Name)
	return nil
}
