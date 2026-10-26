//go:build dev || production || bindings

package gui

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/release"

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
	autoUpdateOnce     sync.Once
	autoUpdateNotice   string
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
	if err := current.Save(); err != nil {
		return err
	}
	current.ApplyLogLevel()
	return nil
}

// ReleaseNotice returns the notice for a newer Spela release, or an empty
// string when Check for updates is off, the build is a development build, no
// newer release is known, or the lookup failed.
func (a *App) ReleaseNotice() string {
	configuration, err := config.Load()
	if err != nil || !configuration.CheckUpdates {
		return ""
	}
	notice, _ := release.Check(context.Background(), a.GetVersion(), time.Now())
	if notice == nil {
		return ""
	}
	return notice.Text()
}

// AutoUpdateDLLs runs the startup DLL update once per process when the
// setting is on, and returns its notice. An empty notice means nothing
// changed, failed, or was skipped.
func (a *App) AutoUpdateDLLs() string {
	a.autoUpdateOnce.Do(func() {
		configuration, err := config.Load()
		if err != nil || !configuration.AutoUpdateDLLs {
			return
		}
		a.dllMutex.Lock()
		defer a.dllMutex.Unlock()
		result := dll.AutoUpdate(game.RunningGames)
		for _, item := range result.Batch.Items {
			a.applyDLLResults(item.Result)
		}
		a.autoUpdateNotice = result.Notice()
	})
	return a.autoUpdateNotice
}

func (a *App) GetVersion() string {
	version := os.Getenv("SPELA_VERSION")
	if version == "" {
		return release.Version
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

type ProfilePatch struct {
	Field     string `json:"field"`
	Operation string `json:"operation"`
	Value     any    `json:"value"`
}

type ProfileFieldSemantics struct {
	Field   string `json:"field"`
	Source  string `json:"source"`
	Impact  string `json:"impact"`
	Restore string `json:"restore"`
	// The remaining fields carry the shared words for the field, so the
	// graphical interface prints exactly what the terminal and the command line do.
	Label       string `json:"label"`
	Description string `json:"description"`
	Origin      string `json:"origin"`
	SourceText  string `json:"sourceText"`
	// Action names what the button on the field does: Follow All games when
	// the game has its own value, Set for this game when it does not; on the
	// All games page Use system default when a value is set there, else Set here.
	Action string `json:"action"`
	// ValueText is the effective value in the shared words (On, Off, none, 150 MHz).
	ValueText string `json:"valueText"`
	// Hint is the unit and range of a free number, or empty. Unit is the
	// unit alone, such as MHz.
	Hint string `json:"hint"`
	Unit string `json:"unit"`
	// OwnValueSuffix, OwnValueCount and OwnValueAction are set on the All games
	// page only: how many games set their own value for the field, in words and
	// as a number, and what the button that applies All games to them says.
	OwnValueSuffix string `json:"ownValueSuffix"`
	OwnValueCount  int    `json:"ownValueCount"`
	OwnValueAction string `json:"ownValueAction"`
}

func profileView(p *profile.Profile, semantics []profile.FieldExplanation, inheritedFromDefault, allGamesScope bool) map[string]any {
	if p == nil {
		return nil
	}
	view := map[string]any{
		"inheritedFromDefault": inheritedFromDefault,
		"semantics":            profileFieldSemanticsFromExplanations(semantics, allGamesScope),
		// clearAll describes the whole-page Follow: its action word, how many
		// settings it would clear, and the sentence that says so.
		"clearAll": map[string]any{
			"action": profile.ClearAllAction(allGamesScope),
			"count":  profile.OwnCount(semantics, allGamesScope),
			"body":   profile.ClearAllBody(allGamesScope, profile.OwnCount(semantics, allGamesScope)),
		},
	}
	for _, descriptor := range profile.Fields() {
		key := profileViewKey(descriptor)
		if key == "" {
			continue
		}
		value, err := profile.ReadField(p, descriptor.Key)
		if err != nil {
			continue
		}
		if descriptor.Kind == profile.PrimitiveOptionalBool {
			if value == nil {
				value = ""
			} else {
				value = fmt.Sprint(value)
			}
		}
		view[key] = value
	}
	return view
}

func profileFieldSemanticsFromExplanations(explanations []profile.FieldExplanation, allGamesScope bool) []ProfileFieldSemantics {
	if len(explanations) == 0 {
		return nil
	}
	items := make([]ProfileFieldSemantics, 0, len(explanations))
	for _, explanation := range explanations {
		descriptor, _ := profile.Field(explanation.Field)
		items = append(items, ProfileFieldSemantics{
			Field:       explanation.Field,
			Source:      string(explanation.Source),
			Impact:      string(explanation.Impact),
			Restore:     string(explanation.Restore),
			Label:       descriptor.Label,
			Description: descriptor.Description,
			Origin:      string(explanation.Origin),
			SourceText:  profile.SourceText(descriptor, explanation),
			Action:      fieldActionText(explanation, allGamesScope),
			ValueText:   profile.ValueText(descriptor, explanation),
			Hint:        profile.NumberHint(descriptor),
			Unit:        descriptor.Unit,
		})
	}
	return items
}

func fieldActionText(explanation profile.FieldExplanation, allGamesScope bool) string {
	switch {
	case allGamesScope && explanation.Origin == profile.OriginSetHere:
		return profile.ActionUseSystemDefault
	case allGamesScope:
		return profile.ActionSetHere
	case explanation.Source == profile.ExplanationSourceOverride:
		return profile.ActionFollowAllGames
	default:
		return profile.ActionSetForThisGame
	}
}

func (a *App) GetProfile(appID uint64) map[string]any {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).profileView(&appID)
}

func (a *App) GetDefaultProfile() map[string]any {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).profileView(nil)
}

func (a *App) PatchProfile(appID uint64, patches []ProfilePatch) (map[string]any, error) {
	boundary := defaultGUIApplicationBoundary(a.databaseSnapshot())
	if err := boundary.patchGameProfile(appID, patches); err != nil {
		return nil, err
	}
	return boundary.profileView(&appID), nil
}

// VKD3DHeapCompatibilityNotice returns a human-readable inline notice
// describing any vkd3d_heap compatibility problem for the given AppID.
// An empty string means the environment is compatible or checks skipped
// cleanly. Mirrors the helper used by the CLI `proton show` command.
func (a *App) VKD3DHeapCompatibilityNotice(appID uint64) string {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).vkd3dHeapCompatibilityNotice(appID)
}

func (a *App) PatchDefaultProfile(patches []ProfilePatch) (map[string]any, error) {
	boundary := defaultGUIApplicationBoundary(a.databaseSnapshot())
	if err := boundary.patchDefaultProfile(patches); err != nil {
		return nil, err
	}
	return boundary.profileView(nil), nil
}

// PreviewApplyToGamesWithOwnValue words the question asked before All games is
// applied to the games that set their own value for the fields: the action, the
// number of games, the sentence that names them, and their app IDs.
func (a *App) PreviewApplyToGamesWithOwnValue(fields []string) (map[string]any, error) {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).previewApplyToOwnValues(fields)
}

// ApplyToGamesWithOwnValue makes the games the question named (appIDs, from the
// preview) follow All games for the fields. It returns the message to show, the
// failure part of it, the games that could not be written, and the refreshed
// All games profile.
func (a *App) ApplyToGamesWithOwnValue(fields []string, appIDs []uint64) (map[string]any, error) {
	return defaultGUIApplicationBoundary(a.databaseSnapshot()).applyToOwnValues(fields, appIDs)
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
