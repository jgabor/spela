package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
)

func executeExpectedError(t *testing.T, command *cobra.Command, args []string, fragment string) {
	t.Helper()
	command.SetArgs(args)
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("%s %v error = %v, want %q", command.Name(), args, err, fragment)
	}
}

func TestProfileSubsystemCommandsRejectUnsupportedGamesAndFields(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_CACHE_HOME", state+"/cache")
	seedGame(t, "Cyberpunk 2077", 1091500)

	for _, test := range []struct {
		command *cobra.Command
		args    []string
		want    string
	}{
		{DLSSCmd, []string{"reset", "Cyberpunk 2077", "unknown"}, "unknown DLSS field"},
		{GPUCmd, []string{"profile-reset", "Cyberpunk 2077", "unknown"}, "unknown GPU field"},
		{CPUCmd, []string{"profile-reset", "Cyberpunk 2077", "unknown"}, "unknown CPU field"},
		{OverlayCmd, []string{"reset", "Cyberpunk 2077", "unknown"}, "unknown overlay field"},
		{ProtonCmd, []string{"reset", "Cyberpunk 2077", "unknown"}, "unknown proton field"},
		{DLSSCmd, []string{"show", "missing"}, "game not found"},
		{GPUCmd, []string{"show", "missing"}, "game not found"},
		{CPUCmd, []string{"show", "missing"}, "game not found"},
		{OverlayCmd, []string{"show", "missing"}, "game not found"},
		{ProtonCmd, []string{"show", "missing"}, "game not found"},
	} {
		executeExpectedError(t, test.command, test.args, test.want)
	}
	for _, test := range []struct {
		command *cobra.Command
		args    []string
		want    string
	}{
		{DLSSCmd, []string{"reset", "Cyberpunk 2077", "sr_mode"}, "already inherited"},
		{GPUCmd, []string{"profile-reset", "Cyberpunk 2077", "clock_offset"}, "already inherited"},
		{CPUCmd, []string{"profile-reset", "Cyberpunk 2077", "governor"}, "already inherited"},
		{OverlayCmd, []string{"reset", "Cyberpunk 2077", "enabled"}, "already inherited"},
		{ProtonCmd, []string{"reset", "Cyberpunk 2077", "hdr"}, "already inherited"},
	} {
		if output := executeSupportedCommand(t, test.command, test.args...); !strings.Contains(output, test.want) {
			t.Errorf("%s %v output missing %q:\n%s", test.command.Name(), test.args, test.want, output)
		}
	}
}

func TestListProfileAndDenylistAlternateSupportedProjections(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_CACHE_HOME", state+"/cache")
	configuration := config.Default()
	configuration.RescanOnStartup = false
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	entry := &game.Game{AppID: 7, Name: "Fixture", DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Version: "3.8.10", Type: game.DLLTypeDLSS}}}
	database := &game.Database{Games: map[uint64]*game.Game{7: entry}}
	if err := database.Save(); err != nil {
		t.Fatal(err)
	}
	if output := executeSupportedCommand(t, ListCmd, "--json", "--with-dlls"); !strings.Contains(output, `"AppID": 7`) {
		t.Fatalf("JSON list output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ProfileCmd, "list"); !strings.Contains(output, "No profiles found") {
		t.Fatalf("empty profiles output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ProfileCmd, "create", "Fixture"); !strings.Contains(output, "Created profile") {
		t.Fatal(output)
	}
	if output := executeSupportedCommand(t, ProfileCmd, "delete", "Fixture"); !strings.Contains(output, "Deleted profile") {
		t.Fatalf("profile delete output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DenylistCmd, "deny", "999", "--reason", "numeric fixture"); !strings.Contains(output, "999") {
		t.Fatalf("numeric deny output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DenylistCmd, "allow", "999"); !strings.Contains(output, "Force-allowed") {
		t.Fatalf("numeric allow output:\n%s", output)
	}
	executeExpectedError(t, DenylistCmd, []string{"check", "not-a-game"}, "game not found")
}

func TestScanCommandExecutesConfiguredSteamLibraryFlow(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", filepath.Join(state, "home"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(state, "cache"))
	steamRoot := filepath.Join(state, "steam")
	steamapps := filepath.Join(steamRoot, "steamapps")
	gameDirectory := filepath.Join(steamapps, "common", "Fixture Game")
	if err := os.MkdirAll(gameDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	libraries := "\"libraryfolders\"\n{\n\"0\"\n{\n\"path\" \"" + steamRoot + "\"\n}\n}\n"
	if err := os.WriteFile(filepath.Join(steamapps, "libraryfolders.vdf"), []byte(libraries), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "\"AppState\"\n{\n\"appid\" \"42\"\n\"name\" \"Fixture Game\"\n\"installdir\" \"Fixture Game\"\n\"StateFlags\" \"4\"\n}\n"
	if err := os.WriteFile(filepath.Join(steamapps, "appmanifest_42.acf"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDirectory, "nvngx_dlss.dll"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	configuration := config.Default()
	configuration.SteamPath = steamRoot
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	if output := executeSupportedCommand(t, ScanCmd); !strings.Contains(output, "Found") || !strings.Contains(output, "1") || !strings.Contains(output, "Fixture Game") || !strings.Contains(output, "unknown") {
		t.Fatalf("scan output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ScanCmd, "--json"); !strings.Contains(output, `"42"`) || !strings.Contains(output, "Fixture Game") {
		t.Fatalf("scan JSON output:\n%s", output)
	}
}

func TestProfileSetValidationAndNoChangeContracts(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	command := &cobra.Command{}
	command.Flags().Bool("fg-indicator", false, "")
	command.Flags().Bool("indicator", false, "")
	dlssSetSRMode, dlssSetSRPreset, dlssSetSRModelPreset = "", "", ""
	dlssSetRRMode, dlssSetRRPreset, dlssSetRROverride = "", "", ""
	dlssSetFGEnabled, dlssSetFGOverride = "", ""
	dlssSetMultiFrame = -1
	t.Cleanup(func() {
		dlssSetSRMode, dlssSetSRPreset, dlssSetSRModelPreset = "", "", ""
		dlssSetRRMode, dlssSetRRPreset, dlssSetRROverride = "", "", ""
		dlssSetFGEnabled, dlssSetFGOverride = "", ""
		dlssSetMultiFrame = -1
	})
	if output := captureStdout(t, func() {
		if err := runDLSSSet(command, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "No changes specified") {
		t.Fatalf("DLSS no-change output:\n%s", output)
	}
	for name, configure := range map[string]func(){
		"rr override":      func() { dlssSetRROverride = "invalid" },
		"frame generation": func() { dlssSetFGEnabled = "invalid" },
		"frame override":   func() { dlssSetFGOverride = "invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			dlssSetRROverride, dlssSetFGEnabled, dlssSetFGOverride = "", "", ""
			configure()
			if err := runDLSSSet(command, []string{"Cyberpunk 2077"}); err == nil || !strings.Contains(err.Error(), "invalid bool") {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
	dlssSetRROverride, dlssSetFGEnabled, dlssSetFGOverride = "", "", ""
	dlssSetSRModelPreset = "k"
	if err := runDLSSSet(command, []string{"Cyberpunk 2077"}); err != nil {
		t.Fatal(err)
	}

	if displayGPUInt(0) != "(default)" || displayGPUInt(5) != "5" || displayGPUPercent(0) != "(auto)" || displayGPUPercent(70) != "70%" || displayGPUString("") != "(default)" || displayGPUString("max") != "max" || displayOrDefault("") != "(default)" || displayOrDefault("value") != "value" {
		t.Fatal("profile display fallbacks changed")
	}
	on, off := true, false
	if displaySMT(nil) != "(default)" || displaySMT(&on) != "on" || displaySMT(&off) != "off" {
		t.Fatal("SMT display contract changed")
	}
}
