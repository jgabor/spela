package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func TestOptionsModalPathEditingAndBoundaryContracts(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	modal := NewOptionsModal(styles)
	if got := modal.getConfigValue("theme"); got != "" {
		t.Fatalf("nil config value = %q", got)
	}
	modal.setConfigValue("theme", "dark")
	modal.startPathEditing()

	configuration := config.Default()
	modal.OpenEmbedded(configuration)
	for sectionIndex, section := range modal.sections {
		for optionIndex, option := range section.Options {
			if option.Kind != config.KindPath {
				continue
			}
			modal.sectionCursor, modal.optionCursor = sectionIndex, optionIndex
			modal.startPathEditing()
			if !modal.editingPath {
				t.Fatalf("%s did not enter path editing", option.Key)
			}
			modal.pathInput.SetValue("/tmp/" + option.Key)
			next, _ := modal.updatePathEditing(keyMsg("enter"))
			modal = *next.(*OptionsModalModel)
			if got := modal.getConfigValue(option.Key); got != "/tmp/"+option.Key {
				t.Errorf("%s = %q", option.Key, got)
			}
			modal.startPathEditing()
			modal.pathInput.SetValue("")
			next, _ = modal.updatePathEditing(keyMsg("enter"))
			modal = *next.(*OptionsModalModel)
			if got := modal.getConfigValue(option.Key); got != "(default)" {
				t.Errorf("reset %s = %q", option.Key, got)
			}
		}
	}

	modal.sectionCursor = len(modal.sections)
	modal.cycleValue(1)
	modal.sectionCursor = 0
	modal.optionCursor = len(modal.sections[0].Options)
	modal.cycleValue(1)
	modal.sections = []config.Section{{Title: "Empty"}}
	modal.sectionCursor, modal.optionCursor = 0, 0
	modal.moveCursor(1)
	modal.startPathEditing()
	modal.cycleValue(1)
	modal.SyncNavSection(nav.SettingsSection(100))
	if got := stripANSI(modal.ViewInline()); got == "" {
		t.Fatalf("empty inline section:\n%s", got)
	}
}

func TestOptionsModalSupportedKeyAlternatesAndValueProjection(t *testing.T) {
	modal := NewOptionsModal(NewStyles(DefaultTheme, true))
	configuration := config.Default()
	configuration.PreferredDLLSource = ""
	configuration.Theme = ""
	modal.Open(configuration)
	for _, key := range []string{"j", "k", "h", "l"} {
		next, _ := modal.Update(keyMsg(key))
		modal = *next.(*OptionsModalModel)
	}
	for _, key := range []string{"preferred_dll_source", "theme", "unknown"} {
		_ = modal.getConfigValue(key)
	}
	modal.setConfigValue("unknown", "ignored")

	modal.Open(config.Default())
	next, command := modal.Update(keyMsg("q"))
	modal = *next.(*OptionsModalModel)
	if modal.Visible() || command == nil {
		t.Fatal("modal q did not cancel")
	}
	modal.OpenEmbedded(config.Default())
	for _, key := range []string{"q", "esc"} {
		next, command = modal.Update(keyMsg(key))
		modal = *next.(*OptionsModalModel)
		if command != nil || !modal.Visible() {
			t.Fatalf("embedded %s closed settings", key)
		}
	}
}

func TestResourcePaneSupportedDestinationsScopesAndDefaultMutations(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", stateRoot+"/config")
	t.Setenv("XDG_CACHE_HOME", stateRoot+"/cache")
	t.Setenv("XDG_DATA_HOME", stateRoot+"/data")
	styles := NewStyles(DefaultTheme, true)
	entry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	content := testContent(entry)
	pane := newResourcePane(styles, content)
	pane.refreshDefaultsDetail()
	services := testServices()
	services.LoadDefaultProfile = func() (*profile.Profile, error) {
		return &profile.Profile{DLSS: profile.DLSSSettings{SRMode: profile.DLSSModeQuality}}, errors.New("stale cache")
	}
	pane.setServices(services)
	pane.SetSize(100, 30)
	state := nav.DefaultState()
	pane.BindNavState(&state)

	for _, aspect := range []nav.Aspect{nav.AspectOverview, nav.AspectDLLs, nav.AspectProfile, nav.Aspect(99)} {
		state = state.SelectScope(nav.Scope{Kind: nav.ScopeGlobal}).SelectAspect(aspect)
		view := stripANSI(pane.View(true))
		if view == "" {
			t.Errorf("global aspect %v rendered empty", aspect)
		}
	}
	pane.loadGameScope(entry)
	for _, aspect := range []nav.Aspect{nav.AspectOverview, nav.AspectDLLs, nav.AspectProfile} {
		state = pane.State().SelectAspect(aspect)
		pane.SetState(state)
		if view := stripANSI(pane.View(false)); view == "" {
			t.Errorf("game aspect %v rendered empty", aspect)
		}
		pane, _ = pane.Update(keyMsg("down"))
	}

	state = pane.State().SelectDestination(nav.DestinationSettings)
	pane.SetState(state)
	pane.settings.OpenEmbedded(config.Default())
	if view := stripANSI(pane.View(true)); !strings.Contains(view, "Settings") {
		t.Fatalf("settings pane:\n%s", view)
	}
	pane, _ = pane.Update(keyMsg("down"))
	pane.settings.editingPath = true
	if !pane.HasModalOpen() {
		t.Fatal("path editor was not reported as modal input")
	}

	state = pane.State().SelectDestination(nav.DestinationMonitor)
	pane.SetState(state)
	if pane.View(false) == "" {
		t.Fatal("monitor destination rendered empty")
	}
	if next, command := pane.Update(keyMsg("down")); command != nil || next.contentModel() != nil || next.HasModalOpen() {
		t.Fatal("monitor routed unsupported content input")
	}

	pane.loadGlobalScope()
	state = pane.State().SelectDestination(nav.DestinationLibrary).SelectAspect(nav.AspectProfile)
	pane.SetState(state)
	for _, key := range []string{"left", "h", "right", "l", "r", "R", "down"} {
		next, _ := pane.Update(keyMsg(key))
		pane = next
	}
	if pane.contentModel() != nil {
		t.Fatal("global scope exposed game content")
	}

	unbound := newResourcePane(styles, content)
	unbound.loadGlobalScope()
	unbound.loadGameScope(entry)
	if unbound.saveDefaultProfile() != nil || unbound.View(false) == "" {
		t.Fatal("unbound resource pane fallback changed")
	}
}

func TestProfileDisplaySupportedDefaultsAndOverrides(t *testing.T) {
	valueTrue, valueFalse := true, false
	checks := []struct{ got, want string }{
		{srPresetValue(""), "default"},
		{srPresetValue(profile.DLSSPresetAuto), "auto"},
		{srPresetValue(profile.DLSSPresetK), "K"},
		{formatFieldValue(&profile.Profile{}, profile.FieldGPUPowerMizer), "(default)"},
		{formatFieldValue(&profile.Profile{GPU: profile.GPUSettings{PowerMizer: "max"}}, profile.FieldGPUPowerMizer), "max"},
		{displayValue("default"), "(default)"},
		{displayValue("quality"), "quality"},
		{displayBool(false), "(default)"},
		{displayBool(true), "true"},
		{displayBoolPtr(nil), "(default)"},
		{displayBoolPtr(&valueTrue), "true"},
		{displayBoolPtr(&valueFalse), "false"},
		{formatFieldValue(&profile.Profile{}, profile.FieldDLSSFGEnabled), "(default)"},
		{displayInt(0), "(default)"},
		{displayInt(42), "42"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("display value = %q, want %q", check.got, check.want)
		}
	}
}

func TestDefaultServicesCachedDLLMutationAndNoticeContracts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	services := DefaultServices()
	if configuration, err := services.LoadConfig(); err != nil || configuration == nil {
		t.Fatalf("default config = %+v, %v", configuration, err)
	}
	if len(services.KnownDLLTypes()) == 0 {
		t.Fatal("default services returned no known DLL types")
	}
	_ = services.VKD3DNotice(1091500)

	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.8.0"))
	entry.AppID = 42
	typeInfo := services.KnownDLLTypes()[0]
	cachePath := filepath.Join(root, "cached.dll")
	entry.DLLs[0].Path = filepath.Join(root, "game", entry.DLLs[0].Name)
	if err := os.MkdirAll(filepath.Dir(entry.DLLs[0].Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry.DLLs[0].Path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := defaultUpdateCachedDLL(DLLUpdateRequest{Game: entry, TypeInfo: typeInfo, LatestVersion: "3.9.0", InstalledName: entry.DLLs[0].Name}); err == nil {
		t.Fatal("missing cache file unexpectedly updated a game")
	}
	if err := os.WriteFile(cachePath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := DLLUpdateRequest{Game: entry, TypeInfo: typeInfo, LatestVersion: "3.9.0", InstalledName: entry.DLLs[0].Name}
	actualCachePath := dll.GetDLLCachePath(typeInfo.ManifestKey, request.LatestVersion)
	if err := os.MkdirAll(filepath.Dir(actualCachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(cachePath, actualCachePath); err != nil {
		t.Fatal(err)
	}
	if err := defaultUpdateCachedDLL(request); err != nil {
		t.Fatal(err)
	}
	if entry.DLLs[0].Version != "3.9.0" {
		t.Fatalf("updated game DLL = %+v", entry.DLLs[0])
	}
}
