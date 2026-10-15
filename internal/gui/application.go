//go:build dev || production || bindings

package gui

import (
	"fmt"
	"slices"
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
	mutateProfile       func(uint64, func(*profile.Profile, *profile.Profile) error) error
	mutateDefault       func(func(*profile.Profile) error) error
	followAllGames      func([]profile.OwnValueEntry, []string) profile.FollowOutcome
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
		mutateProfile:       profile.Mutate,
		mutateDefault:       profile.MutateDefault,
		followAllGames:      profile.FollowAllGamesIn,
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

func (b guiApplicationBoundary) profileView(appID *uint64) map[string]any {
	defaultProfile, err := b.loadDefaultProfile()
	if err != nil {
		return nil
	}
	var raw *profile.Profile
	if appID != nil {
		raw, err = b.loadProfile(*appID)
		if err != nil {
			return nil
		}
	}
	resolved := defaultProfile
	if raw != nil {
		resolved = raw.ResolveForApply(defaultProfile)
	}
	if resolved == nil {
		return nil
	}
	var explanations []profile.FieldExplanation
	if appID == nil {
		explanations, err = defaultProfile.ExplainAllGames()
	} else {
		explanations, err = raw.Explain(defaultProfile)
	}
	if err != nil {
		return nil
	}
	view := profileView(resolved, explanations, raw == nil, appID == nil)
	if appID == nil && view != nil {
		b.annotateOwnValueGames(view)
	}
	return view
}

// ownValueEntries reads every game profile once. The count on All games and
// the action that applies All games to those games both start from this list,
// so the number on screen and the games that change are the same.
func (b guiApplicationBoundary) ownValueEntries() []profile.OwnValueEntry {
	if b.db == nil {
		return nil
	}
	var entries []profile.OwnValueEntry
	for _, entry := range b.db.List() {
		if entry == nil {
			continue
		}
		if stored, err := b.loadProfile(entry.AppID); err == nil {
			entries = append(entries, profile.OwnValueEntry{AppID: entry.AppID, Name: entry.Name, Profile: stored})
		}
	}
	return entries
}

// annotateOwnValueGames tells each All games row how many games set their own
// value for it, so a change there is not mistaken for a change everywhere.
func (b guiApplicationBoundary) annotateOwnValueGames(view map[string]any) {
	items, _ := view["semantics"].([]ProfileFieldSemantics)
	if len(items) == 0 {
		return
	}
	games := profile.CollectOwnValueGames(b.ownValueEntries())
	for index := range items {
		count := len(games[items[index].Field])
		items[index].OwnValueSuffix = profile.OwnValueSuffix(count)
		items[index].OwnValueCount = count
		if count > 0 {
			items[index].OwnValueAction = profile.ActionApplyToGamesWithOwnValue
		}
	}
}

// ownValueGamesFor names the games that set their own value for any of the
// fields, sorted and without repeats.
func ownValueGamesFor(entries []profile.OwnValueEntry, fields []string) []string {
	games := profile.CollectOwnValueGames(entries)
	var names []string
	for _, field := range fields {
		names = append(names, games[field]...)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// previewApplyToOwnValues words the question asked before All games is applied
// to the games that set their own value for the fields.
func (b guiApplicationBoundary) previewApplyToOwnValues(fields []string) (map[string]any, error) {
	label, valueText, err := b.ownValueLabel(fields)
	if err != nil {
		return nil, err
	}
	names := ownValueGamesFor(b.ownValueEntries(), fields)
	// appIds are the games the question names; the apply changes only those.
	var appIDs []uint64
	for _, entry := range b.ownValueEntries() {
		if slices.ContainsFunc(fields, entry.Profile.IsOverridden) {
			appIDs = append(appIDs, entry.AppID)
		}
	}
	preview := map[string]any{"action": profile.ActionApplyToGamesWithOwnValue, "count": len(names), "body": "", "appIds": appIDs}
	if len(names) > 0 {
		preview["body"] = profile.ApplyToOwnValueBody(label, valueText, names)
	}
	return preview, nil
}

// applyToOwnValues makes the games the question named follow All games for the
// fields, and returns what happened with the refreshed All games view. A game
// that gained its own value after the question was asked is not touched: the
// games that change are the games that were named.
func (b guiApplicationBoundary) applyToOwnValues(fields []string, appIDs []uint64) (map[string]any, error) {
	label, _, err := b.ownValueLabel(fields)
	if err != nil {
		return nil, err
	}
	var named []profile.OwnValueEntry
	for _, entry := range b.ownValueEntries() {
		if slices.Contains(appIDs, entry.AppID) {
			named = append(named, entry)
		}
	}
	outcome := b.followAllGames(named, fields)
	return map[string]any{
		"message": profile.FollowOutcomeText(label, outcome),
		"failure": profile.FollowFailureText(outcome),
		"failed":  outcome.Failed,
		"profile": b.profileView(nil),
	}, nil
}

// ownValueLabel is the label of the first field, and the All games value in
// words when one field is asked about (empty for a control that covers two).
func (b guiApplicationBoundary) ownValueLabel(fields []string) (label, valueText string, err error) {
	if len(fields) == 0 {
		return "", "", fmt.Errorf("no profile field given")
	}
	for _, field := range fields {
		if !profile.IsValidField(field) {
			return "", "", fmt.Errorf("unknown profile field: %q", field)
		}
	}
	descriptor, _ := profile.Field(fields[0])
	if len(fields) > 1 {
		return descriptor.Label, "", nil
	}
	defaults, err := b.loadDefaultProfile()
	if err != nil {
		return "", "", err
	}
	if defaults == nil {
		defaults = &profile.Profile{}
	}
	explanation, err := defaults.ExplainAllGamesField(fields[0])
	if err != nil {
		return "", "", err
	}
	return descriptor.Label, profile.ValueText(descriptor, explanation), nil
}

func (b guiApplicationBoundary) patchGameProfile(appID uint64, patches []ProfilePatch) error {
	return b.mutateProfile(appID, func(current, defaults *profile.Profile) error {
		for _, patch := range patches {
			if err := applyProfilePatch(current, defaults, patch, false); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b guiApplicationBoundary) patchDefaultProfile(patches []ProfilePatch) error {
	return b.mutateDefault(func(current *profile.Profile) error {
		for _, patch := range patches {
			if err := applyProfilePatch(current, nil, patch, true); err != nil {
				return err
			}
		}
		return nil
	})
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
