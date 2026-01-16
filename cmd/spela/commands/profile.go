package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/spf13/cobra"
)

var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage game profiles",
	Long:  "Create, edit, and manage per-game configuration profiles.",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfileList,
}

var (
	profileCreatePreset string
	profileCreateCmd    = &cobra.Command{
		Use:   "create <game>",
		Short: "Create a profile for a game",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileCreate,
	}
)

var profileShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show a game's profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileShow,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <game>",
	Short: "Delete a game's profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileDelete,
}

func init() {
	profileCreateCmd.Flags().StringVar(&profileCreatePreset, "preset", "balanced", "Preset to use (performance, balanced, quality, custom)")

	ProfileCmd.AddCommand(profileListCmd)
	ProfileCmd.AddCommand(profileCreateCmd)
	ProfileCmd.AddCommand(profileShowCmd)
	ProfileCmd.AddCommand(profileDeleteCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	profiles, err := profile.List()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found.")
		return nil
	}

	db, _ := game.LoadDatabase()

	for appID, p := range profiles {
		name := fmt.Sprintf("%d", appID)
		if db != nil {
			if g := db.GetGame(appID); g != nil {
				name = g.Name
			}
		}
		fmt.Printf("%s (%d): %s\n", name, appID, p.Preset)
	}

	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	if profile.Exists(g.AppID) {
		return fmt.Errorf("profile already exists for %s", g.Name)
	}

	p := profile.FromPreset(profile.Preset(profileCreatePreset))
	p.Name = g.Name

	if err := profile.Save(g.AppID, p); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Printf("Created %s profile for %s\n", p.Preset, g.Name)
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("no profile for %s", g.Name)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	if err := profile.Delete(g.AppID); err != nil {
		return err
	}

	fmt.Printf("Deleted profile for %s\n", g.Name)
	return nil
}
