//go:build dev || production || bindings

package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

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

func fullProfileInfo() ProfileInfo {
	return ProfileInfo{
		SRMode: "quality", SRPreset: "K", SROverride: true,
		RRMode: "dlaa", RRPreset: "L", RROverride: true,
		FGEnabled: true, FGOverride: true, FGIndicator: true, MultiFrame: 4, Indicator: true,
		ShaderCache: true, ShaderCachePath: "/shader", ThreadedOptimization: true,
		PowerMizer: "prefer_maximum_performance", ClockOffset: 150, MemoryOffset: 500,
		Governor: "performance", SMT: "false",
		EnableHDR: true, EnableWayland: true, EnableNGXUpdater: true, VKD3DHeap: true,
		OverlayEnabled: true, OverlayPosition: "top-right", OverlayShowFPS: true,
		OverlayShowFrametime: true, OverlayShowCPU: true, OverlayShowGPU: true,
		OverlayShowVRAM: true, OverlayToggleKey: "F12",
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

	defaultInfo := fullProfileInfo()
	defaultInfo.SMT = "true"
	if err := app.SaveDefaultProfile(defaultInfo); err != nil {
		t.Fatal(err)
	}
	if got := app.GetDefaultProfile(); got == nil || got.SMT != "true" || !got.EnableHDR {
		t.Fatalf("default profile projection = %+v", got)
	}
	gameInfo := fullProfileInfo()
	if err := app.SaveProfile(1091500, gameInfo); err != nil {
		t.Fatal(err)
	}
	if got := app.GetProfile(1091500); got == nil || got.SMT != "false" || got.SRMode != "quality" {
		t.Fatalf("game profile projection = %+v", got)
	}
	if stringToBoolPtr("") != nil || boolPtrToString(nil) != "" {
		t.Fatal("unset SMT projection changed")
	}

	entry := &game.Game{
		AppID: 1091500, Name: "Cyberpunk 2077", InstallDir: "/games/cp", PrefixPath: "/prefix",
		DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: "/games/cp/nvngx_dlss.dll", Version: "3.8.10", Type: game.DLLTypeDLSS}},
	}
	app.db = &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
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
	for name, operation := range map[string]func() error{
		"install": func() error { return app.InstallDLL(1, "dlss", "3.8.10") },
		"update":  func() error { return app.UpdateDLLs(1) },
		"restore": func() error { return app.RestoreDLLs(1) },
	} {
		if err := operation(); !errors.Is(err, ErrDatabaseNotLoaded) {
			t.Errorf("%s error = %v", name, err)
		}
	}
	if updates := app.CheckDLLUpdates(1); len(updates) != 0 {
		t.Fatalf("empty database updates = %+v", updates)
	}
	if app.HasDLLBackup(1) {
		t.Fatal("unexpected backup in isolated state")
	}

	db := &game.Database{Games: map[uint64]*game.Game{7: {AppID: 7, Name: "Fixture"}}}
	if err := db.Save(); err != nil {
		t.Fatal(err)
	}
	app.startup(context.Background())
	if app.GetGame(7) == nil {
		t.Fatal("startup did not load isolated game database")
	}
	app.db = nil
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

func TestAppParsingProjectionAndHostReadBranches(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		if parsed, err := parseLogLevel(level); err != nil || string(parsed) != level {
			t.Errorf("parseLogLevel(%q) = %q, %v", level, parsed, err)
		}
	}
	for _, source := range []string{"techpowerup", "github"} {
		if parsed, err := parsePreferredDLLSource(source); err != nil || parsed != source {
			t.Errorf("parsePreferredDLLSource(%q) = %q, %v", source, parsed, err)
		}
	}
	for _, theme := range []string{"default", "dark", "light"} {
		if parsed, err := parseTheme(theme); err != nil || parsed != theme {
			t.Errorf("parseTheme(%q) = %q, %v", theme, parsed, err)
		}
	}
	if profileInfoFromProfile(nil, false) != nil || profileFieldSemanticsFromExplanations(nil) != nil {
		t.Fatal("nil profile/semantics projections changed")
	}
	if got := boolPtrToString(stringToBoolPtr("true")); got != "true" {
		t.Fatalf("true SMT round trip = %q", got)
	}
	if got := boolPtrToString(stringToBoolPtr("false")); got != "false" {
		t.Fatalf("false SMT round trip = %q", got)
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
	if info := boundary.getProfile(1); info == nil || !info.InheritedFromDefault || !info.EnableHDR {
		t.Fatalf("default fallback profile = %+v", info)
	}
	if info := boundary.getDefaultProfile(); info == nil || !info.EnableHDR {
		t.Fatalf("default profile = %+v", info)
	}
	boundary.loadDefaultProfile = func() (*profile.Profile, error) { return nil, errors.New("invalid defaults") }
	if boundary.getProfile(1) != nil || boundary.getDefaultProfile() != nil {
		t.Fatal("default profile load errors did not return nil")
	}
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return nil, errors.New("invalid game profile") }
	if err := boundary.saveGameProfile(1, ProfileInfo{}); err == nil || !strings.Contains(err.Error(), "invalid game profile") {
		t.Fatalf("save profile load error = %v", err)
	}
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return nil, nil }
	boundary.loadDefaultProfile = func() (*profile.Profile, error) { return nil, errors.New("invalid defaults") }
	if err := boundary.saveGameProfile(1, ProfileInfo{}); err == nil || !strings.Contains(err.Error(), "load default profile") {
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
