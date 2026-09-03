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
	SaveConfig         func(*config.Config) error
	ScanGames          func(cfg *config.Config) (*game.Database, error)
	LoadProfile        func(appID uint64) (*profile.Profile, error)
	LoadDefaultProfile func() (*profile.Profile, error)
	ProfileExists      func(appID uint64) bool
	BackupExists       func(appID uint64) bool
	KnownDLLTypes      func() []dll.KnownDLLTypeInfo
	ListCachedDLLs     func(manifestKey string) ([]string, error)
	BatchUpdateDLLs    func([]dll.UpdateRequest) dll.BatchResult
	InstallDLL         func(uint64, string, string) (dll.Result, error)
	UpdateGameDLLs     func(uint64) dll.BatchResult
	UpdateGamesDLLs    func([]uint64) dll.BatchResult
	RestoreDLLs        func(uint64) (dll.Result, error)
	// VKD3DNotice returns a human-readable descriptor_heap compatibility
	// notice for the given AppID, or "" when everything is compatible or
	// checks were skipped cleanly. Tests may override to inject a stub.
	VKD3DNotice func(appID uint64) string
}

// DefaultServices returns the production implementations.
func DefaultServices() *Services {
	return &Services{
		LoadConfig:         config.Load,
		SaveConfig:         func(configuration *config.Config) error { return configuration.Save() },
		ScanGames:          steam.Rescan,
		LoadProfile:        profile.Load,
		LoadDefaultProfile: profile.LoadDefault,
		ProfileExists:      profile.Exists,
		BackupExists:       dll.BackupExists,
		KnownDLLTypes:      dll.KnownDLLTypes,
		ListCachedDLLs:     dll.ListCachedVersions,
		BatchUpdateDLLs:    func(requests []dll.UpdateRequest) dll.BatchResult { return dll.BatchUpdate(requests, nil) },
		InstallDLL: func(appID uint64, dllType, version string) (dll.Result, error) {
			return dll.Install(appID, dllType, version, nil)
		},
		UpdateGameDLLs:  func(appID uint64) dll.BatchResult { return dll.UpdateGame(appID, "", nil) },
		UpdateGamesDLLs: func(appIDs []uint64) dll.BatchResult { return dll.UpdateGames(appIDs, "", nil) },
		RestoreDLLs:     func(appID uint64) (dll.Result, error) { return dll.Restore(appID, nil) },
		VKD3DNotice:     defaultVKD3DNotice,
	}
}

func (services *Services) installDLL(appID uint64, dllType, version string) (dll.Result, error) {
	if services != nil && services.InstallDLL != nil {
		return services.InstallDLL(appID, dllType, version)
	}
	return dll.Install(appID, dllType, version, nil)
}

func (services *Services) updateGameDLLs(appID uint64) dll.BatchResult {
	if services != nil && services.UpdateGameDLLs != nil {
		return services.UpdateGameDLLs(appID)
	}
	return dll.UpdateGame(appID, "", nil)
}

func (services *Services) updateGamesDLLs(appIDs []uint64) dll.BatchResult {
	if services != nil && services.UpdateGamesDLLs != nil {
		return services.UpdateGamesDLLs(appIDs)
	}
	return dll.UpdateGames(appIDs, "", nil)
}

func (services *Services) restoreDLLs(appID uint64) (dll.Result, error) {
	if services != nil && services.RestoreDLLs != nil {
		return services.RestoreDLLs(appID)
	}
	return dll.Restore(appID, nil)
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
