package gui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/dll"
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
		})
	}

	return info
}

type ProfileInfo struct {
	Preset           string `json:"preset"`
	SRMode           string `json:"srMode"`
	SROverride       bool   `json:"srOverride"`
	FGEnabled        bool   `json:"fgEnabled"`
	EnableHDR        bool   `json:"enableHdr"`
	EnableWayland    bool   `json:"enableWayland"`
	EnableNGXUpdater bool   `json:"enableNgxUpdater"`
}

func (a *App) GetProfile(appID uint64) *ProfileInfo {
	p, err := profile.Load(appID)
	if err != nil || p == nil {
		return nil
	}

	return &ProfileInfo{
		Preset:           string(p.Preset),
		SRMode:           string(p.DLSS.SRMode),
		SROverride:       p.DLSS.SROverride,
		FGEnabled:        p.DLSS.FGEnabled,
		EnableHDR:        p.Proton.EnableHDR,
		EnableWayland:    p.Proton.EnableWayland,
		EnableNGXUpdater: p.Proton.EnableNGXUpdater,
	}
}

func (a *App) SaveProfile(appID uint64, info ProfileInfo) error {
	p := &profile.Profile{
		Name:   "",
		Preset: profile.Preset(info.Preset),
		DLSS: profile.DLSSSettings{
			SRMode:     profile.DLSSMode(info.SRMode),
			SROverride: info.SROverride,
			FGEnabled:  info.FGEnabled,
			FGOverride: true,
		},
		Proton: profile.ProtonSettings{
			EnableHDR:        info.EnableHDR,
			EnableWayland:    info.EnableWayland,
			EnableNGXUpdater: info.EnableNGXUpdater,
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
	Model            string `json:"model"`
	Cores            int    `json:"cores"`
	AverageFrequency int    `json:"averageFrequency"`
	Governor         string `json:"governor"`
	SMTEnabled       bool   `json:"smtEnabled"`
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
