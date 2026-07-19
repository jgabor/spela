//go:build dev || production || bindings

package gui

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	ErrDatabaseNotLoaded = errors.New("game database not loaded")
	ErrGameNotFound      = errors.New("game not found")
)

type App struct {
	ctx                context.Context
	database           *game.Database
	configurationMutex sync.Mutex
	dbMutex            sync.RWMutex
	dllMutex           sync.Mutex
}

// ConfigInfo preserves the Wails-owned source model while sharing Config's fields and tags.
type ConfigInfo config.Config

func (a *App) GetSettingsCatalog() []config.CatalogSection {
	return config.WailsCatalog()
}

func (a *App) GetNavContract() nav.GUIContract {
	return nav.WailsContract()
}

func (a *App) DefaultNavState() nav.GUIState {
	return nav.DefaultGUIState()
}

func (a *App) NavSelectDestination(state nav.GUIState, destination int) nav.GUIState {
	return nav.SelectDestinationGUI(state, destination)
}

func (a *App) NavSelectScope(state nav.GUIState, scopeGlobal bool, gameName string) nav.GUIState {
	return nav.SelectScopeGUI(state, scopeGlobal, gameName)
}

func (a *App) NavSelectAspect(state nav.GUIState, aspect int) nav.GUIState {
	return nav.SelectAspectGUI(state, aspect)
}

func (a *App) NavSelectSubsystem(state nav.GUIState, subsystem int) nav.GUIState {
	return nav.SelectSubsystemGUI(state, subsystem)
}

func (a *App) NavSelectDLLSection(state nav.GUIState, section int) nav.GUIState {
	return nav.SelectDLLSectionGUI(state, section)
}

func (a *App) NavSelectMonitorSection(state nav.GUIState, section int) nav.GUIState {
	return nav.SelectMonitorSectionGUI(state, section)
}

func (a *App) NavSelectSettingsSection(state nav.GUIState, section int) nav.GUIState {
	return nav.SelectSettingsSectionGUI(state, section)
}

func (a *App) NavDestinationFromHotkey(key string) (int, bool) {
	return nav.DestinationFromHotkeyGUI(key)
}

func (a *App) GetConfig() (ConfigInfo, error) {
	a.configurationMutex.Lock()
	defer a.configurationMutex.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return ConfigInfo{}, err
	}
	return ConfigInfo(*cfg), nil
}

func (a *App) SaveConfig(info ConfigInfo) error {
	a.configurationMutex.Lock()
	defer a.configurationMutex.Unlock()

	current, err := config.Load()
	if err != nil {
		return err
	}
	*current = config.Config(info)
	if err := config.Validate(current); err != nil {
		return err
	}
	return current.Save()
}

func (a *App) SaveConfigOption(key, value string) error {
	option := config.OptionByJSONKey(key)
	if option == nil || !option.Visibility.Includes(config.VisibilityGUI) {
		return fmt.Errorf("unknown config option: %s", key)
	}

	a.configurationMutex.Lock()
	defer a.configurationMutex.Unlock()

	current, err := config.Load()
	if err != nil {
		return err
	}
	if err := option.Set(current, value); err != nil {
		return err
	}
	return current.Save()
}

func (a *App) GetVersion() string {
	version := os.Getenv("SPELA_VERSION")
	if version == "" {
		return "dev"
	}
	return version
}

func (a *App) GetLogo() string {
	data, err := os.ReadFile("assets/spela.png")
	if err != nil {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return "data:image/png;base64," + encoded
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	db, err := game.LoadDatabase()
	if err != nil {
		logging.Error("failed to load game database", "error", err)
	}
	a.setDatabase(db)
}

func (a *App) shutdown(_ context.Context) {
	// No cleanup required - database is read-only in memory
}

type GameInfo struct {
	AppID      uint64    `json:"appId"`
	Name       string    `json:"name"`
	InstallDir string    `json:"installDir"`
	PrefixPath string    `json:"prefixPath"`
	DLLs       []DLLInfo `json:"dlls"`
	HasProfile bool      `json:"hasProfile"`
}

type DLLInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	DLLType string `json:"dllType"`
}

func (a *App) GetGames() []GameInfo {
	database := a.databaseSnapshot()
	if database == nil {
		return []GameInfo{}
	}

	var games []GameInfo
	for _, g := range database.List() {
		games = append(games, gameInfoFromGame(g))
	}

	return games
}

func (a *App) GetGame(appID uint64) *GameInfo {
	database := a.databaseSnapshot()
	if database == nil {
		return nil
	}

	g := database.GetGame(appID)
	if g == nil {
		return nil
	}

	info := gameInfoFromGame(g)
	return &info
}

func gameInfoFromGame(g *game.Game) GameInfo {
	info := GameInfo{
		AppID:      g.AppID,
		Name:       g.Name,
		InstallDir: g.InstallDir,
		PrefixPath: g.PrefixPath,
		HasProfile: profile.Exists(g.AppID),
	}

	for _, d := range g.DLLs {
		info.DLLs = append(info.DLLs, DLLInfo{
			Name:    d.Name,
			Path:    d.Path,
			Version: d.Version,
			DLLType: string(d.Type),
		})
	}

	return info
}

type ProfileInfo struct {
	SRMode               string                  `json:"srMode"`
	SRPreset             string                  `json:"srPreset"`
	SROverride           bool                    `json:"srOverride"`
	RRMode               string                  `json:"rrMode"`
	RRPreset             string                  `json:"rrPreset"`
	RROverride           bool                    `json:"rrOverride"`
	FGEnabled            bool                    `json:"fgEnabled"`
	FGOverride           bool                    `json:"fgOverride"`
	FGIndicator          bool                    `json:"fgIndicator"`
	MultiFrame           int                     `json:"multiFrame"`
	Indicator            bool                    `json:"indicator"`
	ShaderCache          bool                    `json:"shaderCache"`
	ShaderCachePath      string                  `json:"shaderCachePath"`
	ThreadedOptimization bool                    `json:"threadedOptimization"`
	PowerMizer           string                  `json:"powerMizer"`
	ClockOffset          int                     `json:"clockOffset"`
	MemoryOffset         int                     `json:"memoryOffset"`
	Governor             string                  `json:"governor"`
	SMT                  string                  `json:"smt"`
	EnableHDR            bool                    `json:"enableHdr"`
	EnableWayland        bool                    `json:"enableWayland"`
	EnableNGXUpdater     bool                    `json:"enableNgxUpdater"`
	VKD3DHeap            bool                    `json:"vkd3dHeap"`
	OverlayEnabled       bool                    `json:"overlayEnabled"`
	OverlayPosition      string                  `json:"overlayPosition"`
	OverlayShowFPS       bool                    `json:"overlayShowFps"`
	OverlayShowFrametime bool                    `json:"overlayShowFrametime"`
	OverlayShowCPU       bool                    `json:"overlayShowCpu"`
	OverlayShowGPU       bool                    `json:"overlayShowGpu"`
	OverlayShowVRAM      bool                    `json:"overlayShowVram"`
	OverlayToggleKey     string                  `json:"overlayToggleKey"`
	InheritedFromDefault bool                    `json:"inheritedFromDefault"`
	Semantics            []ProfileFieldSemantics `json:"semantics"`
}

type ProfileFieldSemantics struct {
	Field   string `json:"field"`
	Source  string `json:"source"`
	Impact  string `json:"impact"`
	Restore string `json:"restore"`
}

func profileInfoFromProfile(p *profile.Profile, inheritedFromDefault bool) *ProfileInfo {
	return profileInfoFromProfileWithSemantics(p, nil, inheritedFromDefault)
}

func profileInfoFromProfileWithSemantics(p *profile.Profile, semantics []profile.FieldExplanation, inheritedFromDefault bool) *ProfileInfo {
	if p == nil {
		return nil
	}

	return &ProfileInfo{
		SRMode:               string(p.DLSS.SRMode),
		SRPreset:             string(p.DLSS.SRPreset),
		SROverride:           p.DLSS.SROverride,
		RRMode:               string(p.DLSS.RRMode),
		RRPreset:             string(p.DLSS.RRPreset),
		RROverride:           p.DLSS.RROverride,
		FGEnabled:            p.DLSS.FGEnabled,
		FGOverride:           p.DLSS.FGOverride,
		FGIndicator:          p.DLSS.FGIndicator,
		MultiFrame:           p.DLSS.MultiFrame,
		Indicator:            p.DLSS.Indicator,
		ShaderCache:          p.GPU.ShaderCache,
		ShaderCachePath:      p.GPU.ShaderCachePath,
		ThreadedOptimization: p.GPU.ThreadedOptimization,
		PowerMizer:           p.GPU.PowerMizer,
		ClockOffset:          p.GPU.ClockOffset,
		MemoryOffset:         p.GPU.MemoryOffset,
		Governor:             p.CPU.Governor,
		SMT:                  boolPtrToString(p.CPU.SMT),
		EnableHDR:            p.Proton.EnableHDR,
		EnableWayland:        p.Proton.EnableWayland,
		EnableNGXUpdater:     p.Proton.EnableNGXUpdater,
		VKD3DHeap:            p.Proton.VKD3DHeap,
		OverlayEnabled:       p.Overlay.Enabled,
		OverlayPosition:      p.Overlay.Position,
		OverlayShowFPS:       p.Overlay.ShowFPS,
		OverlayShowFrametime: p.Overlay.ShowFrametime,
		OverlayShowCPU:       p.Overlay.ShowCPU,
		OverlayShowGPU:       p.Overlay.ShowGPU,
		OverlayShowVRAM:      p.Overlay.ShowVRAM,
		OverlayToggleKey:     p.Overlay.ToggleKey,
		InheritedFromDefault: inheritedFromDefault,
		Semantics:            profileFieldSemanticsFromExplanations(semantics),
	}
}

func profileFieldSemanticsFromExplanations(explanations []profile.FieldExplanation) []ProfileFieldSemantics {
	if len(explanations) == 0 {
		return nil
	}
	items := make([]ProfileFieldSemantics, 0, len(explanations))
	for _, explanation := range explanations {
		items = append(items, ProfileFieldSemantics{
			Field:   explanation.Field,
			Source:  string(explanation.Source),
			Impact:  string(explanation.Impact),
			Restore: string(explanation.Restore),
		})
	}
	return items
}

func profileFromInfo(info ProfileInfo) *profile.Profile {
	return &profile.Profile{
		DLSS: profile.DLSSSettings{
			SRMode:      profile.DLSSMode(info.SRMode),
			SRPreset:    profile.NormalizeSRPreset(info.SRPreset),
			SROverride:  info.SROverride,
			RRMode:      profile.DLSSMode(info.RRMode),
			RRPreset:    profile.DLSSPreset(info.RRPreset),
			RROverride:  info.RROverride,
			FGEnabled:   info.FGEnabled,
			FGOverride:  info.FGOverride,
			FGIndicator: info.FGIndicator,
			MultiFrame:  info.MultiFrame,
			Indicator:   info.Indicator,
		},
		GPU: profile.GPUSettings{
			ShaderCache:          info.ShaderCache,
			ShaderCachePath:      info.ShaderCachePath,
			ThreadedOptimization: info.ThreadedOptimization,
			PowerMizer:           info.PowerMizer,
			ClockOffset:          info.ClockOffset,
			MemoryOffset:         info.MemoryOffset,
		},
		CPU: profile.CPUSettings{
			Governor: info.Governor,
			SMT:      stringToBoolPtr(info.SMT),
		},
		Proton: profile.ProtonSettings{
			EnableHDR:        info.EnableHDR,
			EnableWayland:    info.EnableWayland,
			EnableNGXUpdater: info.EnableNGXUpdater,
			VKD3DHeap:        info.VKD3DHeap,
		},
		Overlay: profile.OverlaySettings{
			Enabled:       info.OverlayEnabled,
			Position:      info.OverlayPosition,
			ShowFPS:       info.OverlayShowFPS,
			ShowFrametime: info.OverlayShowFrametime,
			ShowCPU:       info.OverlayShowCPU,
			ShowGPU:       info.OverlayShowGPU,
			ShowVRAM:      info.OverlayShowVRAM,
			ToggleKey:     info.OverlayToggleKey,
		},
	}
}

func boolPtrToString(b *bool) string {
	if b == nil {
		return ""
	}
	if *b {
		return "true"
	}
	return "false"
}

func stringToBoolPtr(s string) *bool {
	switch s {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	default:
		return nil
	}
}

func (a *App) GetProfile(appID uint64) *ProfileInfo {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).getProfile(appID)
}

func (a *App) GetDefaultProfile() *ProfileInfo {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).getDefaultProfile()
}

func (a *App) SaveProfile(appID uint64, info ProfileInfo) error {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).saveGameProfile(appID, info)
}

// VKD3DHeapCompatibilityNotice returns a human-readable inline notice
// describing any vkd3d_heap compatibility problem for the given AppID.
// An empty string means the environment is compatible or checks skipped
// cleanly. Mirrors the helper used by the CLI `proton show` command.
func (a *App) VKD3DHeapCompatibilityNotice(appID uint64) string {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).vkd3dHeapCompatibilityNotice(appID)
}

func (a *App) SaveDefaultProfile(info ProfileInfo) error {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).saveDefault(info)
}

type GPUInfo struct {
	Name          string  `json:"name"`
	Temperature   int     `json:"temperature"`
	PowerDraw     float64 `json:"powerDraw"`
	PowerLimit    float64 `json:"powerLimit"`
	Utilization   int     `json:"utilization"`
	MemoryUsed    int     `json:"memoryUsed"`
	MemoryTotal   int     `json:"memoryTotal"`
	GraphicsClock int     `json:"graphicsClock"`
	MemoryClock   int     `json:"memoryClock"`
}

func (a *App) GetGPUInfo() *GPUInfo {
	info, err := gpu.GetGPUInfo()
	if err != nil {
		return nil
	}

	metrics, _ := gpu.GetGPUMetrics()

	result := &GPUInfo{
		Name: info["name"],
	}

	if metrics != nil {
		result.Temperature = metrics.Temperature
		result.PowerDraw = metrics.PowerDraw
		result.PowerLimit = metrics.PowerLimit
		result.Utilization = metrics.Utilization
		result.MemoryUsed = metrics.MemoryUsed
		result.MemoryTotal = metrics.MemoryTotal
		result.GraphicsClock = metrics.GraphicsClock
		result.MemoryClock = metrics.MemoryClock
	}

	return result
}

type CPUInfo struct {
	Model                string  `json:"model"`
	Cores                int     `json:"cores"`
	AverageFrequency     int     `json:"averageFrequency"`
	Governor             string  `json:"governor"`
	SMTEnabled           bool    `json:"smtEnabled"`
	UtilizationPercent   float64 `json:"utilizationPercent"`
	MemoryUsedMegabytes  int     `json:"memoryUsedMegabytes"`
	MemoryTotalMegabytes int     `json:"memoryTotalMegabytes"`
}

func (a *App) GetCPUInfo() *CPUInfo {
	info, err := cpu.GetCPUInfo()
	if err != nil {
		return nil
	}

	metrics, _ := cpu.GetCPUMetrics()

	result := &CPUInfo{
		Model: info["model"],
		Cores: cpu.GetCPUCount(),
	}

	if metrics != nil {
		result.AverageFrequency = metrics.AverageFrequency
		result.Governor = string(metrics.Governor)
		result.SMTEnabled = metrics.SMTEnabled
		result.UtilizationPercent = metrics.Utilization
		result.MemoryUsedMegabytes = metrics.RAMUsedMB
		result.MemoryTotalMegabytes = metrics.RAMTotalMB
	}

	return result
}

func (a *App) ScanGames() error {
	a.dllMutex.Lock()
	defer a.dllMutex.Unlock()
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}
	a.setDatabase(db)
	return nil
}

type DLLUpdateInfo struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
}

type DLLUpdateOutcome struct {
	Updated   int                `json:"updated"`
	Unchanged int                `json:"unchanged"`
	Failed    int                `json:"failed"`
	Failures  []DLLUpdateFailure `json:"failures"`
}

type DLLUpdateFailure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

func (a *App) CheckDLLUpdates(appID uint64) []DLLUpdateInfo {
	return newGUIApplicationBoundary(a.databaseSnapshot(), a.emitDLLProgress).checkDLLUpdates(appID)
}

func (a *App) ListDLLInstallTypes(appID uint64) ([]string, error) {
	return newGUIApplicationBoundary(a.databaseSnapshot(), a.emitDLLProgress).listDLLInstallTypes(appID)
}

func (a *App) ListDLLVersions(dllType string) ([]string, error) {
	return newGUIApplicationBoundary(a.databaseSnapshot(), a.emitDLLProgress).listDLLVersions(dllType)
}

func (a *App) emitDLLProgress(stage string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "dll:progress", stage)
}

func (a *App) InstallDLL(appID uint64, dllType, version string) error {
	return a.runDLLOperation(func() (dll.Result, error) {
		return newGUIApplicationBoundary(a.databaseSnapshot(), a.emitDLLProgress).installDLLVersion(appID, dllType, version)
	})
}

func (a *App) UpdateDLLs(appID uint64) (DLLUpdateOutcome, error) {
	a.dllMutex.Lock()
	defer a.dllMutex.Unlock()
	batch, err := newGUIApplicationBoundary(nil, a.emitDLLProgress).updateDLLs(appID)
	outcome := DLLUpdateOutcome{Updated: batch.Updated, Unchanged: batch.Unchanged, Failed: batch.Failed}
	for _, item := range batch.Items {
		a.applyDLLResults(item.Result)
		if item.Err != nil {
			outcome.Failures = append(outcome.Failures, DLLUpdateFailure{Path: item.Path, Error: item.Err.Error()})
		}
	}
	return outcome, err
}

func (a *App) RestoreDLLs(appID uint64) error {
	return a.runDLLOperation(func() (dll.Result, error) {
		return newGUIApplicationBoundary(a.databaseSnapshot(), a.emitDLLProgress).restoreDLLs(appID)
	})
}

func (a *App) HasDLLBackup(appID uint64) bool {
	return dll.BackupExists(appID)
}

func (a *App) LaunchGame(appID uint64) error {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).rejectDirectLaunch(appID)
}

func (a *App) setDatabase(database *game.Database) {
	a.dbMutex.Lock()
	a.database = database
	a.dbMutex.Unlock()
}

func (a *App) databaseSnapshot() *game.Database {
	a.dbMutex.RLock()
	defer a.dbMutex.RUnlock()
	if a.database == nil {
		return nil
	}
	clone := &game.Database{Games: make(map[uint64]*game.Game, len(a.database.Games)), UpdatedAt: a.database.UpdatedAt}
	for appID, entry := range a.database.Games {
		gameClone := *entry
		gameClone.DLLs = append([]game.DetectedDLL(nil), entry.DLLs...)
		clone.Games[appID] = &gameClone
	}
	return clone
}

func (a *App) applyDLLResults(results ...dll.Result) {
	a.dbMutex.Lock()
	defer a.dbMutex.Unlock()
	if a.database == nil {
		return
	}
	for _, result := range results {
		if result.Game != nil {
			a.database.Games[result.Game.AppID] = result.Game
		}
	}
}

func (a *App) runDLLOperation(operation func() (dll.Result, error)) error {
	a.dllMutex.Lock()
	defer a.dllMutex.Unlock()
	result, err := operation()
	a.applyDLLResults(result)
	return err
}
