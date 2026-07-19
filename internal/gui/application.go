//go:build dev || production || bindings

package gui

import (
	"fmt"
	"strings"

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

func (b guiApplicationBoundary) getProfile(appID uint64) map[string]any {
	perGameProfile, err := b.loadProfile(appID)
	if err != nil {
		return nil
	}
	defaultProfile, err := b.loadDefaultProfile()
	if err != nil {
		return nil
	}
	if perGameProfile != nil {
		explanations, err := perGameProfile.Explain(defaultProfile)
		if err != nil {
			return nil
		}
		return profileView(perGameProfile.ResolveForApply(defaultProfile), explanations, false)
	}

	if defaultProfile == nil {
		return nil
	}
	explanations, err := (*profile.Profile)(nil).Explain(defaultProfile)
	if err != nil {
		return nil
	}
	return profileView(defaultProfile, explanations, true)
}

func (b guiApplicationBoundary) getDefaultProfile() map[string]any {
	defaultProfile, err := b.loadDefaultProfile()
	if err != nil {
		return nil
	}
	explanations, err := (*profile.Profile)(nil).Explain(defaultProfile)
	if err != nil {
		return nil
	}
	return profileView(defaultProfile, explanations, true)
}

func (b guiApplicationBoundary) patchGameProfile(appID uint64, patches []ProfilePatch) error {
	current, err := b.loadProfile(appID)
	if err != nil {
		return err
	}
	current = current.Clone()
	defaults, err := b.loadDefaultProfile()
	if err != nil {
		return fmt.Errorf("load default profile: %w", err)
	}
	for _, patch := range patches {
		if err := applyProfilePatch(current, defaults, patch, false); err != nil {
			return err
		}
	}
	return b.saveProfile(appID, current)
}

func (b guiApplicationBoundary) patchDefaultProfile(patches []ProfilePatch) error {
	current, err := b.loadDefaultProfile()
	if err != nil {
		return err
	}
	current = current.Clone()
	for _, patch := range patches {
		if err := applyProfilePatch(current, nil, patch, true); err != nil {
			return err
		}
	}
	return b.saveDefaultProfile(current)
}

func applyProfilePatch(current, defaults *profile.Profile, patch ProfilePatch, root bool) error {
	if _, ok := profile.Field(patch.Field); !ok {
		return fmt.Errorf("unknown profile field: %q", patch.Field)
	}
	switch patch.Operation {
	case "set":
		return current.Set(patch.Field, patch.Value)
	case "pin":
		if root {
			return fmt.Errorf("cannot pin a default profile field")
		}
		return current.PinField(patch.Field, defaults)
	case "reset":
		return current.Reset(patch.Field)
	default:
		return fmt.Errorf("unknown profile patch operation %q (valid: set, pin, reset)", patch.Operation)
	}
	return nil
}

func profileViewKey(descriptor profile.FieldDescriptor) string {
	if descriptor.Key == profile.FieldGPUPowerLimit || descriptor.Key == profile.FieldGPUFanSpeed || descriptor.Key == profile.FieldCPUAffinity {
		return "" // These fields have no current GUI control.
	}
	leaf := strings.TrimPrefix(descriptor.Key, descriptor.Subsystem+".")
	parts := strings.Split(leaf, "_")
	for index := 1; index < len(parts); index++ {
		parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
	}
	key := strings.Join(parts, "")
	if descriptor.Subsystem == "overlay" {
		key = "overlay" + strings.ToUpper(key[:1]) + key[1:]
	}
	return key
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

		latest := manifest.GetLatestDLL(string(d.Type))
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

func (b guiApplicationBoundary) installDLLVersion(appID uint64, dllType, version string) (dll.Result, error) {
	defer b.emitDLLProgress("")
	return dll.Install(appID, dllType, version, b.dllProgress)
}

func (b guiApplicationBoundary) updateDLLs(appID uint64) (dll.BatchResult, error) {
	defer b.emitDLLProgress("")
	return dll.UpdateGame(appID, "", b.dllProgress), nil
}

func (b guiApplicationBoundary) restoreDLLs(appID uint64) (dll.Result, error) {
	defer b.emitDLLProgress("")
	return dll.Restore(appID, b.dllProgress)
}

func (b guiApplicationBoundary) dllProgress(event dll.ProgressEvent) {
	switch event.Stage {
	case dll.StageResolving:
		b.emitDLLProgress("Resolving manifest")
	case dll.StageDownloading:
		b.emitDLLProgress(fmt.Sprintf("Downloading %s %s", event.DLLType, event.Version))
	case dll.StageInstalling:
		b.emitDLLProgress(fmt.Sprintf("Installing %s", event.Filename))
	case dll.StageUpdating:
		b.emitDLLProgress(fmt.Sprintf("Swapping %s", event.Filename))
	case dll.StageRestoring:
		b.emitDLLProgress("Restoring backup")
	case dll.StageScanning:
		b.emitDLLProgress("Scanning install directory")
	case dll.StageSaving:
		b.emitDLLProgress("Saving database")
	}
}
