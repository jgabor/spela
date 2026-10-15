package commands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

func plain(text string) string { return ansiPattern.ReplaceAllString(text, "") }

func seedVocabularyFixture(t *testing.T) {
	t.Helper()
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	defaults := &profile.Profile{
		GPU:    profile.GPUSettings{ClockOffset: 150, FanSpeed: 60, ThreadedOptimization: true},
		Proton: profile.ProtonSettings{EnableWayland: true},
	}
	if err := profile.SaveDefault(defaults); err != nil {
		t.Fatalf("SaveDefault: %v", err)
	}
	game := &profile.Profile{Name: "Cyberpunk 2077"}
	for field, value := range map[string]any{
		profile.FieldProtonEnableHDR:     true,
		profile.FieldProtonEnableWayland: true,
		profile.FieldGPUClockOffset:      100,
	} {
		if err := game.Set(field, value); err != nil {
			t.Fatalf("Set %s: %v", field, err)
		}
	}
	if err := profile.Save(1091500, game); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func showAll(t *testing.T, query string) string {
	t.Helper()
	profileShowJSON = false
	return plain(captureStdout(t, func() {
		ProfileCmd.SetArgs([]string{"show", query})
		if err := ProfileCmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}))
}

// profile show prints every descriptor, labelled with the descriptor label,
// with its effective value and where the value comes from.
func TestProfileShow_PrintsEveryFieldWithValueAndSource(t *testing.T) {
	seedVocabularyFixture(t)
	out := showAll(t, "1091500")

	for _, descriptor := range profile.Fields() {
		if !strings.Contains(out, descriptor.Label) {
			t.Errorf("profile show is missing the label %q", descriptor.Label)
		}
	}
	for _, want := range []string{
		"HDR",
		"On",
		"this game · All games: Off",
		"this game · same as All games",
		"100 MHz",
		"this game · All games: 150 MHz",
		"60%",
		"from All games",
		"system default",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("profile show is missing %q:\n%s", want, out)
		}
	}
	for _, retired := range profile.RetiredWords() {
		if strings.Contains(out, retired) {
			t.Errorf("profile show prints the retired word %q", retired)
		}
	}
}

func TestProfileShow_GameWithoutProfileShowsAllGamesValues(t *testing.T) {
	seedVocabularyFixture(t)
	if err := profile.Delete(1091500); err != nil {
		t.Fatal(err)
	}
	out := showAll(t, "1091500")
	for _, want := range []string{"No profile for Cyberpunk 2077", "150 MHz", "from All games", "system default"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "this game") {
		t.Errorf("a game without a profile owns no value:\n%s", out)
	}
	if profile.Exists(1091500) {
		t.Error("show must not create a profile")
	}
}

func TestProfileShowJSON_StaysCleanWithoutProfile(t *testing.T) {
	seedVocabularyFixture(t)
	if err := profile.Delete(1091500); err != nil {
		t.Fatal(err)
	}
	gpuShowJSON = true
	t.Cleanup(func() { gpuShowJSON = false })
	out := captureStdout(t, func() {
		if err := runGPUShow(nil, []string{"1091500"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "No profile") || !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("--json output must be JSON only, got:\n%s", out)
	}
}

// Every subsystem show command prints the descriptor labels of its fields.
func TestSubsystemShow_UsesDescriptorLabels(t *testing.T) {
	seedVocabularyFixture(t)
	gpuShowJSON, dlssShowJSON, protonShowJSON, cpuShowJSON, overlayShowJSON = false, false, false, false, false
	for subsystem, show := range map[string]func() error{
		"gpu":     func() error { return runGPUShow(nil, []string{"1091500"}) },
		"dlss":    func() error { return runDLSSShow(nil, []string{"1091500"}) },
		"proton":  func() error { return runProtonShowWithNotice([]string{"1091500"}, func(uint64) string { return "" }) },
		"cpu":     func() error { return runCPUShow(nil, []string{"1091500"}) },
		"overlay": func() error { return runOverlayShow(nil, []string{"1091500"}) },
	} {
		out := plain(captureStdout(t, func() {
			if err := show(); err != nil {
				t.Fatalf("%s show: %v", subsystem, err)
			}
		}))
		for _, descriptor := range profile.Fields() {
			if descriptor.Subsystem == subsystem && !strings.Contains(out, descriptor.Label) {
				t.Errorf("%s show is missing the label %q:\n%s", subsystem, descriptor.Label, out)
			}
		}
		for _, retired := range profile.RetiredWords() {
			if strings.Contains(out, retired) {
				t.Errorf("%s show prints the retired word %q", subsystem, retired)
			}
		}
	}
}

func TestProfileReset_LeavesGameFullyInherited(t *testing.T) {
	seedVocabularyFixture(t)
	out := plain(executeSupportedCommand(t, ProfileCmd, "reset", "1091500"))
	if !strings.Contains(out, "now follows All games for all settings (3 cleared)") {
		t.Fatalf("reset output = %q", out)
	}
	loaded, err := profile.Load(1091500)
	if err != nil || loaded == nil {
		t.Fatalf("Load: %v %v", loaded, err)
	}
	if loaded.Name != "Cyberpunk 2077" {
		t.Errorf("reset must keep the name, got %q", loaded.Name)
	}
	for _, field := range profile.AllFields() {
		if loaded.IsOverridden(field) {
			t.Errorf("%s is still overridden after profile reset", field)
		}
	}
	defaults, _ := profile.LoadDefault()
	if resolved := loaded.ResolveForApply(defaults); resolved.GPU.ClockOffset != 150 {
		t.Errorf("clock offset = %d, want the All games value 150", resolved.GPU.ClockOffset)
	}

	again := plain(executeSupportedCommand(t, ProfileCmd, "reset", "1091500"))
	if !strings.Contains(again, "already follows All games for all settings") {
		t.Errorf("second reset output = %q", again)
	}
}

func TestProfileReset_GameWithoutProfileIsNotCreated(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	out := plain(executeSupportedCommand(t, ProfileCmd, "reset", "1091500"))
	if !strings.Contains(out, "No profile for Cyberpunk 2077") {
		t.Errorf("output = %q", out)
	}
	if profile.Exists(1091500) {
		t.Error("profile reset must not create a profile")
	}
}

func TestProfileReset_UnknownGameFails(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	executeExpectedError(t, ProfileCmd, []string{"reset", "Nope"}, "game not found")
}
