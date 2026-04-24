package tui

import (
	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
	"github.com/jgabor/spela/internal/steam"
)

// Services provides injectable dependencies for TUI models.
// Production code uses DefaultServices(); tests substitute fakes.
type Services struct {
	LoadConfig         func() (*config.Config, error)
	LoadProfile        func(appID uint64) (*profile.Profile, error)
	LoadDefaultProfile func() (*profile.Profile, error)
	ProfileExists      func(appID uint64) bool
	BackupExists       func(appID uint64) bool
	KnownDLLTypes      func() []dll.KnownDLLTypeInfo
	ListCachedDLLs     func(manifestKey string) ([]string, error)
	UpdateCachedDLL    func(req DLLUpdateRequest) error
	// VKD3DNotice returns a human-readable descriptor_heap compatibility
	// notice for the given AppID, or "" when everything is compatible or
	// checks were skipped cleanly. Tests may override to inject a stub.
	VKD3DNotice func(appID uint64) string
}

// DefaultServices returns the production implementations.
func DefaultServices() *Services {
	return &Services{
		LoadConfig:         config.Load,
		LoadProfile:        profile.Load,
		LoadDefaultProfile: profile.LoadDefault,
		ProfileExists:      profile.Exists,
		BackupExists:       dll.BackupExists,
		KnownDLLTypes:      dll.KnownDLLTypes,
		ListCachedDLLs:     dll.ListCachedVersions,
		UpdateCachedDLL:    defaultUpdateCachedDLL,
		VKD3DNotice:        defaultVKD3DNotice,
	}
}

// DLLUpdateRequest describes a single stale deployment cell update.
type DLLUpdateRequest struct {
	Game          *game.Game
	TypeInfo      dll.KnownDLLTypeInfo
	LatestVersion string
	InstalledName string
}

func defaultUpdateCachedDLL(req DLLUpdateRequest) error {
	cachePath := dll.GetDLLCachePath(req.TypeInfo.ManifestKey, req.LatestVersion)
	gameDLLs := dll.GameDLLsFromDetected(req.Game.DLLs)
	if err := dll.SwapDLL(req.Game.AppID, req.Game.Name, gameDLLs, req.InstalledName, cachePath); err != nil {
		return err
	}
	for i := range req.Game.DLLs {
		if req.Game.DLLs[i].Type == req.TypeInfo.Type {
			req.Game.DLLs[i].Version = req.LatestVersion
		}
	}
	return nil
}

// defaultVKD3DNotice wires the production resolver + NVML driver probe
// into the proton.CompatibilityNotice helper. Consulted lazily so the
// Steam path and NVML state are re-evaluated each time the widget
// renders — the user may install a new Proton build between frames.
func defaultVKD3DNotice(appID uint64) string {
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
