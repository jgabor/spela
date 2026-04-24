//go:build dev || production || bindings

package gui

import (
	"fmt"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
	"github.com/jgabor/spela/internal/steam"
)

type guiApplicationBoundary struct {
	db                  *game.Database
	loadConfig          func() (*config.Config, error)
	loadProfile         func(uint64) (*profile.Profile, error)
	loadDefaultProfile  func() (*profile.Profile, error)
	saveProfile         func(uint64, *profile.Profile) error
	saveDefaultProfile  func(*profile.Profile) error
	findSteamPath       func() string
	compatibilityNotice func(uint64, proton.NoticeDeps) string
	resolveProton       func(string, uint64) (proton.Build, error)
	supportsVKD3DHeap   func(proton.Build) (bool, error)
	driverVersion       func() (string, error)
}

func defaultGUIApplicationBoundary(db *game.Database) guiApplicationBoundary {
	return guiApplicationBoundary{
		db:                  db,
		loadConfig:          config.Load,
		loadProfile:         profile.Load,
		loadDefaultProfile:  profile.LoadDefault,
		saveProfile:         profile.Save,
		saveDefaultProfile:  profile.SaveDefault,
		findSteamPath:       steam.FindSteamPath,
		compatibilityNotice: proton.CompatibilityNotice,
		resolveProton:       proton.ResolveForAppID,
		supportsVKD3DHeap:   proton.SupportsVKD3DHeap,
		driverVersion:       gpu.DriverVersionString,
	}
}

func (b guiApplicationBoundary) getProfile(appID uint64) *ProfileInfo {
	perGameProfile, err := b.loadProfile(appID)
	if err != nil {
		return nil
	}
	if perGameProfile != nil {
		return profileInfoFromProfile(perGameProfile, false)
	}

	defaultProfile, err := b.loadDefaultProfile()
	if err != nil || defaultProfile == nil {
		return nil
	}
	return profileInfoFromProfile(defaultProfile, true)
}

func (b guiApplicationBoundary) getDefaultProfile() *ProfileInfo {
	defaultProfile, err := b.loadDefaultProfile()
	if err != nil {
		return nil
	}
	return profileInfoFromProfile(defaultProfile, false)
}

func (b guiApplicationBoundary) saveGameProfile(appID uint64, info ProfileInfo) error {
	return b.saveProfile(appID, profileFromInfo(info))
}

func (b guiApplicationBoundary) saveDefault(info ProfileInfo) error {
	return b.saveDefaultProfile(profileFromInfo(info))
}

func (b guiApplicationBoundary) vkd3dHeapCompatibilityNotice(appID uint64) string {
	cfg, _ := b.loadConfig()
	steamRoot := ""
	if cfg != nil {
		steamRoot = cfg.SteamPath
	}
	if steamRoot == "" {
		steamRoot = b.findSteamPath()
	}
	return b.compatibilityNotice(appID, proton.NoticeDeps{
		SteamRoot:         steamRoot,
		ResolveForAppID:   b.resolveProton,
		SupportsVKD3DHeap: b.supportsVKD3DHeap,
		DriverVersion:     b.driverVersion,
	})
}

func (b guiApplicationBoundary) rejectDirectLaunch(appID uint64) error {
	if b.db == nil {
		return ErrDatabaseNotLoaded
	}

	g := b.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	return fmt.Errorf("direct Steam URI launch cannot track the game lifetime; set %s's Steam launch options to `spela %%command%%` instead", g.Name)
}
