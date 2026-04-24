//go:build dev || production || bindings

package gui

import (
	"fmt"
	"os"
	"time"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/logging"
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
	getManifest         func(bool, string) (*dll.Manifest, error)
	ensureDLLCached     func(*dll.DLL, string) (string, error)
	installDLL          func(uint64, string, string, []dll.GameDLL, string, string) error
	swapDLL             func(uint64, string, []dll.GameDLL, string, string) error
	restoreBackup       func(uint64) error
	scanDLLDirectory    func(string) ([]game.DetectedDLL, error)
	backupExists        func(uint64) bool
	saveDatabase        func(*game.Database) error
	emitDLLProgress     func(string)
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
		getManifest:         dll.GetManifest,
		ensureDLLCached:     ensureDLLCached,
		installDLL:          dll.InstallDLL,
		swapDLL:             dll.SwapDLL,
		restoreBackup:       dll.RestoreBackup,
		scanDLLDirectory:    dll.ScanDirectory,
		backupExists:        dll.BackupExists,
		saveDatabase:        func(db *game.Database) error { return db.Save() },
		emitDLLProgress:     func(string) {},
	}
}

func newGUIApplicationBoundary(db *game.Database, emitDLLProgress func(string)) guiApplicationBoundary {
	boundary := defaultGUIApplicationBoundary(db)
	if emitDLLProgress != nil {
		boundary.emitDLLProgress = emitDLLProgress
	}
	return boundary
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

func (b guiApplicationBoundary) checkDLLUpdates(appID uint64) []DLLUpdateInfo {
	if b.db == nil {
		return []DLLUpdateInfo{}
	}

	g := b.db.GetGame(appID)
	if g == nil {
		return []DLLUpdateInfo{}
	}

	manifest, err := b.getManifest(false, "")
	if err != nil {
		logging.Debug("failed to get DLL manifest", "error", err)
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

func (b guiApplicationBoundary) listDLLInstallTypes(appID uint64) ([]string, error) {
	if b.db == nil {
		return nil, ErrDatabaseNotLoaded
	}

	g := b.db.GetGame(appID)
	if g == nil {
		return nil, fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	manifest, err := b.getManifest(false, "")
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

func (b guiApplicationBoundary) listDLLVersions(dllType string) ([]string, error) {
	if dllType == "" {
		return nil, fmt.Errorf("dll type is required")
	}

	manifest, err := b.getManifest(false, "")
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

func ensureDLLCached(target *dll.DLL, dllName string) (string, error) {
	cachePath := dll.GetDLLCachePath(dllName, target.Version)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}
	return dll.DownloadDLLWithProgress(target, dllName, nil)
}

func (b guiApplicationBoundary) installDLLVersion(appID uint64, dllType, version string) error {
	defer b.emitDLLProgress("")

	if b.db == nil {
		return ErrDatabaseNotLoaded
	}
	if dllType == "" {
		return fmt.Errorf("dll type is required")
	}

	g := b.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	b.emitDLLProgress("Resolving manifest")
	manifest, err := b.getManifest(false, "")
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

	b.emitDLLProgress(fmt.Sprintf("Downloading %s %s", dllType, targetVersion))
	cachePath, err := b.ensureDLLCached(targetDLL, dllType)
	if err != nil {
		return fmt.Errorf("download %s: %w", targetVersion, err)
	}

	gameDLLs := dll.GameDLLsFromDetected(g.DLLs)

	b.emitDLLProgress(fmt.Sprintf("Installing %s", targetDLL.Filename))
	if err := b.installDLL(g.AppID, g.Name, g.InstallDir, gameDLLs, targetDLL.Filename, cachePath); err != nil {
		return fmt.Errorf("install DLL: %w", err)
	}

	return b.scanAndSaveDLLs(g, "install")
}

func (b guiApplicationBoundary) updateDLLs(appID uint64) error {
	defer b.emitDLLProgress("")

	if b.db == nil {
		return ErrDatabaseNotLoaded
	}

	g := b.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	b.emitDLLProgress("Resolving manifest")
	manifest, err := b.getManifest(false, "")
	if err != nil {
		return fmt.Errorf("load DLL manifest: %w", err)
	}

	gameDLLs := dll.GameDLLsFromDetected(g.DLLs)

	for _, d := range g.DLLs {
		latest := manifest.GetLatestDLL(d.Name)
		if latest == nil || latest.Version == d.Version {
			continue
		}

		b.emitDLLProgress(fmt.Sprintf("Downloading %s %s", d.Name, latest.Version))
		cachePath, err := b.ensureDLLCached(latest, d.Name)
		if err != nil {
			return fmt.Errorf("download %s: %w", d.Name, err)
		}

		b.emitDLLProgress(fmt.Sprintf("Swapping %s", d.Name))
		if err := b.swapDLL(appID, g.Name, gameDLLs, d.Name, cachePath); err != nil {
			return fmt.Errorf("swap %s: %w", d.Name, err)
		}
	}

	b.emitDLLProgress("Scanning install directory")
	detected, err := b.scanDLLDirectory(g.InstallDir)
	if err != nil {
		logging.Warn("scan after update failed", "appID", appID, "error", err)
	} else {
		g.DLLs = detected
		g.ScannedAt = time.Now()
	}
	b.emitDLLProgress("Saving database")
	if err := b.saveDatabase(b.db); err != nil {
		return fmt.Errorf("save game database after update: %w", err)
	}
	return nil
}

func (b guiApplicationBoundary) restoreDLLs(appID uint64) error {
	defer b.emitDLLProgress("")

	if b.db == nil {
		return ErrDatabaseNotLoaded
	}

	g := b.db.GetGame(appID)
	if g == nil {
		return fmt.Errorf("%w: %d", ErrGameNotFound, appID)
	}

	b.emitDLLProgress("Restoring backup")
	if err := b.restoreBackup(appID); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	b.emitDLLProgress("Scanning install directory")
	detected, err := b.scanDLLDirectory(g.InstallDir)
	if err != nil {
		logging.Warn("scan after restore failed", "appID", appID, "error", err)
	} else {
		g.DLLs = detected
		g.ScannedAt = time.Now()
	}
	b.emitDLLProgress("Saving database")
	if err := b.saveDatabase(b.db); err != nil {
		return fmt.Errorf("save game database after restore: %w", err)
	}
	return nil
}

func (b guiApplicationBoundary) scanAndSaveDLLs(g *game.Game, operation string) error {
	b.emitDLLProgress("Scanning install directory")
	detected, err := b.scanDLLDirectory(g.InstallDir)
	if err != nil {
		return fmt.Errorf("scan install directory: %w", err)
	}

	g.DLLs = detected
	g.ScannedAt = time.Now()
	b.emitDLLProgress("Saving database")
	if err := b.saveDatabase(b.db); err != nil {
		return fmt.Errorf("save game database after %s: %w", operation, err)
	}
	return nil
}

func (b guiApplicationBoundary) hasDLLBackup(appID uint64) bool {
	return b.backupExists(appID)
}
