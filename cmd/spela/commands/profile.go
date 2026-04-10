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

	if profile.Exists(g.AppID) {
		return fmt.Errorf("profile already exists for %s", g.Name)
	}

	p := &profile.Profile{Name: g.Name}

	if err := profile.Save(g.AppID, p); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Printf("%s %s\n", tui.CLISuccess("Created profile for"), tui.CLIPrimary(g.Name))
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
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
		return fmt.Errorf("no profile for %s", g.Name)
	}

	if profileShowJSON {
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Profile for %s:\n", tui.CLIPrimary(g.Name))

	// DLSS
	fmt.Printf("\n%s\n", tui.CLIPrimary("DLSS"))
	fmt.Printf("  %s  %s\n", tui.CLIDim("SR mode:"), profileVal(string(p.DLSS.SRMode)))
	fmt.Printf("  %s  %s\n", tui.CLIDim("SR preset:"), profileVal(string(p.DLSS.SRPreset)))
	fmt.Printf("  %s  %s\n", tui.CLIDim("SR model preset:"), profileVal(string(p.DLSS.SRModelPreset)))
	fmt.Printf("  %s  %v\n", tui.CLIDim("SR override:"), p.DLSS.SROverride)
	fmt.Printf("  %s  %s\n", tui.CLIDim("RR mode:"), profileVal(string(p.DLSS.RRMode)))
	fmt.Printf("  %s  %s\n", tui.CLIDim("RR preset:"), profileVal(string(p.DLSS.RRPreset)))
	fmt.Printf("  %s  %v\n", tui.CLIDim("RR override:"), p.DLSS.RROverride)
	fmt.Printf("  %s  %v\n", tui.CLIDim("Frame generation:"), p.DLSS.FGEnabled)
	fmt.Printf("  %s  %v\n", tui.CLIDim("FG override:"), p.DLSS.FGOverride)
	fmt.Printf("  %s  %d\n", tui.CLIDim("Multi-frame:"), p.DLSS.MultiFrame)

	// GPU
	fmt.Printf("\n%s\n", tui.CLIPrimary("GPU"))
	fmt.Printf("  %s  %v\n", tui.CLIDim("Shader cache:"), p.GPU.ShaderCache)
	fmt.Printf("  %s  %s\n", tui.CLIDim("Shader cache path:"), profileVal(p.GPU.ShaderCachePath))
	fmt.Printf("  %s  %v\n", tui.CLIDim("Threaded opt:"), p.GPU.ThreadedOptimization)
	fmt.Printf("  %s  %d\n", tui.CLIDim("Clock offset:"), p.GPU.ClockOffset)
	fmt.Printf("  %s  %d\n", tui.CLIDim("Memory offset:"), p.GPU.MemoryOffset)
	fmt.Printf("  %s  %s\n", tui.CLIDim("Power limit:"), profileInt(p.GPU.PowerLimit, "W"))
	fmt.Printf("  %s  %s\n", tui.CLIDim("Fan speed:"), profilePercent(p.GPU.FanSpeed))
	fmt.Printf("  %s  %s\n", tui.CLIDim("PowerMizer:"), profileVal(p.GPU.PowerMizer))

	// CPU
	fmt.Printf("\n%s\n", tui.CLIPrimary("CPU"))
	fmt.Printf("  %s  %s\n", tui.CLIDim("Governor:"), profileVal(p.CPU.Governor))
	smt := "(default)"
	if p.CPU.SMT != nil {
		smt = strconv.FormatBool(*p.CPU.SMT)
	}
	fmt.Printf("  %s  %s\n", tui.CLIDim("SMT:"), smt)
	fmt.Printf("  %s  %s\n", tui.CLIDim("Affinity:"), profileVal(p.CPU.Affinity))

	// Proton
	fmt.Printf("\n%s\n", tui.CLIPrimary("Proton"))
	fmt.Printf("  %s  %v\n", tui.CLIDim("HDR:"), p.Proton.EnableHDR)
	fmt.Printf("  %s  %v\n", tui.CLIDim("Wayland:"), p.Proton.EnableWayland)
	fmt.Printf("  %s  %v\n", tui.CLIDim("NGX updater:"), p.Proton.EnableNGXUpdater)

	// Overlay
	fmt.Printf("\n%s\n", tui.CLIPrimary("Overlay"))
	fmt.Printf("  %s  %v\n", tui.CLIDim("Enabled:"), p.Overlay.Enabled)
	fmt.Printf("  %s  %s\n", tui.CLIDim("Position:"), profileVal(p.Overlay.Position))
	fmt.Printf("  %s  %v\n", tui.CLIDim("Show FPS:"), p.Overlay.ShowFPS)
	fmt.Printf("  %s  %v\n", tui.CLIDim("Show frametime:"), p.Overlay.ShowFrametime)
	fmt.Printf("  %s  %v\n", tui.CLIDim("Show CPU:"), p.Overlay.ShowCPU)
	fmt.Printf("  %s  %v\n", tui.CLIDim("Show GPU:"), p.Overlay.ShowGPU)
	fmt.Printf("  %s  %v\n", tui.CLIDim("Show VRAM:"), p.Overlay.ShowVRAM)
	fmt.Printf("  %s  %s\n", tui.CLIDim("Toggle key:"), profileVal(p.Overlay.ToggleKey))

	return nil
}

func profileVal(s string) string {
	if s == "" {
		return "(default)"
	}
	return s
}

func profilePercent(v int) string {
	if v == 0 {
		return "(auto)"
	}
	return strconv.Itoa(v) + "%"
}

func profileInt(v int, unit string) string {
	if v == 0 {
		return "(default)"
	}
	return strconv.Itoa(v) + unit
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
