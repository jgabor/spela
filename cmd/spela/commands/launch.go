package commands

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/launcher"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

var (
	launchGameID uint64
	launchDryRun bool
)

var LaunchCmd = &cobra.Command{
	Use:   "launch <game>",
	Short: "Launch a game with its profile",
	Long:  "Launch a game applying its profile settings. Can specify game by name or ID.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runLaunch,
}

func init() {
	LaunchCmd.Flags().Uint64Var(&launchGameID, "game-id", 0, "Launch by Steam App ID")
	LaunchCmd.Flags().BoolVar(&launchDryRun, "dry-run", false, "Show what would happen without launching")
}

func runLaunch(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game

	if launchGameID != 0 {
		g = db.GetGame(launchGameID)
	} else {
		g = db.FindGame(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found")
	}

	rawProfile, defaults, effectiveProfile, err := LoadLaunchProfiles(g.AppID)
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	if launchDryRun {
		return RunLaunchDryRun(g, rawProfile, defaults, effectiveProfile, args)
	}

	if launchGameID != 0 || len(args) == 1 {
		return fmt.Errorf("direct Steam URI launch cannot track the game lifetime or cleanup coverage; set the game's Steam launch options to `spela %%command%%` instead")
	}

	l := launcher.New(g)
	l.Profile = effectiveProfile
	PrintLaunchSummary(g, rawProfile, defaults, effectiveProfile, args)
	if err := l.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare launch: %w", err)
	}

	if effectiveProfile != nil {
		fmt.Printf("Launching %s with profile...\n", g.Name)
	} else {
		fmt.Printf("Launching %s (no profile)...\n", g.Name)
	}
	return l.Launch(args)
}

// RunLaunchDryRun prints the launch preparation summary without mutating state.
func RunLaunchDryRun(g *game.Game, rawProfile, defaults, effectiveProfile *profile.Profile, args []string) error {
	PrintLaunchSummary(g, rawProfile, defaults, effectiveProfile, args)
	return nil
}

// LoadLaunchProfiles returns the raw game profile, defaults, and resolved launch profile.
func LoadLaunchProfiles(appID uint64) (*profile.Profile, *profile.Profile, *profile.Profile, error) {
	rawProfile, err := profile.Load(appID)
	if err != nil {
		return nil, nil, nil, err
	}
	defaults, err := profile.LoadDefault()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load default profile: %w", err)
	}
	if rawProfile != nil {
		return rawProfile, defaults, rawProfile.ResolveForApply(defaults), nil
	}
	return nil, defaults, defaults, nil
}

// PrintLaunchSummary shows launch preparation effects before any mutation occurs.
func PrintLaunchSummary(g *game.Game, rawProfile, defaults, effectiveProfile *profile.Profile, args []string) {
	fmt.Printf("%s %s (AppID %d)\n\n", tui.CLIDim("Game:"), tui.CLIPrimary(g.Name), g.AppID)

	launchCmd := fmt.Sprintf("steam steam://rungameid/%d", g.AppID)
	if len(args) > 1 {
		launchCmd = strings.Join(args, " ")
	}
	fmt.Printf("%s %s\n", tui.CLIDim("Command:"), launchCmd)
	if len(args) == 1 {
		fmt.Printf("%s direct Steam URI launch cannot track cleanup coverage; use Steam launch options: spela %%command%%\n", tui.CLIAccent("Launch option:"))
	}

	if !hasProfileSpecificOverrides(rawProfile) {
		fmt.Printf("%s no profile-specific mutation planned\n", tui.CLIDim("Profile:"))
	} else {
		fmt.Printf("%s %d profile-specific override(s) planned\n", tui.CLIDim("Profile:"), countProfileOverrides(rawProfile))
	}

	if effectiveProfile == nil {
		fmt.Printf("\n%s\n", tui.CLIDim("No profile or defaults will affect launch preparation"))
	}

	printImpactSummary(rawProfile, defaults)
	printEnvironmentSummary(effectiveProfile)
	printDLLSummary(g)
	printHardwareSummary(effectiveProfile)
	printOverlaySummary(effectiveProfile)
}

func printEnvironmentSummary(p *profile.Profile) {
	e := env.New()
	if p != nil {
		p.ApplyEnv(e)
	}
	envVars := e.All()
	fmt.Printf("\n%s\n", tui.CLIPrimary("Environment"))
	if len(envVars) == 0 {
		fmt.Printf("  %s\n", tui.CLIDim("no launch environment variables planned"))
		return
	}
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s=%s\n", tui.CLIDim(k), envVars[k])
	}
}

func printHardwareSummary(p *profile.Profile) {
	if p == nil {
		fmt.Printf("\n%s\n", tui.CLIPrimary("Hardware"))
		fmt.Printf("  %s\n", tui.CLIDim("no privileged hardware mutation planned"))
		return
	}
	hasHardware := p.GPU.ClockOffset != 0 || p.GPU.MemoryOffset != 0 || p.GPU.PowerLimit > 0 || p.GPU.FanSpeed > 0 || p.CPU.Governor != "" || p.CPU.SMT != nil
	fmt.Printf("\n%s\n", tui.CLIPrimary("Hardware"))
	if !hasHardware {
		fmt.Printf("  %s\n", tui.CLIDim("no privileged hardware mutation planned"))
		return
	}
	fmt.Printf("  %s\n", tui.CLIDim("privileged changes apply via pkexec and restore on cleanup"))
	if p.GPU.ClockOffset != 0 {
		fmt.Printf("  %s  %d MHz\n", tui.CLIDim("GPU clock offset:"), p.GPU.ClockOffset)
	}
	if p.GPU.MemoryOffset != 0 {
		fmt.Printf("  %s  %d MHz\n", tui.CLIDim("GPU memory offset:"), p.GPU.MemoryOffset)
	}
	if p.GPU.PowerLimit > 0 {
		fmt.Printf("  %s  %d W\n", tui.CLIDim("GPU power limit:"), p.GPU.PowerLimit)
	}
	if p.GPU.FanSpeed > 0 {
		fmt.Printf("  %s  %d%%\n", tui.CLIDim("GPU fan speed:"), p.GPU.FanSpeed)
	}
	if p.CPU.Governor != "" {
		fmt.Printf("  %s  %s\n", tui.CLIDim("CPU governor:"), p.CPU.Governor)
	}
	if p.CPU.SMT != nil {
		fmt.Printf("  %s  %s\n", tui.CLIDim("CPU SMT:"), strconv.FormatBool(*p.CPU.SMT))
	}
}

func printOverlaySummary(p *profile.Profile) {
	fmt.Printf("\n%s\n", tui.CLIPrimary("Overlay"))
	if p != nil && p.Overlay.Enabled {
		fmt.Printf("  %s\n", tui.CLIDim("collector IPC will be created before launch"))
		fmt.Printf("  %s  %s\n", tui.CLIDim("Position:"), profileVal(p.Overlay.Position))
		return
	}
	fmt.Printf("  %s\n", tui.CLIDim("overlay collector not planned"))
}

func printDLLSummary(g *game.Game) {
	fmt.Printf("\n%s\n", tui.CLIPrimary("DLL"))
	fmt.Printf("  %s\n", tui.CLIDim("no launch-time DLL file mutation planned"))
	denied, reason := denylist.IsDenied(g.AppID)
	if denied {
		fmt.Printf("  %s denied (%s)\n", tui.CLIDim("denylist:"), profileVal(reason))
	} else {
		fmt.Printf("  %s allowed\n", tui.CLIDim("denylist:"))
	}
	if dll.BackupExists(g.AppID) {
		fmt.Printf("  %s available\n", tui.CLIDim("backup:"))
	} else {
		fmt.Printf("  %s not created yet\n", tui.CLIDim("backup:"))
	}
	if len(g.DLLs) == 0 {
		return
	}
	dlls := append([]game.DetectedDLL(nil), g.DLLs...)
	sort.Slice(dlls, func(i, j int) bool {
		if dlls[i].Type == dlls[j].Type {
			return dlls[i].Name < dlls[j].Name
		}
		return dlls[i].Type < dlls[j].Type
	})
	for _, detected := range dlls {
		version := profileVal(detected.Version)
		fmt.Printf("  %s %s %s; %s\n", tui.CLIDim(string(detected.Type)+":"), detected.Name, version, dllPathWriteOutcome(detected.Path))
	}
}

func dllPathWriteOutcome(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "path write: unknown (path not accessible)"
	}
	if info.Mode().Perm()&0o222 == 0 {
		return "path write: no write bit"
	}
	return "path write: write bit present"
}

func printImpactSummary(rawProfile, defaults *profile.Profile) {
	sourceProfile := rawProfile
	if sourceProfile == nil {
		sourceProfile = &profile.Profile{}
	}
	explanations, err := sourceProfile.Explain(defaults)
	if err != nil {
		fmt.Printf("\n%s %v\n", tui.CLIAccent("Profile explanation unavailable:"), err)
		return
	}

	groups := map[profile.LaunchImpact][]profile.FieldExplanation{}
	for _, item := range explanations {
		if item.Source != profile.ExplanationSourceOverride && !isActiveSummaryValue(item.Value) {
			continue
		}
		groups[item.Impact] = append(groups[item.Impact], item)
	}

	fmt.Printf("\n%s\n", tui.CLIPrimary("Profile impacts"))
	for _, impact := range []profile.LaunchImpact{
		profile.LaunchImpactCompatibility,
		profile.LaunchImpactEnvironment,
		profile.LaunchImpactGameFile,
		profile.LaunchImpactSystemState,
		profile.LaunchImpactOverlay,
	} {
		items := groups[impact]
		if len(items) == 0 {
			fmt.Printf("  %s %s\n", tui.CLIDim(string(impact)+":"), "none")
			continue
		}
		fmt.Printf("  %s\n", tui.CLIDim(string(impact)+":"))
		for _, item := range items {
			fmt.Printf("    %s=%s source=%s restore=%s\n", item.Field, summaryValue(item.Value), item.Source, item.Restore)
		}
	}
}

func hasProfileSpecificOverrides(p *profile.Profile) bool {
	return countProfileOverrides(p) > 0
}

func countProfileOverrides(p *profile.Profile) int {
	if p == nil {
		return 0
	}
	count := 0
	for _, overridden := range p.Overrides {
		if overridden {
			count++
		}
	}
	return count
}

func isActiveSummaryValue(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	return !rv.IsZero()
}

func summaryValue(v any) string {
	if v == nil {
		return "unset"
	}
	s := fmt.Sprint(v)
	if strings.TrimSpace(s) == "" {
		return "unset"
	}
	return s
}
