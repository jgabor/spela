//go:build dev || production || bindings

package gui

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func isolateGUIState(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	return root
}

func saveGUIDatabase(t *testing.T, database *game.Database) {
	t.Helper()
	if _, err := game.Transaction(func(current *game.Database) (bool, error) {
		*current = *database
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAppSerializesDLLOperationThroughSnapshotApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.dll")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := &App{database: &game.Database{Games: map[uint64]*game.Game{1: {AppID: 1}}}}
	updatedOnDisk := make(chan struct{})
	releaseUpdate := make(chan struct{})
	restoreStarted := make(chan struct{})
	updateDone := make(chan error, 1)
	restoreDone := make(chan error, 1)

	go func() {
		updateDone <- app.runDLLOperation(func() (dll.Result, error) {
			if err := os.WriteFile(path, []byte("updated"), 0o644); err != nil {
				return dll.Result{}, err
			}
			close(updatedOnDisk)
			<-releaseUpdate
			return dll.Result{Game: &game.Game{AppID: 1, DLLs: []game.DetectedDLL{{Version: "updated"}}}}, nil
		})
	}()
	<-updatedOnDisk
	go func() {
		restoreDone <- app.runDLLOperation(func() (dll.Result, error) {
			close(restoreStarted)
			if err := os.WriteFile(path, []byte("restored"), 0o644); err != nil {
				return dll.Result{}, err
			}
			return dll.Result{Game: &game.Game{AppID: 1, DLLs: []game.DetectedDLL{{Version: "restored"}}}}, nil
		})
	}()
	select {
	case <-restoreStarted:
		t.Fatal("restore started before the update snapshot was applied")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseUpdate)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	if err := <-restoreDone; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "restored" || app.databaseSnapshot().Games[1].DLLs[0].Version != "restored" {
		t.Fatalf("disk = %q, memory = %+v", data, app.databaseSnapshot().Games[1].DLLs)
	}
}

func TestAppSupportedConfigProfileGameAndNavigationFlows(t *testing.T) {
	isolateGUIState(t)
	app := NewApp()
	if app.GetVersion() != "dev" {
		t.Fatalf("default version = %q", app.GetVersion())
	}
	t.Setenv("SPELA_VERSION", "v1.2.3")
	if app.GetVersion() != "v1.2.3" {
		t.Fatalf("configured version = %q", app.GetVersion())
	}

	configInfo := ConfigInfo{
		LogLevel: "debug", ShaderCache: "/cache", CheckUpdates: false, ShowHints: false,
		RescanOnStartup: false, AutoUpdateDLLs: true, SteamPath: "/steam",
		AdditionalLibraryPaths: []string{"/games"}, DLLCachePath: "/dlls", BackupPath: "/backups",
		DLLManifestURL: "https://example.test/manifest", AutoRefreshManifest: false,
		ManifestRefreshHours: 0, PreferredDLLSource: "github", Theme: "light",
		CompactMode: true, ConfirmDestructive: false,
	}
	if err := app.SaveConfig(configInfo); err != nil {
		t.Fatal(err)
	}
	gotConfig, err := app.GetConfig()
	if err != nil || !reflect.DeepEqual(gotConfig, configInfo) {
		t.Fatalf("config round trip = %+v, %v; want %+v", gotConfig, err, configInfo)
	}
	if err := app.SaveConfigOption("showHints", "true"); err != nil {
		t.Fatal(err)
	}
	if got, err := app.GetConfig(); err != nil || !got.ShowHints {
		t.Fatalf("field-level config save = %+v, %v", got, err)
	}
	if err := app.SaveConfigOption("shaderCache", "/new"); err == nil || err.Error() != "unknown config option: shaderCache" {
		t.Fatalf("hidden GUI option error = %v", err)
	}
	themeChoices := []string{}
	for _, section := range app.GetSettingsCatalog() {
		for _, option := range section.Options {
			if option.Key == "theme" {
				themeChoices = option.Choices
			}
		}
	}
	if !reflect.DeepEqual(themeChoices, []string{"default", "dark", "light"}) {
		t.Fatalf("GUI theme choices = %v", themeChoices)
	}
	for _, test := range []struct {
		name string
		edit func(*ConfigInfo)
		want string
	}{
		{"log level", func(info *ConfigInfo) { info.LogLevel = "verbose" }, "unsupported log level"},
		{"DLL source", func(info *ConfigInfo) { info.PreferredDLLSource = "mirror" }, "unsupported DLL source"},
		{"theme", func(info *ConfigInfo) { info.Theme = "neon" }, "unsupported theme"},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := configInfo
			test.edit(&invalid)
			if err := app.SaveConfig(invalid); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SaveConfig error = %v, want %q", err, test.want)
			}
		})
	}

	if _, err := app.PatchDefaultProfile([]ProfilePatch{{Field: profile.FieldCPUSMT, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.PatchDefaultProfile([]ProfilePatch{{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	if got := app.GetDefaultProfile(); got == nil || got["smt"] != "true" || got["enableHdr"] != true {
		t.Fatalf("default profile projection = %+v", got)
	}
	if _, err := app.PatchProfile(1091500, []ProfilePatch{{Field: profile.FieldCPUSMT, Operation: "set", Value: false}}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.PatchProfile(1091500, []ProfilePatch{{Field: profile.FieldDLSSSRMode, Operation: "set", Value: "quality"}}); err != nil {
		t.Fatal(err)
	}
	if got := app.GetProfile(1091500); got == nil || got["smt"] != "false" || got["srMode"] != "quality" {
		t.Fatalf("game profile projection = %+v", got)
	}

	entry := &game.Game{
		AppID: 1091500, Name: "Cyberpunk 2077", InstallDir: "/games/cp", PrefixPath: "/prefix",
		DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: "/games/cp/nvngx_dlss.dll", Version: "3.8.10", Type: game.DLLTypeDLSS}},
	}
	app.setDatabase(&game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}})
	games := app.GetGames()
	if len(games) != 1 || games[0].Name != entry.Name || !games[0].HasProfile || len(games[0].DLLs) != 1 {
		t.Fatalf("GetGames projection = %+v", games)
	}
	if app.GetGame(0) != nil || app.GetGame(entry.AppID).PrefixPath != "/prefix" {
		t.Fatalf("GetGame projection changed: %+v", app.GetGame(entry.AppID))
	}

	state := app.DefaultNavState()
	state = app.NavSelectDestination(state, int(nav.DestinationMonitor))
	state = app.NavSelectMonitorSection(state, int(nav.MonitorCPU))
	if state.Destination != int(nav.DestinationMonitor) || state.MonitorSection != int(nav.MonitorCPU) {
		t.Fatalf("monitor navigation = %+v", state)
	}
	state = app.NavSelectDestination(state, int(nav.DestinationLibrary))
	state = app.NavSelectScope(state, false, entry.Name)
	state = app.NavSelectAspect(state, int(nav.AspectProfile))
	state = app.NavSelectSubsystem(state, int(nav.SubsystemGPU))
	if state.ScopeGlobal || state.GameName != entry.Name || state.Aspect != int(nav.AspectProfile) || state.Subsystem != int(nav.SubsystemGPU) {
		t.Fatalf("library navigation = %+v", state)
	}
	state = app.NavSelectDestination(state, int(nav.DestinationDLLCatalog))
	state = app.NavSelectDLLSection(state, int(nav.SectionDLLDeployment))
	state = app.NavSelectDestination(state, int(nav.DestinationSettings))
	state = app.NavSelectSettingsSection(state, int(nav.SettingsLogging))
	if state.SettingsSection != int(nav.SettingsLogging) {
		t.Fatalf("settings navigation = %+v", state)
	}
	if destination, ok := app.NavDestinationFromHotkey("3"); !ok || destination != int(nav.DestinationMonitor) {
		t.Fatalf("destination hotkey = %d, %v", destination, ok)
	}
	if _, ok := app.NavDestinationFromHotkey("x"); ok {
		t.Fatal("unknown hotkey unexpectedly resolved")
	}
	if len(app.GetSettingsCatalog()) == 0 || len(app.GetNavContract().DestinationLabels) == 0 {
		t.Fatal("Wails catalogs must expose supported settings and navigation")
	}

	app.shutdown(context.Background())
}

func TestSaveConfigOptionSerializesConcurrentFieldEdits(t *testing.T) {
	isolateGUIState(t)
	app := NewApp()

	type edit struct {
		key   string
		value string
	}
	edits := []edit{{key: "showHints", value: "false"}, {key: "theme", value: "light"}}
	ready := make(chan struct{}, len(edits))
	start := make(chan struct{})
	results := make(chan struct {
		edit edit
		err  error
	}, len(edits))

	var workers sync.WaitGroup
	app.configurationMutex.Lock()
	for _, requested := range edits {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ready <- struct{}{}
			<-start
			results <- struct {
				edit edit
				err  error
			}{edit: requested, err: app.SaveConfigOption(requested.key, requested.value)}
		}()
	}
	for range edits {
		<-ready
	}
	close(start)
	app.configurationMutex.Unlock()
	workers.Wait()
	close(results)

	returned := make(map[string]bool, len(edits))
	for result := range results {
		if result.err != nil {
			t.Errorf("SaveConfigOption(%q, %q) error = %v", result.edit.key, result.edit.value, result.err)
		}
		returned[result.edit.key] = true
	}
	if len(returned) != len(edits) {
		t.Fatalf("completed edits = %v, want both distinct calls", returned)
	}
	got, err := app.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.ShowHints || got.Theme != "light" {
		t.Fatalf("concurrent field edits persisted ShowHints=%v Theme=%q", got.ShowHints, got.Theme)
	}
}

func TestAppStartupScanLogoAndMissingDatabasePaths(t *testing.T) {
	stateRoot := isolateGUIState(t)
	app := NewApp()
	if app.GetGame(1) != nil || len(app.GetGames()) != 0 {
		t.Fatal("empty app exposed games")
	}
	if err := app.LaunchGame(1); !errors.Is(err, ErrDatabaseNotLoaded) {
		t.Fatalf("empty database launch error = %v", err)
	}
	if _, err := app.ListDLLInstallTypes(1); !errors.Is(err, ErrDatabaseNotLoaded) {
		t.Fatalf("empty database DLL types error = %v", err)
	}
	if err := app.InstallDLL(1, "dlss", "3.8.10"); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("install domain error = %v", err)
	}
	if outcome, err := app.UpdateDLLs(1); err != nil || outcome.Failed != 1 || len(outcome.Failures) != 1 {
		t.Fatalf("update domain outcome = %+v, %v", outcome, err)
	}
	if err := app.RestoreDLLs(1); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("restore domain error = %v", err)
	}
	if updates := app.CheckDLLUpdates(1); len(updates) != 0 {
		t.Fatalf("empty database updates = %+v", updates)
	}
	if app.HasDLLBackup(1) {
		t.Fatal("unexpected backup in isolated state")
	}

	db := &game.Database{Games: map[uint64]*game.Game{7: {AppID: 7, Name: "Fixture"}}}
	saveGUIDatabase(t, db)
	app.startup(context.Background())
	if app.GetGame(7) == nil {
		t.Fatal("startup did not load isolated game database")
	}
	app.setDatabase(nil)
	if err := app.ScanGames(); err != nil || app.GetGame(7) == nil {
		t.Fatalf("ScanGames = %v, game %+v", err, app.GetGame(7))
	}

	if logo := app.GetLogo(); logo != "" {
		t.Fatalf("logo outside repository root = %q", logo)
	}
	_, filename, _, _ := runtime.Caller(0)
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	oldDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repositoryRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDirectory) })
	if logo := app.GetLogo(); !strings.HasPrefix(logo, "data:image/png;base64,") {
		t.Fatalf("repository logo projection = %q", logo)
	}
	if stateRoot == "" {
		t.Fatal("isolated state root missing")
	}
}

func TestAppDLLMutationInterleavesSafelyWithReaders(t *testing.T) {
	root := isolateGUIState(t)
	installDirectory := filepath.Join(root, "game")
	if err := os.MkdirAll(installDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
	secondTargetPath := filepath.Join(installDirectory, "nvngx_dlssg.dll")
	if err := os.WriteFile(targetPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondTargetPath, []byte("old frame generation"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := []byte("new")
	checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
	secondPayload := []byte("new frame generation")
	secondChecksum := fmt.Sprintf("%x", sha256.Sum256(secondPayload))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("corrupt")) }))
	defer server.Close()
	if err := dll.SaveManifest(&dll.Manifest{UpdatedAt: time.Now(), DLLs: map[string][]dll.DLL{
		"dlss":  {{Version: "2.0.0", Filename: filepath.Base(targetPath), SHA256: checksum}},
		"dlssg": {{Version: "2.0.0", Filename: filepath.Base(secondTargetPath), URL: server.URL, SHA256: secondChecksum}},
	}}); err != nil {
		t.Fatal(err)
	}
	cachePath := dll.GetDLLCachePath("dlss", "2.0.0")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	secondCachePath := dll.GetDLLCachePath("dlssg", "2.0.0")
	if err := os.MkdirAll(filepath.Dir(secondCachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondCachePath, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := &game.Game{AppID: 1091500, Name: "fixture", InstallDir: installDirectory, DLLs: []game.DetectedDLL{
		{Name: filepath.Base(targetPath), Path: targetPath, Type: game.DLLTypeDLSS, Version: "1.0.0"},
		{Name: filepath.Base(secondTargetPath), Path: secondTargetPath, Type: game.DLLTypeDLSSG, Version: "1.0.0"},
	}}
	database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
	saveGUIDatabase(t, database)
	app := NewApp()
	app.setDatabase(database)

	start := make(chan struct{})
	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			for range 100 {
				_ = app.GetGames()
				_ = app.GetGame(entry.AppID)
			}
		}()
	}
	close(start)
	outcome, err := app.UpdateDLLs(entry.AppID)
	readers.Wait()
	if err != nil || outcome.Updated != 1 || outcome.Failed != 1 || len(outcome.Failures) != 1 {
		t.Fatalf("UpdateDLLs() = %+v, %v", outcome, err)
	}
	if !strings.Contains(outcome.Failures[0].Error, "checksum mismatch") {
		t.Fatalf("failure detail = %+v", outcome.Failures)
	}
	if info := app.GetGame(entry.AppID); info == nil || len(info.DLLs) != 2 {
		t.Fatalf("updated game = %+v", info)
	}
	if data, readErr := os.ReadFile(targetPath); readErr != nil || string(data) != string(payload) {
		t.Fatalf("successful item = %q, %v", data, readErr)
	}
	if data, readErr := os.ReadFile(secondTargetPath); readErr != nil || string(data) != "old frame generation" {
		t.Fatalf("failed item mutated file = %q, %v", data, readErr)
	}
}

func TestAppProjectionAndHostReadBranches(t *testing.T) {
	if profileView(nil, nil, false) != nil || profileFieldSemanticsFromExplanations(nil) != nil {
		t.Fatal("nil profile/semantics projections changed")
	}
	app := NewApp()
	cpuInfo := app.GetCPUInfo()
	if cpuInfo == nil || cpuInfo.Cores <= 0 || cpuInfo.Model == "" {
		t.Fatalf("CPU projection = %+v", cpuInfo)
	}
	if gpuInfo := app.GetGPUInfo(); gpuInfo != nil && gpuInfo.Name == "" {
		t.Fatalf("GPU projection = %+v", gpuInfo)
	}
}

func TestGUIBoundaryProfileFallbackAndErrorContracts(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return nil, nil }
	boundary.loadDefaultProfile = func() (*profile.Profile, error) {
		return &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}, nil
	}
	if info := boundary.getProfile(1); info == nil || info["inheritedFromDefault"] != true || info["enableHdr"] != true {
		t.Fatalf("default fallback profile = %+v", info)
	}
	if info := boundary.getDefaultProfile(); info == nil || info["enableHdr"] != true {
		t.Fatalf("default profile = %+v", info)
	}
	boundary.loadDefaultProfile = func() (*profile.Profile, error) { return nil, errors.New("invalid defaults") }
	if boundary.getProfile(1) != nil || boundary.getDefaultProfile() != nil {
		t.Fatal("default profile load errors did not return nil")
	}
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return nil, errors.New("invalid game profile") }
	if err := boundary.patchGameProfile(1, []ProfilePatch{{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true}}); err == nil || !strings.Contains(err.Error(), "invalid game profile") {
		t.Fatalf("save profile load error = %v", err)
	}
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return nil, nil }
	boundary.loadDefaultProfile = func() (*profile.Profile, error) { return nil, errors.New("invalid defaults") }
	if err := boundary.patchGameProfile(1, []ProfilePatch{{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true}}); err == nil || !strings.Contains(err.Error(), "load default profile") {
		t.Fatalf("save profile defaults error = %v", err)
	}
	if err := boundary.rejectDirectLaunch(1); !errors.Is(err, ErrDatabaseNotLoaded) {
		t.Fatalf("missing database launch error = %v", err)
	}
	boundary.db = &game.Database{Games: map[uint64]*game.Game{}}
	if err := boundary.rejectDirectLaunch(1); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("missing game launch error = %v", err)
	}
}

func TestAppConcurrentProfileTransactionsPreserveEveryCaller(t *testing.T) {
	isolateGUIState(t)
	app := NewApp()
	patches := []ProfilePatch{
		{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true},
		{Field: profile.FieldProtonEnableWayland, Operation: "set", Value: true},
		{Field: profile.FieldDLSSSRMode, Operation: "set", Value: "quality"},
		{Field: profile.FieldDLSSMultiFrame, Operation: "set", Value: float64(0)},
		{Field: profile.FieldGPUClockOffset, Operation: "set", Value: float64(0)},
		{Field: profile.FieldGPUShaderCachePath, Operation: "set", Value: ""},
		{Field: profile.FieldCPUSMT, Operation: "set", Value: nil},
		{Field: profile.FieldOverlayEnabled, Operation: "set", Value: false},
	}
	start := make(chan struct{})
	errors := make(chan error, len(patches))
	var wait sync.WaitGroup
	for _, patch := range patches {
		wait.Add(1)
		go func(patch ProfilePatch) {
			defer wait.Done()
			<-start
			_, err := app.PatchProfile(1091500, []ProfilePatch{patch})
			errors <- err
		}(patch)
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	stored, err := profile.Load(1091500)
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range patches {
		if !stored.IsOverridden(patch.Field) {
			t.Errorf("concurrent patch lost %s", patch.Field)
		}
	}
}
