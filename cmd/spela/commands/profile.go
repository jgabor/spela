package commands

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
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

var profileCreateCmd = &cobra.Command{
	Use:   "create <game>",
	Short: "Create a profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileCreate,
}

var profileShowJSON bool

var profileShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show a game's profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileShow,
}

var profileResetCmd = &cobra.Command{
	Use:   "reset <game>",
	Short: "Make a game follow All games for every setting",
	Long: `Clear every setting the game has of its own, so it follows All games for
all settings. The profile file stays and keeps its name. A game without a
profile already does.`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileReset,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <game>",
	Short: "Delete a game's profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileDelete,
}

func init() {
	profileShowCmd.Flags().BoolVar(&profileShowJSON, "json", false, "Output as JSON")

	ProfileCmd.AddCommand(profileListCmd)
	ProfileCmd.AddCommand(profileCreateCmd)
	ProfileCmd.AddCommand(profileShowCmd)
	ProfileCmd.AddCommand(profileResetCmd)
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

	type profileEntry struct {
		appID uint64
		name  string
		p     *profile.Profile
	}
	entries := make([]profileEntry, 0, len(profiles))
	for appID, p := range profiles {
		name := strconv.FormatUint(appID, 10)
		if db != nil {
			if g := db.GetGame(appID); g != nil {
				name = g.Name
			}
		}
		entries = append(entries, profileEntry{appID: appID, name: name, p: p})
	}
	slices.SortFunc(entries, func(a, b profileEntry) int {
		return cmp.Compare(a.name, b.name)
	})

	for _, entry := range entries {
		profileName := entry.p.Name
		if profileName == "" {
			profileName = "custom"
		}
		fmt.Printf("%s %s: %s\n", tui.CLIPrimary(entry.name), tui.CLIDim("("+strconv.FormatUint(entry.appID, 10)+")"), tui.CLISecondary(profileName))
	}

	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	created, err := profile.Create(g.AppID, &profile.Profile{Name: g.Name})
	if err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}
	if !created {
		return fmt.Errorf("profile already exists for %s", g.Name)
	}

	fmt.Printf("%s %s\n", tui.CLISuccess("Created profile for"), tui.CLIPrimary(g.Name))
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	snapshot, err := loadProfileSnapshot(args[0])
	if err != nil {
		return err
	}

	if profileShowJSON {
		if !snapshot.exists {
			return fmt.Errorf("no profile for %s", snapshot.game.Name)
		}
		data, err := json.MarshalIndent(snapshot.raw, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Profile for %s:\n\n", tui.CLIPrimary(snapshot.game.Name))
	snapshot.printNoProfileNote()
	for _, subsystem := range subsystemOrder {
		fmt.Printf("%s\n", tui.CLIPrimary(subsystemTitles[subsystem]))
		if err := printSubsystemFields(subsystem, snapshot, "  "); err != nil {
			return err
		}
		fmt.Println()
	}

	return nil
}

func runProfileReset(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	cleared := 0
	exists, err := profile.MutateExisting(g.AppID, func(current, _ *profile.Profile) error {
		for _, overridden := range current.Overrides {
			if overridden {
				cleared++
			}
		}
		if cleared == 0 {
			return nil
		}
		current.ResetAll()
		return nil
	})
	if err != nil {
		return err
	}
	switch {
	case !exists:
		fmt.Printf("No profile for %s; it already follows %s for all settings.\n", g.Name, profile.AllGamesName)
	case cleared == 0:
		fmt.Printf("%s already follows %s for all settings.\n", g.Name, profile.AllGamesName)
	default:
		fmt.Printf("%s now follows %s for all settings (%d cleared).\n", g.Name, profile.AllGamesName, cleared)
	}
	return nil
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	if err := profile.Delete(g.AppID); err != nil {
		return err
	}

	fmt.Printf("%s %s\n", tui.CLISuccess("Deleted profile for"), tui.CLIPrimary(g.Name))
	return nil
}
