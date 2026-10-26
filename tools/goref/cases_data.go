package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/release"
)

// builtInDenied are a few app ids from Go's built-in deny list, which Go
// tests pin.
var builtInDenied = []uint64{1172470, 252490, 1623660}

func (w *fixtureWriter) writeListCases() error {
	checks := append(append([]uint64{}, builtInDenied...), 9100201, 9100202, 9100203, 9100299)

	c, err := w.enterCase("lists/overrides-empty")
	if err != nil {
		return err
	}
	if err := denylist.SaveOverrides(&denylist.Overrides{}); err != nil {
		return err
	}
	w.note("lists/overrides-empty/", "Empty `overrides.yaml` as Go writes it (`allowed: []`, `denied: []`)", "`denylist.SaveOverrides`")
	if err := writeJSON(c.path("oracle.json"), listsOracleFor("empty overrides: the built-in list applies", checks)); err != nil {
		return err
	}

	c, err = w.enterCase("lists/overrides-full")
	if err != nil {
		return err
	}
	steps := []func() error{
		func() error { return denylist.Allow(1172470) },
		func() error { return denylist.Deny(9100201, "Frostline Arena", "kernel anti-cheat") },
		func() error { return denylist.Deny(9100202, "9100202", "user-specified") },
		// One game in both lists: Go's IsDenied lets allow win.
		func() error { return denylist.Allow(9100203) },
		func() error { return denylist.Deny(9100203, "Both Lists", "user-specified") },
		// A second Deny of the same app replaces its name and reason.
		func() error { return denylist.Deny(9100201, "Frostline Arena", "kernel anti-cheat, updated") },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	w.note("lists/overrides-full/", "`overrides.yaml` with a force-allowed built-in game, user denials (one with a numeric name, as `denylist deny` writes for an unknown id), and one game in both lists", "`denylist.Allow`, `denylist.Deny`")
	if err := writeJSON(c.path("oracle.json"), listsOracleFor("allow beats deny; a deny of a built-in game is not needed", checks)); err != nil {
		return err
	}

	c, err = w.enterCase("lists/denylist-user")
	if err != nil {
		return err
	}
	user := denylist.DenyList{Entries: []denylist.Entry{
		{AppID: 9100201, Name: "Frostline Arena", Reason: "kernel anti-cheat"},
		{AppID: 9100204, Name: "Night Courier", Reason: "Easy Anti-Cheat"},
	}}
	data, err := yaml.Marshal(&user)
	if err != nil {
		return err
	}
	if err := writeBytes(c.path("config", "spela", "lists", "denylist.yaml"), data); err != nil {
		return err
	}
	if err := denylist.Deny(9100202, "Pocket Raid", "user-specified"); err != nil {
		return err
	}
	w.note("lists/denylist-user/", "A user `denylist.yaml` (Go never writes it; it replaces the built-in list in Go) next to an overrides file", "yaml.v3 `Marshal` of `denylist.DenyList`; overrides by `denylist.Deny`")
	return writeJSON(c.path("oracle.json"), listsOracleFor("a user denylist.yaml replaces Go's built-in list, so built-in games read as allowed", append(checks, 9100204)))
}

// fixtureGameDir makes a fixture game install with the given DLL files.
// A version string makes a PE stub with that file version; an empty one
// writes a text file that is not a DLL.
func fixtureGameDir(root string, files map[string]string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for relative, version := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		var data []byte
		if version == "" {
			data = []byte("fixture placeholder, not a DLL\n")
		} else {
			data = peStubVersion(version)
		}
		if err := writeBytes(path, data); err != nil {
			return err
		}
	}
	return nil
}

func (w *fixtureWriter) writeGameCases() error {
	c, err := w.enterCase("games/database")
	if err != nil {
		return err
	}
	library := c.path("library")
	common := filepath.Join(library, "steamapps", "common")
	installs := map[string]map[string]string{
		"Glimmer Run":   {"bin/x64/nvngx_dlss.dll": "310.2.1", "bin/x64/nvngx_dlssg.dll": "310.2.1.0", "bin/x64/nvngx_dlssd.dll": "310.1.0"},
		"Harbor Lights": {"nvngx_dlss.dll": "3.7.10", "amd_fidelityfx_vk.dll": "1.0", "libxess.dll": "1.3.1.33"},
		"Paper Comet":   {"NVNGX_DLSS.DLL": ""},
	}
	for name, files := range installs {
		if err := fixtureGameDir(filepath.Join(common, name), files); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(common, "Quiet Orchard"), 0o755); err != nil {
		return err
	}
	scanned := func(appID uint64, name string, prefix bool, offset time.Duration) *game.Game {
		entry := &game.Game{
			AppID:       appID,
			Name:        name,
			InstallDir:  filepath.Join(common, name),
			LibraryPath: library,
			ScannedAt:   pinnedTime.Add(offset),
		}
		if prefix {
			entry.PrefixPath = filepath.Join(library, "steamapps", "compatdata", uintText(appID))
		}
		entry.DLLs, _ = dll.ScanDirectory(entry.InstallDir)
		return entry
	}
	_, err = game.Transaction(func(database *game.Database) (bool, error) {
		database.AddGame(scanned(9100301, "Glimmer Run", true, 0))
		database.AddGame(scanned(9100302, "Harbor Lights", true, time.Second))
		database.AddGame(scanned(9100303, "Paper Comet", false, 2*time.Second))
		database.AddGame(scanned(9100304, "Quiet Orchard", false, 3*time.Second))
		// Older Go versions saved tools before the scan filtered them.
		tool := scanned(9100305, "Proton 9.0 (Fixture)", false, 4*time.Second)
		tool.InstallDir = filepath.Join(common, "Proton 9.0")
		database.AddGame(tool)
		runtime := scanned(9100306, "Steam Linux Runtime 3.0 (sniper)", false, 5*time.Second)
		runtime.InstallDir = filepath.Join(common, "SteamLinuxRuntime_sniper")
		database.AddGame(runtime)
		// A time in UTC, to show both zone forms.
		utc := scanned(9100307, "Signal Drift: Director's Cut", false, 0)
		utc.ScannedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		database.AddGame(utc)
		return true, nil
	})
	if err != nil {
		return err
	}
	w.note("games/database/", "`games.yaml` with DLLs of every family and versions in each formatter shape (2, 3 and 4 parts), a DLL with no readable version and an upper-case name, games with and without a prefix, Steam tools that old scans kept, a name with a colon and an apostrophe, local and UTC times, and the fixture game folders with PE stubs", "`game.Transaction`, `(*Database).AddGame`, `dll.ScanDirectory`")
	return writeJSON(c.path("oracle.json"), gamesOracleFor("Go's read side filters tool names; FindGame matches an id first, then an exact name", []string{"9100301", "Glimmer Run", "glimmer run", "9100305", "Proton 9.0 (Fixture)", "Signal Drift: Director's Cut", "9100399"}))
}

func (w *fixtureWriter) writeBackupCases() error {
	// The sha256 scheme, written by CreateBackup before a swap.
	c, err := w.enterCase("backups/sha256-names")
	if err != nil {
		return err
	}
	install := c.path("library", "steamapps", "common", "Glimmer Run")
	if err := fixtureGameDir(install, map[string]string{"bin/x64/nvngx_dlss.dll": "310.2.1", "bin/x64/nvngx_dlssg.dll": "310.2.1", "plugins/nvngx_dlss.dll": "3.5.0"}); err != nil {
		return err
	}
	detected, err := dll.ScanDirectory(install)
	if err != nil {
		return err
	}
	if err := createPinnedBackup(9100401, "Glimmer Run", detected); err != nil {
		return err
	}
	w.note("backups/sha256-names/", "`backup.json` and backup files named `<sha256 of the original path>-<basename>`, two of them with the same basename in different folders, and the game install they came from", "`dll.CreateBackup`, then `dll.SaveBackup` to pin `created_at`")
	if err := writeJSON(c.path("oracle.json"), backupOracleFor("Go's current backup name scheme", 9100401)); err != nil {
		return err
	}

	// The older basename scheme, as Go's swap_test writes it.
	c, err = w.enterCase("backups/basename-names")
	if err != nil {
		return err
	}
	install = c.path("library", "steamapps", "common", "Harbor Lights")
	if err := fixtureGameDir(install, map[string]string{"nvngx_dlss.dll": "310.5.3"}); err != nil {
		return err
	}
	backupDir := filepath.Join(config.DefaultBackupDir(), "9100402")
	legacyPath := filepath.Join(backupDir, "nvngx_dlss.dll")
	if err := writeBytes(legacyPath, peStubVersion("3.7.10")); err != nil {
		return err
	}
	if err := dll.SaveBackup(&dll.Backup{
		AppID: 9100402, GameName: "Harbor Lights", CreatedAt: pinnedTime, BackupPath: backupDir,
		Files: []dll.BackedUpFile{{OriginalPath: filepath.Join(install, "nvngx_dlss.dll"), BackupPath: legacyPath, DLLName: "nvngx_dlss.dll", Version: "3.7.10"}},
	}); err != nil {
		return err
	}
	w.note("backups/basename-names/", "The older scheme where the backup file is the plain basename, as TestLegacyBasenameBackupMetadataRestores writes it; the game holds a newer swapped DLL", "`dll.SaveBackup`")
	if err := writeJSON(c.path("oracle.json"), backupOracleFor("Go's older backup name scheme", 9100402)); err != nil {
		return err
	}

	// Both schemes in one index: a basename entry, then CreateBackup adds a
	// DLL it had not backed up before.
	c, err = w.enterCase("backups/mixed-names")
	if err != nil {
		return err
	}
	install = c.path("library", "steamapps", "common", "Lantern Keep")
	if err := fixtureGameDir(install, map[string]string{"nvngx_dlss.dll": "310.5.3", "nvngx_dlssg.dll": "310.1.0"}); err != nil {
		return err
	}
	backupDir = filepath.Join(config.DefaultBackupDir(), "9100403")
	legacyPath = filepath.Join(backupDir, "nvngx_dlss.dll")
	if err := writeBytes(legacyPath, peStubVersion("3.1.30")); err != nil {
		return err
	}
	if err := dll.SaveBackup(&dll.Backup{
		AppID: 9100403, GameName: "Lantern Keep", CreatedAt: pinnedTime, BackupPath: backupDir,
		Files: []dll.BackedUpFile{{OriginalPath: filepath.Join(install, "nvngx_dlss.dll"), BackupPath: legacyPath, DLLName: "nvngx_dlss.dll", Version: "3.1.30"}},
	}); err != nil {
		return err
	}
	detected, err = dll.ScanDirectory(install)
	if err != nil {
		return err
	}
	if _, err := dll.CreateBackup(9100403, "Lantern Keep", detected); err != nil {
		return err
	}
	w.note("backups/mixed-names/", "One index with a basename entry and a sha256 entry that CreateBackup added later; the first backup of a path is kept", "`dll.SaveBackup`, then `dll.CreateBackup`")
	if err := writeJSON(c.path("oracle.json"), backupOracleFor("both name schemes in one index", 9100403)); err != nil {
		return err
	}

	// backup_path set: files go to the custom folder, the index stays in
	// the data folder.
	c, err = w.enterCase("backups/custom-backup-path")
	if err != nil {
		return err
	}
	configuration := config.Default()
	configuration.BackupPath = c.path("dll-backups")
	if err := configuration.Save(); err != nil {
		return err
	}
	install = c.path("library", "steamapps", "common", "Paper Comet")
	if err := fixtureGameDir(install, map[string]string{"nvngx_dlss.dll": "310.4.0"}); err != nil {
		return err
	}
	detected, err = dll.ScanDirectory(install)
	if err != nil {
		return err
	}
	if err := createPinnedBackup(9100404, "Paper Comet", detected); err != nil {
		return err
	}
	w.note("backups/custom-backup-path/", "`backup_path` set in config.yaml: backup files sit under it while `backup.json` stays in `data/spela/backups/<appid>/`", "`(*Config).Save`, `dll.CreateBackup`, `dll.SaveBackup`")
	if err := writeJSON(c.path("oracle.json"), backupOracleFor("the index never moves with backup_path", 9100404)); err != nil {
		return err
	}

	// Indexes Go refuses: wrong app id, no files, an empty path.
	c, err = w.enterCase("backups/invalid-index")
	if err != nil {
		return err
	}
	for appID, index := range map[uint64]dll.Backup{
		9100405: {AppID: 9100499, GameName: "Wrong Id", CreatedAt: pinnedTime, BackupPath: "x", Files: []dll.BackedUpFile{{OriginalPath: "/a", BackupPath: "/b", DLLName: "nvngx_dlss.dll"}}},
		9100406: {AppID: 9100406, GameName: "No Files", CreatedAt: pinnedTime, BackupPath: "x"},
		9100407: {AppID: 9100407, GameName: "Empty Path", CreatedAt: pinnedTime, BackupPath: "x", Files: []dll.BackedUpFile{{OriginalPath: "", BackupPath: "/b", DLLName: "nvngx_dlss.dll"}}},
	} {
		data, err := json.MarshalIndent(index, "", "  ")
		if err != nil {
			return err
		}
		if err := writeBytes(dll.GetBackupMetadataPath(appID), data); err != nil {
			return err
		}
	}
	if err := writeText(dll.GetBackupMetadataPath(9100408), "{\"app_id\": 9100408, \"files\": [\n"); err != nil {
		return err
	}
	w.note("backups/invalid-index/", "Backup indexes Go's LoadBackup refuses: wrong app id, no files, an empty path, JSON that does not parse", "`json.MarshalIndent` of `dll.Backup` (SaveBackup's encoder) into the index path; the last one literal")
	oracles := []*backupOracle{}
	for _, appID := range []uint64{9100405, 9100406, 9100407, 9100408} {
		oracles = append(oracles, backupOracleFor("LoadBackup fails closed", appID))
	}
	return writeJSON(c.path("oracle.json"), oracles)
}

// createPinnedBackup runs Go's CreateBackup, then saves the same index with
// a pinned created_at through SaveBackup.
func createPinnedBackup(appID uint64, name string, detected []game.DetectedDLL) error {
	backup, err := dll.CreateBackup(appID, name, detected)
	if err != nil {
		return err
	}
	backup.CreatedAt = pinnedTime
	return dll.SaveBackup(backup)
}

// cachedManifest has the shape of dll's unexported cache type: the manifest
// plus fetched_at and source_url, in Go's field order.
type cachedManifest struct {
	dll.Manifest
	FetchedAt time.Time `json:"fetched_at,omitzero"`
	SourceURL string    `json:"source_url,omitempty"`
}

func (w *fixtureWriter) readRepositoryManifest() (*dll.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(w.repo, "data", "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest dll.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (w *fixtureWriter) writeCacheCases() error {
	manifest, err := w.readRepositoryManifest()
	if err != nil {
		return err
	}

	if _, err := w.enterCase("caches/manifest-with-fetch-time"); err != nil {
		return err
	}
	if err := dll.SaveManifest(manifest); err != nil {
		return err
	}
	w.note("caches/manifest-with-fetch-time/", "Manifest cache with `fetched_at` and `source_url`, from the repository's data/manifest.json", "`dll.SaveManifest`")

	c, err := w.enterCase("caches/manifest-without-fetch-time")
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := writeBytes(c.path("cache", "spela", "manifest.json"), data); err != nil {
		return err
	}
	w.note("caches/manifest-without-fetch-time/", "A manifest cache from before `fetched_at` and `source_url` (Go then uses the file time and the default URL)", "`json.MarshalIndent` of `dll.Manifest`, Go's cache encoder")

	// Release check state, written by release.Check against a loopback
	// server that serves a tag list.
	c, err = w.enterCase("caches/update-check")
	if err != nil {
		return err
	}
	if err := releaseCheckAgainst(`[{"name":"v0.7.0"},{"name":"v0.6.1"},{"name":"v0.8.0-rc1"},{"name":"dll-dlss-v310.5.3"}]`, http.StatusOK); err != nil {
		return err
	}
	if err := os.Rename(c.path("cache", "spela", "update-check.json"), c.path("cache", "spela", "update-check.ok.json")); err != nil {
		return err
	}
	if err := releaseCheckAgainst(`rate limited`, http.StatusForbidden); err != nil {
		return err
	}
	if err := os.Rename(c.path("cache", "spela", "update-check.json"), c.path("cache", "spela", "update-check.failed.json")); err != nil {
		return err
	}
	w.note("caches/update-check/", "`update-check.json` after a check that found v0.7.0 (`update-check.ok.json`) and after a failed first check, which has no `latest` (`update-check.failed.json`); rename either to `update-check.json` to use it", "`release.Check` against a loopback test server")
	return nil
}

// releaseCheckAgainst runs release.Check for version 0.6.0 at the pinned
// time against a loopback server answering with body and status.
func releaseCheckAgainst(body string, status int) error {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(status)
		_, _ = response.Write([]byte(body))
	}))
	defer server.Close()
	previous := release.TagsURL
	release.TagsURL = server.URL + "/repos/jgabor/spela/tags?per_page=100"
	defer func() { release.TagsURL = previous }()
	_, _ = release.Check(context.Background(), "0.6.0", pinnedTime)
	return nil
}
