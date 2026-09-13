package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
			modal = next
			if got := modal.getConfigValue(option.Key); got != "/tmp/"+option.Key {
				t.Errorf("%s = %q", option.Key, got)
			}
			modal.startPathEditing()
			modal.pathInput.SetValue("")
			next, _ = modal.updatePathEditing(keyMsg("enter"))
			modal = next
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
	if got := stripANSI(modal.renderOptionsBody()); got == "" {
		t.Fatalf("empty inline section:\n%s", got)
	}
}

func TestOptionsModalSupportedKeyAlternatesAndValueProjection(t *testing.T) {
	modal := NewOptionsModal(NewStyles(DefaultTheme, true))
	configuration := config.Default()
	configuration.PreferredDLLSource = ""
	configuration.Theme = ""
	modal.OpenEmbedded(configuration)
	for _, key := range []string{"j", "k", "h", "l"} {
		next, _ := modal.Update(keyMsg(key))
		modal = next
	}
	for _, key := range []string{"preferred_dll_source", "theme", "unknown"} {
		_ = modal.getConfigValue(key)
	}
	modal.setConfigValue("unknown", "ignored")

	for _, key := range []string{"q", "esc"} {
		next, command := modal.Update(keyMsg(key))
		modal = next
		if command != nil {
			t.Fatalf("embedded %s emitted a command", key)
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
	state.SettingsSection = nav.SettingsPaths
	pane.SetState(state)
	pane, _ = pane.UpdateAction(ActionDetailConfirm)
	if !pane.Editing() || !pane.HasModalOpen() {
		t.Fatal("path editor was not reported as modal input")
	}

	pane, _ = pane.UpdateAction(ActionEditCancel)
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
	for _, action := range []KeyAction{ActionDetailConfirm, ActionEditToggle, ActionEditCommit, ActionDetailReset, ActionDetailResetAll, ActionDetailNextItem} {
		next, _ := pane.UpdateAction(action)
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
		{formatFieldValue(&profile.Profile{}, profile.FieldDLSSSRPreset), "(default)"},
		{formatFieldValue(&profile.Profile{DLSS: profile.DLSSSettings{SRPreset: profile.DLSSPresetAuto}}, profile.FieldDLSSSRPreset), "(default)"},
		{formatFieldValue(&profile.Profile{DLSS: profile.DLSSSettings{SRPreset: profile.DLSSPresetK}}, profile.FieldDLSSSRPreset), "K"},
		{formatFieldValue(&profile.Profile{}, profile.FieldGPUPowerMizer), "(default)"},
		{formatFieldValue(&profile.Profile{GPU: profile.GPUSettings{PowerMizer: "max"}}, profile.FieldGPUPowerMizer), "max"},
		{formatFieldValue(&profile.Profile{DLSS: profile.DLSSSettings{SRMode: "quality"}}, profile.FieldDLSSSRMode), "quality"},
		{formatFieldValue(&profile.Profile{}, profile.FieldProtonEnableHDR), "(default)"},
		{formatFieldValue(&profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}, profile.FieldProtonEnableHDR), "true"},
		{formatFieldValue(&profile.Profile{}, profile.FieldCPUSMT), "(default)"},
		{formatFieldValue(&profile.Profile{CPU: profile.CPUSettings{SMT: &valueTrue}}, profile.FieldCPUSMT), "true"},
		{formatFieldValue(&profile.Profile{CPU: profile.CPUSettings{SMT: &valueFalse}}, profile.FieldCPUSMT), "false"},
		{formatFieldValue(&profile.Profile{}, profile.FieldDLSSFGEnabled), "(default)"},
		{formatFieldValue(&profile.Profile{}, profile.FieldGPUClockOffset), "(default)"},
		{formatFieldValue(&profile.Profile{GPU: profile.GPUSettings{ClockOffset: 42}}, profile.FieldGPUClockOffset), "42"},
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
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
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
	database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
	typeInfo := services.KnownDLLTypes()[0]
	cachePath := filepath.Join(root, "cached.dll")
	entry.DLLs[0].Path = filepath.Join(root, "game", entry.DLLs[0].Name)
	entry.InstallDir = filepath.Dir(entry.DLLs[0].Path)
	if err := os.MkdirAll(filepath.Dir(entry.DLLs[0].Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry.DLLs[0].Path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	payloadChecksum := fmt.Sprintf("%x", sha256.Sum256([]byte("new")))
	if err := dll.SaveManifest(&dll.Manifest{UpdatedAt: time.Now(), DLLs: map[string][]dll.DLL{typeInfo.ManifestKey: {{Version: "3.9.0", Filename: entry.DLLs[0].Name, SHA256: payloadChecksum}}}}); err != nil {
		t.Fatal(err)
	}
	saveTestDatabase(t, database)
	request := dll.UpdateRequest{AppID: entry.AppID, DLLType: typeInfo.ManifestKey, Version: "3.9.0", InstalledPath: entry.DLLs[0].Path, CachedOnly: true}
	if result := services.BatchUpdateDLLs([]dll.UpdateRequest{request}); result.Failed != 1 {
		t.Fatal("missing cache file unexpectedly updated a game")
	}
	if err := os.WriteFile(cachePath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	actualCachePath := dll.GetDLLCachePath(typeInfo.ManifestKey, request.Version)
	if err := os.MkdirAll(filepath.Dir(actualCachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(cachePath, actualCachePath); err != nil {
		t.Fatal(err)
	}
	batch := services.BatchUpdateDLLs([]dll.UpdateRequest{request})
	if batch.Failed != 0 || len(batch.Items) != 1 {
		t.Fatalf("batch update = %+v", batch)
	}
	result := batch.Items[0].Result
	if result.Game == nil || len(result.Game.DLLs) != 1 || result.Game.DLLs[0].Path == "" {
		t.Fatalf("updated game DLL = %+v", result.Game)
	}
}
