package dll

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jgabor/spela/internal/game"
)

type Outcome string

const (
	OutcomeChanged Outcome = "changed"
	OutcomeNoOp    Outcome = "no-op"
)

type Stage string

const (
	StageResolving   Stage = "resolving"
	StageDownloading Stage = "downloading"
	StageInstalling  Stage = "installing"
	StageUpdating    Stage = "updating"
	StageRestoring   Stage = "restoring"
	StageScanning    Stage = "scanning"
	StageSaving      Stage = "saving"
)

type ProgressEvent struct {
	Stage      Stage
	DLLType    string
	Version    string
	Filename   string
	Downloaded int64
	Total      int64
}

type Progress func(ProgressEvent)

type Result struct {
	Outcome           Outcome
	FilesChanged      bool
	MetadataPersisted bool
	Game              *game.Game
}

type PartialFailure struct {
	Result Result
	Stage  Stage
	Err    error
}

func (e *PartialFailure) Error() string {
	return fmt.Sprintf("DLL files changed but metadata did not fully persist during %s: %v", e.Stage, e.Err)
}

func (e *PartialFailure) Unwrap() error { return e.Err }

type UpdateRequest struct {
	AppID         uint64
	DLLType       string
	Version       string
	InstalledPath string
	CachedOnly    bool
}

type BatchItem struct {
	Path   string
	Result Result
	Err    error
}

type BatchResult struct {
	Items     []BatchItem
	Updated   int
	Unchanged int
	Failed    int
}

func Install(appID uint64, dllType, version string, progress Progress) (Result, error) {
	return mutate(progress, func(database *game.Database) (Result, error) {
		return install(database, appID, dllType, version, progress)
	})
}

func Update(request UpdateRequest, progress Progress) (Result, error) {
	return mutate(progress, func(database *game.Database) (Result, error) { return update(database, request, progress) })
}

func BatchUpdate(requests []UpdateRequest, progress Progress) BatchResult {
	return batchUpdate(requests, nil, progress)
}

func UpdateGame(appID uint64, dllType string, progress Progress) BatchResult {
	return UpdateGames([]uint64{appID}, dllType, progress)
}

func UpdateGames(appIDs []uint64, dllType string, progress Progress) BatchResult {
	fallback := make([]UpdateRequest, len(appIDs))
	for index, appID := range appIDs {
		fallback[index] = UpdateRequest{AppID: appID, DLLType: dllType}
	}
	return batchUpdate(fallback, func(database *game.Database) []UpdateRequest {
		var requests []UpdateRequest
		for _, appID := range appIDs {
			entry := database.GetGame(appID)
			if entry == nil {
				requests = append(requests, UpdateRequest{AppID: appID, DLLType: dllType})
				continue
			}
			for _, detected := range entry.DLLs {
				if dllType == "" || strings.EqualFold(string(detected.Type), dllType) {
					requests = append(requests, UpdateRequest{AppID: appID, DLLType: string(detected.Type), InstalledPath: detected.Path})
				}
			}
		}
		return requests
	}, progress)
}

func batchUpdate(requests []UpdateRequest, resolve func(*game.Database) []UpdateRequest, progress Progress) BatchResult {
	batch := BatchResult{Items: make([]BatchItem, 0, len(requests))}
	processed := 0
	_, transactionErr := game.Transaction(func(database *game.Database) (bool, error) {
		if resolve != nil {
			requests = resolve(database)
		}
		changed := false
		for _, request := range requests {
			result, itemErr := update(database, request, progress)
			batch.add(request, result, itemErr)
			processed++
			changed = changed || itemErr == nil && result.FilesChanged
		}
		if changed {
			emit(progress, ProgressEvent{Stage: StageSaving})
		}
		return changed, nil
	})
	if transactionErr != nil {
		for _, request := range requests[processed:] {
			batch.add(request, Result{}, transactionErr)
		}
	}
	for index := range batch.Items {
		item := &batch.Items[index]
		if item.Err == nil && item.Result.FilesChanged {
			item.Err = finalize(&item.Result, transactionErr)
		}
		if item.Err != nil {
			batch.Failed++
		} else if item.Result.Outcome == OutcomeNoOp {
			batch.Unchanged++
		} else {
			batch.Updated++
		}
	}
	return batch
}

func Restore(appID uint64, progress Progress) (Result, error) {
	return mutate(progress, func(database *game.Database) (Result, error) { return restore(database, appID, progress, copyFile) })
}

func mutate(progress Progress, operation func(*game.Database) (Result, error)) (Result, error) {
	var result Result
	var operationErr error
	_, transactionErr := game.Transaction(func(database *game.Database) (bool, error) {
		result, operationErr = operation(database)
		if operationErr != nil || !result.FilesChanged {
			return false, nil
		}
		emit(progress, ProgressEvent{Stage: StageSaving})
		return true, nil
	})
	if operationErr != nil {
		return result, operationErr
	}
	transactionErr = finalize(&result, transactionErr)
	return result, transactionErr
}

func finalize(result *Result, err error) error {
	if !result.FilesChanged {
		return err
	}
	if err != nil {
		return &PartialFailure{Result: *result, Stage: StageSaving, Err: fmt.Errorf("save game database: %w", err)}
	}
	result.MetadataPersisted = true
	return nil
}

func restore(database *game.Database, appID uint64, progress Progress, replace func(string, string) error) (Result, error) {
	result := Result{}
	entry, err := databaseGame(database, appID)
	if err != nil {
		return result, err
	}
	emit(progress, ProgressEvent{Stage: StageRestoring})
	changed, err := restoreBackup(appID, replace)
	if err != nil {
		if changed > 0 {
			result.Outcome, result.FilesChanged = OutcomeChanged, true
			return result, &PartialFailure{Result: result, Stage: StageRestoring, Err: fmt.Errorf("restore backup: %w", err)}
		}
		return result, fmt.Errorf("restore backup: %w", err)
	}
	return refresh(entry, result, progress)
}

func install(database *game.Database, appID uint64, dllType, version string, progress Progress) (Result, error) {
	result := Result{}
	entry, err := databaseGame(database, appID)
	if err != nil {
		return result, err
	}
	dllType = strings.ToLower(dllType)
	if dllType == "" {
		return result, errors.New("DLL type is required")
	}
	emit(progress, ProgressEvent{Stage: StageResolving, DLLType: dllType, Version: version})
	target, err := resolveManifestDLL(dllType, version)
	if err != nil {
		return result, err
	}
	if err := validateManifestEntry(target, dllType); err != nil {
		return result, err
	}
	targetPath, targetVersion, err := installTarget(entry, target.Filename)
	if err != nil {
		return result, err
	}
	if err := guardSwap(entry.AppID); err != nil {
		return result, err
	}
	cachePath, err := acquire(target, dllType, progress, true)
	if err != nil {
		return result, fmt.Errorf("acquire %s %s: %w", dllType, target.Version, err)
	}
	emit(progress, ProgressEvent{Stage: StageInstalling, DLLType: dllType, Version: target.Version, Filename: target.Filename})
	if err := installDLLAtPath(entry.AppID, entry.Name, entry.DLLs, targetPath, targetVersion, target.Filename, cachePath); err != nil {
		return result, fmt.Errorf("install DLL: %w", err)
	}
	return refresh(entry, result, progress)
}

func update(database *game.Database, request UpdateRequest, progress Progress) (Result, error) {
	result := Result{}
	entry, err := databaseGame(database, request.AppID)
	if err != nil {
		return result, err
	}
	dllType := strings.ToLower(request.DLLType)
	installed, err := detectedDLL(entry, dllType, request.InstalledPath)
	if err != nil {
		return result, err
	}
	if request.CachedOnly && request.Version == "" {
		return result, errors.New("cached DLL version is required")
	}
	emit(progress, ProgressEvent{Stage: StageResolving, DLLType: dllType, Version: request.Version})
	target, err := resolveManifestDLL(dllType, request.Version)
	if err != nil {
		return result, err
	}
	if err := validateManifestFilename(target.Filename); err != nil {
		return result, fmt.Errorf("invalid %s %s manifest filename: %w", dllType, target.Version, err)
	}
	if installed.Version != "" && !IsNewer(installed.Version, target.Version) {
		result.Outcome, result.MetadataPersisted, result.Game = OutcomeNoOp, true, cloneGame(entry)
		return result, nil
	}
	if err := validateManifestChecksum(target, dllType); err != nil {
		return result, err
	}
	if err := guardSwap(entry.AppID); err != nil {
		return result, err
	}
	cachePath, err := acquire(target, dllType, progress, !request.CachedOnly)
	if err != nil {
		return result, fmt.Errorf("acquire %s %s: %w", dllType, target.Version, err)
	}
	emit(progress, ProgressEvent{Stage: StageUpdating, DLLType: dllType, Version: target.Version, Filename: installed.Name})
	if err := swapDLLAtPath(entry.AppID, entry.Name, entry.DLLs, installed.Path, cachePath); err != nil {
		return result, fmt.Errorf("swap %s: %w", dllType, err)
	}
	return refresh(entry, result, progress)
}

func (batch *BatchResult) add(request UpdateRequest, result Result, err error) {
	batch.Items = append(batch.Items, BatchItem{Path: request.InstalledPath, Result: result, Err: err})
}

func databaseGame(database *game.Database, appID uint64) (*game.Game, error) {
	entry := database.GetGame(appID)
	if entry == nil {
		return nil, fmt.Errorf("game not found: %d", appID)
	}
	return entry, nil
}

func refresh(entry *game.Game, result Result, progress Progress) (Result, error) {
	result.Outcome, result.FilesChanged = OutcomeChanged, true
	emit(progress, ProgressEvent{Stage: StageScanning})
	detected, err := ScanDirectory(entry.InstallDir)
	if err != nil {
		return result, &PartialFailure{Result: result, Stage: StageScanning, Err: fmt.Errorf("scan install directory: %w", err)}
	}
	entry.DLLs, entry.ScannedAt = detected, time.Now()
	result.Game = cloneGame(entry)
	return result, nil
}

func resolveManifestDLL(dllType, version string) (*DLL, error) {
	manifest, err := GetManifest(false, "")
	if err != nil {
		return nil, fmt.Errorf("load DLL manifest: %w", err)
	}
	target := manifest.GetLatestDLL(dllType)
	if version != "" && version != "latest" {
		target = manifest.GetDLLVersion(dllType, version)
	}
	if target == nil {
		return nil, fmt.Errorf("no %s version %q available in manifest", dllType, version)
	}
	return target, nil
}

func detectedDLL(entry *game.Game, dllType, installedPath string) (*game.DetectedDLL, error) {
	for index := range entry.DLLs {
		candidate := &entry.DLLs[index]
		if strings.EqualFold(string(candidate.Type), dllType) && filepath.Clean(candidate.Path) == filepath.Clean(installedPath) {
			if err := pathWithin(entry.InstallDir, candidate.Path); err != nil {
				return nil, err
			}
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("detected %s DLL path not found: %s", dllType, installedPath)
}

func acquire(target *DLL, dllType string, progress Progress, allowDownload bool) (string, error) {
	emit(progress, ProgressEvent{Stage: StageDownloading, DLLType: dllType, Version: target.Version})
	return acquireDLL(target, dllType, allowDownload, func(downloaded, total int64) {
		emit(progress, ProgressEvent{Stage: StageDownloading, DLLType: dllType, Version: target.Version, Downloaded: downloaded, Total: total})
	})
}

func validateManifestEntry(entry *DLL, dllType string) error {
	if entry.Version == "" {
		return fmt.Errorf("invalid %s manifest entry: version is required", dllType)
	}
	if err := validateManifestFilename(entry.Filename); err != nil {
		return fmt.Errorf("invalid %s %s manifest filename: %w", dllType, entry.Version, err)
	}
	return validateManifestChecksum(entry, dllType)
}

func validateManifestChecksum(entry *DLL, dllType string) error {
	checksum, err := hex.DecodeString(entry.SHA256)
	if err != nil || len(checksum) != 32 {
		return fmt.Errorf("invalid %s %s manifest checksum", dllType, entry.Version)
	}
	return nil
}

func validateManifestFilename(filename string) error {
	if filename == "" || filename == "." || filename == ".." || filepath.IsAbs(filename) || filepath.Base(filename) != filename || strings.ContainsAny(filename, `/\\`) {
		return errors.New("must be a base name without path components")
	}
	return nil
}

func installTarget(entry *game.Game, filename string) (string, string, error) {
	var match *game.DetectedDLL
	for index := range entry.DLLs {
		if entry.DLLs[index].Name == filename {
			if match != nil {
				return "", "", fmt.Errorf("multiple DLLs named %s found; an exact path is required", filename)
			}
			match = &entry.DLLs[index]
		}
	}
	if match != nil {
		if err := pathWithin(entry.InstallDir, match.Path); err != nil {
			return "", "", err
		}
		return match.Path, match.Version, nil
	}
	target := filepath.Join(entry.InstallDir, filename)
	if err := pathWithin(entry.InstallDir, target); err != nil {
		return "", "", err
	}
	return target, "", nil
}

func pathWithin(root, target string) error {
	rootPath, err := filepath.Abs(root)
	if err != nil || root == "" {
		return errors.New("install directory is required")
	}
	targetPath, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(rootPath, targetPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("DLL target escapes install directory: %s", target)
	}
	return nil
}

func cloneGame(entry *game.Game) *game.Game {
	if entry == nil {
		return nil
	}
	clone := *entry
	clone.DLLs = append([]game.DetectedDLL(nil), entry.DLLs...)
	return &clone
}

func emit(progress Progress, event ProgressEvent) {
	if progress != nil {
		progress(event)
	}
}
