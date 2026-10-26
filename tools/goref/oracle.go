package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

// droppedFields are the profile fields decision 10 drops from the rewrite.
var droppedFields = map[string]bool{
	profile.FieldOverlayEnabled:       true,
	profile.FieldOverlayPosition:      true,
	profile.FieldOverlayShowFPS:       true,
	profile.FieldOverlayShowFrametime: true,
	profile.FieldOverlayShowCPU:       true,
	profile.FieldOverlayShowGPU:       true,
	profile.FieldOverlayShowVRAM:      true,
	profile.FieldOverlayToggleKey:     true,
	profile.FieldGPUPowerMizer:        true,
	profile.FieldCPUAffinity:          true,
}

// errorText returns nil for a nil error, so JSON shows null.
func errorText(err error) *string {
	if err == nil {
		return nil
	}
	text := err.Error()
	return &text
}

// writeJSON writes value as indented JSON with a trailing newline.
func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeBytes(path, append(data, '\n'))
}

type gameFieldOracle struct {
	Field         string `json:"field"`
	Kept          bool   `json:"kept"`
	Stored        any    `json:"stored"`
	Overridden    bool   `json:"overridden"`
	Effective     any    `json:"effective"`
	Source        string `json:"source"`
	Origin        string `json:"origin"`
	AllGamesSets  bool   `json:"all_games_sets"`
	AllGamesValue any    `json:"all_games_value"`
}

type gameProfileOracle struct {
	AppID     uint64            `json:"app_id"`
	File      string            `json:"file"`
	Exists    bool              `json:"exists"`
	LoadError *string           `json:"load_error"`
	Loaded    bool              `json:"loaded"`
	Name      string            `json:"name"`
	Overrides map[string]bool   `json:"overrides_after_load"`
	Fields    []gameFieldOracle `json:"fields,omitempty"`
}

type allGamesFieldOracle struct {
	Field        string `json:"field"`
	Kept         bool   `json:"kept"`
	Stored       any    `json:"stored"`
	Marked       bool   `json:"marked"`
	Origin       string `json:"origin"`
	AllGamesSets bool   `json:"all_games_sets"`
}

type allGamesOracle struct {
	File      string                `json:"file"`
	Exists    bool                  `json:"exists"`
	LoadError *string               `json:"load_error"`
	Name      string                `json:"name"`
	Overrides map[string]bool       `json:"overrides_after_load"`
	Fields    []allGamesFieldOracle `json:"fields,omitempty"`
}

type profileCaseOracle struct {
	Note      string              `json:"note"`
	AllGames  allGamesOracle      `json:"all_games"`
	Games     []gameProfileOracle `json:"games"`
	List      []uint64            `json:"list"`
	ListError *string             `json:"list_error"`
}

// profileOracle records what Go makes of the profiles in the current case:
// the loaded values after Go's in-memory migrations, the effective launch
// values from ResolveForApply, and the origin words from Explain.
func profileOracle(c *fixtureCase, note string, appIDs []uint64) (*profileCaseOracle, error) {
	result := &profileCaseOracle{Note: note}
	profilesDir := c.path("config", "spela", "profiles")

	defaultPath := filepath.Join(profilesDir, "default.yaml")
	result.AllGames.File = "config/spela/profiles/default.yaml"
	result.AllGames.Exists = fileExists(defaultPath)
	defaults, defaultsErr := profile.LoadDefault()
	result.AllGames.LoadError = errorText(defaultsErr)
	if defaultsErr == nil && defaults != nil {
		result.AllGames.Name = defaults.Name
		result.AllGames.Overrides = defaults.Overrides
		explanations, err := defaults.ExplainAllGames()
		if err != nil {
			return nil, err
		}
		for _, item := range explanations {
			stored, err := profile.ReadField(defaults, item.Field)
			if err != nil {
				return nil, err
			}
			result.AllGames.Fields = append(result.AllGames.Fields, allGamesFieldOracle{
				Field:        item.Field,
				Kept:         !droppedFields[item.Field],
				Stored:       stored,
				Marked:       defaults.IsOverridden(item.Field),
				Origin:       string(item.Origin),
				AllGamesSets: item.AllGamesSets,
			})
		}
	}

	for _, appID := range appIDs {
		entry := gameProfileOracle{AppID: appID, File: "config/spela/profiles/" + uintText(appID) + ".yaml"}
		entry.Exists = fileExists(filepath.Join(profilesDir, uintText(appID)+".yaml"))
		loaded, err := profile.Load(appID)
		entry.LoadError = errorText(err)
		if err == nil {
			entry.Loaded = loaded != nil
			if loaded != nil {
				entry.Name = loaded.Name
				entry.Overrides = loaded.Overrides
			}
			// Launch resolution as LoadLaunchProfiles does it: with no game
			// profile, the defaults apply directly.
			var effective *profile.Profile
			if defaultsErr == nil {
				if loaded != nil {
					effective = loaded.ResolveForApply(defaults)
				} else if defaults != nil {
					effective = defaults
				} else {
					effective = &profile.Profile{}
				}
				explanations, err := loaded.Explain(defaults)
				if err != nil {
					return nil, err
				}
				for _, item := range explanations {
					var stored any
					if loaded != nil {
						stored, _ = profile.ReadField(loaded, item.Field)
					}
					effectiveValue, _ := profile.ReadField(effective, item.Field)
					entry.Fields = append(entry.Fields, gameFieldOracle{
						Field:         item.Field,
						Kept:          !droppedFields[item.Field],
						Stored:        stored,
						Overridden:    loaded.IsOverridden(item.Field),
						Effective:     effectiveValue,
						Source:        string(item.Source),
						Origin:        string(item.Origin),
						AllGamesSets:  item.AllGamesSets,
						AllGamesValue: item.DefaultValue,
					})
				}
			}
		}
		result.Games = append(result.Games, entry)
	}

	listed, err := profile.List()
	result.ListError = errorText(err)
	result.List = []uint64{}
	for appID := range listed {
		result.List = append(result.List, appID)
	}
	slices.Sort(result.List)
	return result, nil
}

type configOracle struct {
	Note          string         `json:"note"`
	File          string         `json:"file"`
	Exists        bool           `json:"exists"`
	LoadError     *string        `json:"load_error"`
	Effective     map[string]any `json:"effective"`
	ValidateError *string        `json:"validate_error"`
	DLLCacheDir   string         `json:"dll_cache_dir"`
	BackupDir     string         `json:"backup_files_dir"`
	ShaderDir     string         `json:"shader_cache_dir"`
}

// configOracleFor records Go's view of the current case's config.yaml: the
// effective values after defaults are merged in, and whether Go's own option
// table accepts them.
func configOracleFor(c *fixtureCase, note string) (*configOracle, error) {
	result := &configOracle{
		Note:   note,
		File:   "config/spela/config.yaml",
		Exists: fileExists(c.path("config", "spela", "config.yaml")),
	}
	loaded, err := config.Load()
	result.LoadError = errorText(err)
	if err == nil {
		data, err := yaml.Marshal(loaded)
		if err != nil {
			return nil, err
		}
		result.Effective = map[string]any{}
		if err := yaml.Unmarshal(data, &result.Effective); err != nil {
			return nil, err
		}
		result.ValidateError = errorText(config.Validate(loaded))
	}
	result.DLLCacheDir = config.DLLCacheDir()
	result.BackupDir = config.BackupFilesDir()
	result.ShaderDir = config.ShaderCacheDir()
	return result, nil
}

type denyCheck struct {
	AppID  uint64 `json:"app_id"`
	Denied bool   `json:"denied"`
	Reason string `json:"reason"`
}

type listsOracle struct {
	Note           string              `json:"note"`
	OverridesError *string             `json:"overrides_load_error"`
	Overrides      *denylist.Overrides `json:"overrides"`
	DenyListError  *string             `json:"denylist_load_error"`
	DenyList       *denylist.DenyList  `json:"denylist"`
	Checks         []denyCheck         `json:"is_denied"`
}

// listsOracleFor records Go's IsDenied answers for the given app ids.
func listsOracleFor(note string, appIDs []uint64) *listsOracle {
	result := &listsOracle{Note: note}
	overrides, err := denylist.LoadOverrides()
	result.OverridesError = errorText(err)
	result.Overrides = overrides
	list, err := denylist.LoadDenyList()
	result.DenyListError = errorText(err)
	result.DenyList = list
	for _, appID := range appIDs {
		denied, reason := denylist.IsDenied(appID)
		result.Checks = append(result.Checks, denyCheck{AppID: appID, Denied: denied, Reason: reason})
	}
	return result
}

type gamesOracle struct {
	Note      string            `json:"note"`
	LoadError *string           `json:"load_error"`
	AllAppIDs []uint64          `json:"all_app_ids"`
	Listed    []string          `json:"list_names_sorted"`
	WithDLSS  []string          `json:"games_with_dlss_sorted"`
	FindGame  map[string]uint64 `json:"find_game"`
	ToolNames map[string]bool   `json:"is_tool_name"`
}

// gamesOracleFor records Go's read-side view of the current case's games.yaml.
func gamesOracleFor(note string, queries []string) *gamesOracle {
	result := &gamesOracle{Note: note, FindGame: map[string]uint64{}, ToolNames: map[string]bool{}}
	database, err := game.LoadDatabase()
	result.LoadError = errorText(err)
	if err != nil {
		return result
	}
	result.AllAppIDs = []uint64{}
	for appID, entry := range database.Games {
		result.AllAppIDs = append(result.AllAppIDs, appID)
		result.ToolNames[entry.Name] = game.IsToolName(entry.Name)
	}
	slices.Sort(result.AllAppIDs)
	result.Listed = []string{}
	for _, entry := range database.List() {
		result.Listed = append(result.Listed, entry.Name)
	}
	slices.Sort(result.Listed)
	result.WithDLSS = []string{}
	for _, entry := range database.GamesWithDLSS() {
		result.WithDLSS = append(result.WithDLSS, entry.Name)
	}
	slices.Sort(result.WithDLSS)
	for _, query := range queries {
		if found := database.FindGame(query); found != nil {
			result.FindGame[query] = found.AppID
		} else {
			result.FindGame[query] = 0
		}
	}
	return result
}

type backupFileOracle struct {
	OriginalPath string `json:"original_path"`
	BackupPath   string `json:"backup_path"`
	Scheme       string `json:"name_scheme"`
	BackupExists bool   `json:"backup_file_exists"`
}

type backupOracle struct {
	Note         string             `json:"note"`
	AppID        uint64             `json:"app_id"`
	IndexPath    string             `json:"index_path"`
	LoadError    *string            `json:"load_error"`
	BackupExists bool               `json:"backup_exists"`
	Files        []backupFileOracle `json:"files"`
}

// backupOracleFor records Go's LoadBackup result for one app and which name
// scheme each backup file uses.
func backupOracleFor(note string, appID uint64) *backupOracle {
	result := &backupOracle{Note: note, AppID: appID, IndexPath: dll.GetBackupMetadataPath(appID)}
	backup, err := dll.LoadBackup(appID)
	result.LoadError = errorText(err)
	result.BackupExists = dll.BackupExists(appID)
	if backup != nil {
		for _, file := range backup.Files {
			scheme := "basename"
			if base := filepath.Base(file.BackupPath); base != filepath.Base(file.OriginalPath) && strings.HasSuffix(base, "-"+filepath.Base(file.OriginalPath)) {
				scheme = "sha256-basename"
			}
			result.Files = append(result.Files, backupFileOracle{
				OriginalPath: file.OriginalPath,
				BackupPath:   file.BackupPath,
				Scheme:       scheme,
				BackupExists: fileExists(file.BackupPath),
			})
		}
	}
	return result
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
