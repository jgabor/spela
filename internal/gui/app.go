package gui

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/profile"
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
	case "default", "dark":
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

		games = append(games, info)
	}

	return games
}

func (a *App) GetGame(appID uint64) *GameInfo {
	if a.db == nil {
		return nil
	}

	g, ok := a.db.Games[appID]
	if !ok || g == nil {
		return nil
	}

	info := &GameInfo{
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
	SROverride           bool   `json:"srOverride"`
	FGEnabled            bool   `json:"fgEnabled"`
	MultiFrame           int    `json:"multiFrame"`
	Indicator            bool   `json:"indicator"`
	ShaderCache          bool   `json:"shaderCache"`
	ThreadedOptimization bool   `json:"threadedOptimization"`
	PowerMizer           string `json:"powerMizer"`
	EnableHDR            bool   `json:"enableHdr"`
	EnableWayland        bool   `json:"enableWayland"`
	EnableNGXUpdater     bool   `json:"enableNgxUpdater"`
	BackupOnLaunch       bool   `json:"backupOnLaunch"`
}

func (a *App) GetProfile(appID uint64) *ProfileInfo {
	p, err := profile.Load(appID)
	if err != nil || p == nil {
		return nil
	}

	return &ProfileInfo{
		SRMode:               string(p.DLSS.SRMode),
		SRPreset:             string(p.DLSS.SRPreset),
		SROverride:           p.DLSS.SROverride,
		FGEnabled:            p.DLSS.FGEnabled,
		MultiFrame:           p.DLSS.MultiFrame,
		Indicator:            p.DLSS.Indicator,
		ShaderCache:          p.GPU.ShaderCache,
		ThreadedOptimization: p.GPU.ThreadedOptimization,
		PowerMizer:           p.GPU.PowerMizer,
		EnableHDR:            p.Proton.EnableHDR,
		EnableWayland:        p.Proton.EnableWayland,
		EnableNGXUpdater:     p.Proton.EnableNGXUpdater,
		BackupOnLaunch:       p.Ludusavi.BackupOnLaunch,
	}
}

func (a *App) SaveProfile(appID uint64, info ProfileInfo) error {
	p := &profile.Profile{
		DLSS: profile.DLSSSettings{
			SRMode:     profile.DLSSMode(info.SRMode),
			SRPreset:   profile.DLSSPreset(info.SRPreset),
			SROverride: info.SROverride,
			FGEnabled:  info.FGEnabled,
			FGOverride: true,
			MultiFrame: info.MultiFrame,
			Indicator:  info.Indicator,
		},
		GPU: profile.GPUSettings{
			ShaderCache:          info.ShaderCache,
			ThreadedOptimization: info.ThreadedOptimization,
			PowerMizer:           info.PowerMizer,
		},
		Proton: profile.ProtonSettings{
			EnableHDR:        info.EnableHDR,
			EnableWayland:    info.EnableWayland,
			EnableNGXUpdater: info.EnableNGXUpdater,
		},
		Ludusavi: profile.LudusaviSettings{
			BackupOnLaunch: info.BackupOnLaunch,
		},
	}

	return profile.Save(appID, p)
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

	g, ok := a.db.Games[appID]
	if !ok || g == nil {
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

func (a *App) UpdateDLLs(appID uint64) error {
	if a.db == nil {
		return ErrDatabaseNotLoaded
	}

	g, ok := a.db.Games[appID]
	if !ok || g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return err
	}

	var gameDLLs []dll.GameDLL
	for _, d := range g.DLLs {
		gameDLLs = append(gameDLLs, dll.GameDLL{
			Name:    d.Name,
			Path:    d.Path,
			Version: d.Version,
		})
	}

	for _, d := range g.DLLs {
		latest := manifest.GetLatestDLL(d.Name)
		if latest == nil || latest.Version == d.Version {
			continue
		}

		cachePath, err := dll.GetOrDownloadDLL(manifest, d.Name, "latest")
		if err != nil {
			return err
		}

		if err := dll.SwapDLL(appID, g.Name, gameDLLs, d.Name, cachePath); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) RestoreDLLs(appID uint64) error {
	return dll.RestoreBackup(appID)
}

func (a *App) HasDLLBackup(appID uint64) bool {
	return dll.BackupExists(appID)
}

func (a *App) LaunchGame(appID uint64) error {
	if a.db == nil {
		return ErrDatabaseNotLoaded
	}

	g, ok := a.db.Games[appID]
	if !ok || g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	p, _ := profile.Load(appID)

	e := env.New()
	if p != nil {
		p.Apply(e)
	}

	cmd := exec.Command("steam", fmt.Sprintf("steam://rungameid/%d", appID))
	e.ApplyToCmd(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch game: %w", err)
	}

	return nil
}
