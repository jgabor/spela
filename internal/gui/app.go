//go:build dev || production || bindings

package gui

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/launcher"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
	"github.com/jgabor/spela/internal/steam"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	ErrDatabaseNotLoaded = errors.New("game database not loaded")
	ErrGameNotFound      = errors.New("game not found")
)

type App struct {
	ctx context.Context
	db  *game.Database
}

type ConfigInfo struct {
	LogLevel               string   `json:"logLevel"`
	ShaderCache            string   `json:"shaderCache"`
	CheckUpdates           bool     `json:"checkUpdates"`
	ShowHints              bool     `json:"showHints"`
	RescanOnStartup        bool     `json:"rescanOnStartup"`
	AutoUpdateDLLs         bool     `json:"autoUpdateDLLs"`
	SteamPath              string   `json:"steamPath"`
	AdditionalLibraryPaths []string `json:"additionalLibraryPaths"`
	DLLCachePath           string   `json:"dllCachePath"`
	BackupPath             string   `json:"backupPath"`
	DLLManifestURL         string   `json:"dllManifestURL"`
	AutoRefreshManifest    bool     `json:"autoRefreshManifest"`
	ManifestRefreshHours   int      `json:"manifestRefreshHours"`
	PreferredDLLSource     string   `json:"preferredDLLSource"`
	Theme                  string   `json:"theme"`
	CompactMode            bool     `json:"compactMode"`
	ConfirmDestructive     bool     `json:"confirmDestructive"`
}

func (a *App) GetConfig() (ConfigInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return ConfigInfo{}, err
	}
	return configInfoFromConfig(cfg), nil
}

func (a *App) SaveConfig(info ConfigInfo) error {
	current, err := config.Load()
	if err != nil {
		return err
	}
	if err := applyConfigInfo(current, info); err != nil {
		return err
	}
	return current.Save()
}

func configInfoFromConfig(cfg *config.Config) ConfigInfo {
	return ConfigInfo{
		LogLevel:               string(cfg.LogLevel),
		ShaderCache:            cfg.ShaderCache,
		CheckUpdates:           cfg.CheckUpdates,
		ShowHints:              cfg.ShowHints,
		RescanOnStartup:        cfg.RescanOnStartup,
		AutoUpdateDLLs:         cfg.AutoUpdateDLLs,
		SteamPath:              cfg.SteamPath,
		AdditionalLibraryPaths: cfg.AdditionalLibraryPaths,
		DLLCachePath:           cfg.DLLCachePath,
		BackupPath:             cfg.BackupPath,
		DLLManifestURL:         cfg.DLLManifestURL,
		AutoRefreshManifest:    cfg.AutoRefreshManifest,
		ManifestRefreshHours:   cfg.ManifestRefreshHours,
		PreferredDLLSource:     cfg.PreferredDLLSource,
		Theme:                  cfg.Theme,
		CompactMode:            cfg.CompactMode,
		ConfirmDestructive:     cfg.ConfirmDestructive,
	}
}

func applyConfigInfo(cfg *config.Config, info ConfigInfo) error {
	logLevel, err := parseLogLevel(info.LogLevel)
	if err != nil {
		return err
	}
	preferredDLLSource, err := parsePreferredDLLSource(info.PreferredDLLSource)
	if err != nil {
		return err
	}
	theme, err := parseTheme(info.Theme)
	if err != nil {
		return err
	}
	cfg.LogLevel = logLevel
	cfg.ShaderCache = info.ShaderCache
	cfg.CheckUpdates = info.CheckUpdates
	cfg.ShowHints = info.ShowHints
	cfg.RescanOnStartup = info.RescanOnStartup
	cfg.AutoUpdateDLLs = info.AutoUpdateDLLs
	cfg.SteamPath = info.SteamPath
	cfg.AdditionalLibraryPaths = info.AdditionalLibraryPaths
	cfg.DLLCachePath = info.DLLCachePath
	cfg.BackupPath = info.BackupPath
	cfg.DLLManifestURL = info.DLLManifestURL
	cfg.AutoRefreshManifest = info.AutoRefreshManifest
	cfg.ManifestRefreshHours = info.ManifestRefreshHours
	cfg.PreferredDLLSource = preferredDLLSource
	cfg.Theme = theme
	cfg.CompactMode = info.CompactMode
	cfg.ConfirmDestructive = info.ConfirmDestructive
	return nil
}

func parseLogLevel(level string) (config.LogLevel, error) {
	switch level {
	case string(config.LogLevelDebug):
		return config.LogLevelDebug, nil
	case string(config.LogLevelInfo):
		return config.LogLevelInfo, nil
	case string(config.LogLevelWarn):
		return config.LogLevelWarn, nil
	case string(config.LogLevelError):
		return config.LogLevelError, nil
	default:
		return "", fmt.Errorf("unsupported log level: %s", level)
	}
}

func parsePreferredDLLSource(source string) (string, error) {
	switch source {
	case "techpowerup", "github":
		return source, nil
	default:
		return "", fmt.Errorf("unsupported DLL source: %s", source)
	}
}

func parseTheme(theme string) (string, error) {
	switch theme {
	case "default", "dark", "light":
		return theme, nil
	default:
		return "", fmt.Errorf("unsupported theme: %s", theme)
	}
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
		slog.Error("failed to load game database", "error", err)
	}
	a.db = db
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
	if a.db == nil {
		return []GameInfo{}
	}

	var games []GameInfo
	for _, g := range a.db.List() {
		games = append(games, gameInfoFromGame(g))
	}

	return games
}

func (a *App) GetGame(appID uint64) *GameInfo {
	if a.db == nil {
		return nil
	}

	g := a.db.GetGame(appID)
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
	SRMode               string `json:"srMode"`
	SRPreset             string `json:"srPreset"`
	SRModelPreset        string `json:"srModelPreset"`
	SROverride           bool   `json:"srOverride"`
	RRMode               string `json:"rrMode"`
	RRPreset             string `json:"rrPreset"`
	RROverride           bool   `json:"rrOverride"`
	FGEnabled            bool   `json:"fgEnabled"`
	FGOverride           bool   `json:"fgOverride"`
	FGIndicator          bool   `json:"fgIndicator"`
	MultiFrame           int    `json:"multiFrame"`
	Indicator            bool   `json:"indicator"`
	ShaderCache          bool   `json:"shaderCache"`
	ShaderCachePath      string `json:"shaderCachePath"`
	ThreadedOptimization bool   `json:"threadedOptimization"`
	PowerMizer           string `json:"powerMizer"`
	ClockOffset          int    `json:"clockOffset"`
	MemoryOffset         int    `json:"memoryOffset"`
	Governor             string `json:"governor"`
	SMT                  string `json:"smt"`
	EnableHDR            bool   `json:"enableHdr"`
	EnableWayland        bool   `json:"enableWayland"`
	EnableNGXUpdater     bool   `json:"enableNgxUpdater"`
	VKD3DHeap            bool   `json:"vkd3dHeap"`
	InheritedFromDefault bool `json:"inheritedFromDefault"`
}

func profileInfoFromProfile(p *profile.Profile, inheritedFromDefault bool) *ProfileInfo {
	if p == nil {
		return nil
	}

	return &ProfileInfo{
		SRMode:               string(p.DLSS.SRMode),
		SRPreset:             string(p.DLSS.SRPreset),
		SRModelPreset:        string(p.DLSS.SRModelPreset),
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
		InheritedFromDefault: inheritedFromDefault,
	}
}

func profileFromInfo(info ProfileInfo) *profile.Profile {
	return &profile.Profile{
		DLSS: profile.DLSSSettings{
			SRMode:        profile.DLSSMode(info.SRMode),
			SRPreset:      profile.DLSSPreset(info.SRPreset),
			SRModelPreset: profile.DLSSModelPreset(info.SRModelPreset),
			SROverride:    info.SROverride,
			RRMode:        profile.DLSSMode(info.RRMode),
			RRPreset:      profile.DLSSPreset(info.RRPreset),
			RROverride:    info.RROverride,
			FGEnabled:     info.FGEnabled,
			FGOverride:    info.FGOverride,
			FGIndicator:   info.FGIndicator,
			MultiFrame:    info.MultiFrame,
			Indicator:     info.Indicator,
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
	perGameProfile, err := profile.Load(appID)
	if err != nil {
		return nil
	}
	if perGameProfile != nil {
		return profileInfoFromProfile(perGameProfile, false)
	}

	defaultProfile, err := profile.LoadDefault()
	if err != nil {
		return nil
	}
	if defaultProfile == nil {
		return nil
	}
	return profileInfoFromProfile(defaultProfile, true)
}

func (a *App) GetDefaultProfile() *ProfileInfo {
	p, err := profile.LoadDefault()
	if err != nil {
		return nil
	}
	return profileInfoFromProfile(p, false)
}

func (a *App) SaveProfile(appID uint64, info ProfileInfo) error {
	return profile.Save(appID, profileFromInfo(info))
}

// VKD3DHeapCompatibilityNotice returns a human-readable inline notice
// describing any vkd3d_heap compatibility problem for the given AppID.
// An empty string means the environment is compatible or checks skipped
// cleanly. Mirrors the helper used by the CLI `proton show` command.
func (a *App) VKD3DHeapCompatibilityNotice(appID uint64) string {
	cfg, _ := config.Load()
	steamRoot := ""
	if cfg != nil {
		steamRoot = cfg.SteamPath
	}
	if steamRoot == "" {
		steamRoot = steam.FindSteamPath()
	}
	return proton.CompatibilityNotice(appID, proton.NoticeDeps{
		SteamRoot:         steamRoot,
		ResolveForAppID:   proton.ResolveForAppID,
		SupportsVKD3DHeap: proton.SupportsVKD3DHeap,
		DriverVersion:     gpu.DriverVersionString,
	})
}

func (a *App) SaveDefaultProfile(info ProfileInfo) error {
	return profile.SaveDefault(profileFromInfo(info))
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
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}
	a.db = db
	return nil
}

type DLLUpdateInfo struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
}

func (a *App) CheckDLLUpdates(appID uint64) []DLLUpdateInfo {
	if a.db == nil {
		return []DLLUpdateInfo{}
	}

	g := a.db.GetGame(appID)
	if g == nil {
		return []DLLUpdateInfo{}
	}

	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		slog.Debug("failed to get DLL manifest", "error", err)
		return []DLLUpdateInfo{}
	}

	var updates []DLLUpdateInfo
	for _, d := range g.DLLs {
		info := DLLUpdateInfo{
			Name:           d.Name,
			CurrentVersion: d.Version,
		}

		latest := manifest.GetLatestDLL(d.Name)
		if latest != nil {
			info.LatestVersion = latest.Version
			info.HasUpdate = latest.Version != d.Version
		}

		updates = append(updates, info)
	}

	return updates
}

func (a *App) ListDLLInstallTypes(appID uint64) ([]string, error) {
	if a.db == nil {
		return nil, ErrDatabaseNotLoaded
	}

	g := a.db.GetGame(appID)
	if g == nil {
		return nil, fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return nil, err
	}

	validTypes := make(map[string]bool, len(g.DLLs))
	for _, d := range g.DLLs {
		validTypes[string(d.Type)] = true
	}

	allTypes := manifest.ListDLLNames()
	filtered := make([]string, 0, len(allTypes))
	for _, t := range allTypes {
		if len(manifest.DLLs[t]) == 0 {
			continue
		}
		if len(validTypes) > 0 && !validTypes[t] {
			continue
		}
		filtered = append(filtered, t)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no supported DLL types detected for this game")
	}

	return filtered, nil
}

func (a *App) ListDLLVersions(dllType string) ([]string, error) {
	if dllType == "" {
		return nil, fmt.Errorf("dll type is required")
	}

	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return nil, err
	}

	versions, ok := manifest.DLLs[dllType]
	if !ok {
		return nil, fmt.Errorf("no versions found for %s", dllType)
	}

	results := make([]string, 0, len(versions))
	for _, entry := range versions {
		results = append(results, entry.Version)
	}

	return results, nil
}

func (a *App) emitDLLProgress(stage string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "dll:progress", stage)
}

func ensureDLLCached(target *dll.DLL, dllName string) (string, error) {
	cachePath := dll.GetDLLCachePath(dllName, target.Version)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}
	return dll.DownloadDLLWithProgress(target, dllName, nil)
}

func (a *App) InstallDLL(appID uint64, dllType, version string) error {
	defer a.emitDLLProgress("")

	if a.db == nil {
		return ErrDatabaseNotLoaded
	}
	if dllType == "" {
		return fmt.Errorf("dll type is required")
	}

	g := a.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	a.emitDLLProgress("Resolving manifest")
	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return fmt.Errorf("load DLL manifest: %w", err)
	}

	targetVersion := version
	if targetVersion == "" {
		targetVersion = "latest"
	}

	var targetDLL *dll.DLL
	if targetVersion == "latest" {
		targetDLL = manifest.GetLatestDLL(dllType)
	} else {
		targetDLL = manifest.GetDLLVersion(dllType, targetVersion)
	}
	if targetDLL == nil {
		return fmt.Errorf("no version available for %s", dllType)
	}

	a.emitDLLProgress(fmt.Sprintf("Downloading %s %s", dllType, targetVersion))
	cachePath, err := ensureDLLCached(targetDLL, dllType)
	if err != nil {
		return fmt.Errorf("download %s: %w", targetVersion, err)
	}

	gameDLLs := dll.GameDLLsFromDetected(g.DLLs)

	a.emitDLLProgress(fmt.Sprintf("Installing %s", targetDLL.Filename))
	if err := dll.InstallDLL(g.AppID, g.Name, g.InstallDir, gameDLLs, targetDLL.Filename, cachePath); err != nil {
		return fmt.Errorf("install DLL: %w", err)
	}

	a.emitDLLProgress("Scanning install directory")
	detected, err := dll.ScanDirectory(g.InstallDir)
	if err != nil {
		return fmt.Errorf("scan install directory: %w", err)
	}

	g.DLLs = detected
	g.ScannedAt = time.Now()
	a.emitDLLProgress("Saving database")
	if err := a.db.Save(); err != nil {
		return fmt.Errorf("save game database after install: %w", err)
	}
	return nil
}

func (a *App) UpdateDLLs(appID uint64) error {
	defer a.emitDLLProgress("")

	if a.db == nil {
		return ErrDatabaseNotLoaded
	}

	g := a.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	a.emitDLLProgress("Resolving manifest")
	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return fmt.Errorf("load DLL manifest: %w", err)
	}

	gameDLLs := dll.GameDLLsFromDetected(g.DLLs)

	for _, d := range g.DLLs {
		latest := manifest.GetLatestDLL(d.Name)
		if latest == nil || latest.Version == d.Version {
			continue
		}

		a.emitDLLProgress(fmt.Sprintf("Downloading %s %s", d.Name, latest.Version))
		cachePath, err := ensureDLLCached(latest, d.Name)
		if err != nil {
			return fmt.Errorf("download %s: %w", d.Name, err)
		}

		a.emitDLLProgress(fmt.Sprintf("Swapping %s", d.Name))
		if err := dll.SwapDLL(appID, g.Name, gameDLLs, d.Name, cachePath); err != nil {
			return fmt.Errorf("swap %s: %w", d.Name, err)
		}
	}

	a.emitDLLProgress("Scanning install directory")
	detected, err := dll.ScanDirectory(g.InstallDir)
	if err != nil {
		slog.Warn("scan after update failed", "appID", appID, "error", err)
	} else {
		g.DLLs = detected
		g.ScannedAt = time.Now()
	}
	a.emitDLLProgress("Saving database")
	if err := a.db.Save(); err != nil {
		return fmt.Errorf("save game database after update: %w", err)
	}
	return nil
}

func (a *App) RestoreDLLs(appID uint64) error {
	defer a.emitDLLProgress("")

	if a.db == nil {
		return ErrDatabaseNotLoaded
	}

	g := a.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	a.emitDLLProgress("Restoring backup")
	if err := dll.RestoreBackup(appID); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	a.emitDLLProgress("Scanning install directory")
	detected, err := dll.ScanDirectory(g.InstallDir)
	if err != nil {
		slog.Warn("scan after restore failed", "appID", appID, "error", err)
	} else {
		g.DLLs = detected
		g.ScannedAt = time.Now()
	}
	a.emitDLLProgress("Saving database")
	if err := a.db.Save(); err != nil {
		return fmt.Errorf("save game database after restore: %w", err)
	}
	return nil
}

func (a *App) HasDLLBackup(appID uint64) bool {
	return dll.BackupExists(appID)
}

func (a *App) LaunchGame(appID uint64) error {
	if a.db == nil {
		return ErrDatabaseNotLoaded
	}

	g := a.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	p, _ := profile.LoadEffective(appID)

	l := launcher.New(g)
	l.Profile = p
	l.Prepare()

	steamURL := fmt.Sprintf("steam://rungameid/%d", appID)
	if err := l.Launch([]string{"steam", steamURL}); err != nil {
		return fmt.Errorf("failed to launch game: %w", err)
	}

	return nil
}
