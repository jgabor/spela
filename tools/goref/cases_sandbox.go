package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/steam"
)

// SandboxManifestURL is the loopback manifest URL of the sandboxes. Nothing
// listens on port 9, so a fetch fails at once and never leaves the machine.
const SandboxManifestURL = "http://127.0.0.1:9/"

// sandboxApp is one app in the fixture Steam install.
type sandboxApp struct {
	AppID      uint64
	Name       string
	InstallDir string
	StateFlags int
	Library    int               // 0 is the Steam root, 1 the second library
	Files      map[string]string // relative path -> PE version, "" for a non-PE file
	Prefix     bool
	Tool       bool // has a toolmanifest.vdf
}

var sandboxApps = []sandboxApp{
	{
		AppID: 9100010, Name: "Glimmer Run", InstallDir: "Glimmer Run", StateFlags: 4, Prefix: true,
		Files: map[string]string{"bin/x64/nvngx_dlss.dll": "310.2.1", "bin/x64/nvngx_dlssg.dll": "310.2.1", "bin/x64/nvngx_dlssd.dll": "310.5.3"},
	},
	{
		AppID: 9100020, Name: "Harbor Lights", InstallDir: "HarborLights", StateFlags: 4, Prefix: true,
		Files: map[string]string{"nvngx_dlss.dll": "3.7.10", "amd_fidelityfx_vk.dll": "1.0", "libxess.dll": "1.3.1.33"},
	},
	{AppID: 9100030, Name: "Quiet Orchard", InstallDir: "Quiet Orchard", StateFlags: 4},
	{
		AppID: 9100040, Name: "Proton Fixture 9.0", InstallDir: "Proton Fixture 9.0", StateFlags: 4, Tool: true,
		Files: map[string]string{"files/lib64/wine/nvngx_dlss.dll": "1.0"},
	},
	{
		AppID: 9100050, Name: "Pending Patch", InstallDir: "Pending Patch", StateFlags: 6,
		Files: map[string]string{"nvngx_dlss.dll": "310.1.0"},
	},
	{
		AppID: 9100060, Name: "Paper Comet: Director's Cut", InstallDir: "Paper Comet", StateFlags: 4, Prefix: true,
		Files: map[string]string{"Engine/Binaries/NVNGX_DLSS.DLL": ""},
	},
	{
		AppID: 9100070, Name: "Frostline Arena", InstallDir: "Frostline Arena", StateFlags: 4, Library: 1, Prefix: true,
		Files: map[string]string{"game/nvngx_dlss.dll": "310.5.3"},
	},
}

// compatTools maps app ids to compat tool names in the fixture config.vdf,
// with "0" the global default. Names follow the shapes found on a real
// machine (see docs/phase0-checks.md).
var compatTools = [][2]string{
	{"0", "proton_experimental"},
	{"9100010", "cachyos-10.0-20260330-slr"},
	{"9100020", "GE-Proton10-34"},
	{"9100060", "GE-Proton10-34"},
	{"9100070", "GE-Proton11-7-x86_64"},
}

func vdfQuote(text string) string { return `"` + text + `"` }

// buildSteamInstall writes the fixture Steam root under HOME and a second
// library, with app manifests, game files, prefixes and compat tools.
func buildSteamInstall(c *fixtureCase) error {
	steamRoot := c.path("home", ".steam", "steam")
	libraries := []string{steamRoot, c.path("library2")}

	var folders strings.Builder
	folders.WriteString("\"libraryfolders\"\n{\n")
	for index, library := range libraries {
		fmt.Fprintf(&folders, "\t%s\n\t{\n", vdfQuote(fmt.Sprint(index)))
		fmt.Fprintf(&folders, "\t\t\"path\"\t\t%s\n", vdfQuote(library))
		fmt.Fprintf(&folders, "\t\t\"label\"\t\t\"\"\n")
		fmt.Fprintf(&folders, "\t\t\"contentid\"\t\t\"100000000000000000%d\"\n", index)
		fmt.Fprintf(&folders, "\t\t\"totalsize\"\t\t\"0\"\n")
		folders.WriteString("\t\t\"apps\"\n\t\t{\n")
		for _, app := range sandboxApps {
			if app.Library == index {
				fmt.Fprintf(&folders, "\t\t\t%s\t\t\"52428800\"\n", vdfQuote(uintText(app.AppID)))
			}
		}
		folders.WriteString("\t\t}\n\t}\n")
	}
	folders.WriteString("}\n")
	if err := writeText(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"), folders.String()); err != nil {
		return err
	}

	for _, app := range sandboxApps {
		steamapps := filepath.Join(libraries[app.Library], "steamapps")
		manifest := fmt.Sprintf("\"AppState\"\n{\n\t\"appid\"\t\t%s\n\t\"universe\"\t\t\"1\"\n\t\"name\"\t\t%s\n\t\"StateFlags\"\t\t\"%d\"\n\t\"installdir\"\t\t%s\n\t\"LastUpdated\"\t\t\"1761490930\"\n\t\"SizeOnDisk\"\t\t\"52428800\"\n\t\"buildid\"\t\t\"1000001\"\n}\n",
			vdfQuote(uintText(app.AppID)), vdfQuote(app.Name), app.StateFlags, vdfQuote(app.InstallDir))
		if err := writeText(filepath.Join(steamapps, "appmanifest_"+uintText(app.AppID)+".acf"), manifest); err != nil {
			return err
		}
		install := filepath.Join(steamapps, "common", app.InstallDir)
		if err := fixtureGameDir(install, app.Files); err != nil {
			return err
		}
		if err := writeText(filepath.Join(install, "fixture-game.txt"), "Fixture install folder for "+app.Name+". Not a real game.\n"); err != nil {
			return err
		}
		if app.Tool {
			if err := writeText(filepath.Join(install, "toolmanifest.vdf"), "\"manifest\"\n{\n\t\"version\" \"2\"\n\t\"commandline\" \"/proton %verb%\"\n}\n"); err != nil {
				return err
			}
		}
		if app.Prefix {
			prefix := filepath.Join(steamapps, "compatdata", uintText(app.AppID))
			if err := writeText(filepath.Join(prefix, "pfx", "drive_c", "windows", "win.ini"), "; fixture prefix\n"); err != nil {
				return err
			}
			if err := writeText(filepath.Join(prefix, "version"), "10.0-fixture\n"); err != nil {
				return err
			}
		}
	}

	var mapping strings.Builder
	mapping.WriteString("\"InstallConfigStore\"\n{\n\t\"Software\"\n\t{\n\t\t\"Valve\"\n\t\t{\n\t\t\t\"Steam\"\n\t\t\t{\n\t\t\t\t\"CompatToolMapping\"\n\t\t\t\t{\n")
	for _, entry := range compatTools {
		priority := "250"
		if entry[0] == "0" {
			priority = "75"
		}
		fmt.Fprintf(&mapping, "\t\t\t\t\t%s\n\t\t\t\t\t{\n\t\t\t\t\t\t\"name\"\t\t%s\n\t\t\t\t\t\t\"config\"\t\t\"\"\n\t\t\t\t\t\t\"priority\"\t\t%s\n\t\t\t\t\t}\n", vdfQuote(entry[0]), vdfQuote(entry[1]), vdfQuote(priority))
	}
	mapping.WriteString("\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n")
	if err := writeText(filepath.Join(steamRoot, "config", "config.vdf"), mapping.String()); err != nil {
		return err
	}

	// Compat tools: a CachyOS build with the legacy descriptor heap gate,
	// a GE build without it, and a GE build whose folder name differs from
	// its internal name (as on the real machine). No folder matches the
	// built-in proton_experimental name.
	tools := map[string]string{
		"cachyos-10.0-20260330-slr": "#!/usr/bin/env python3\n# fixture proton script\n# honours PROTON_VKD3D_HEAP\n",
		"GE-Proton10-34":            "#!/usr/bin/env python3\n# fixture proton script\n",
		"GE-Proton11-7":             "#!/usr/bin/env python3\n# fixture proton script\n",
	}
	for name, script := range tools {
		dir := filepath.Join(steamRoot, "compatibilitytools.d", name)
		if err := writeText(filepath.Join(dir, "proton"), script); err != nil {
			return err
		}
		internal := name
		if name == "GE-Proton11-7" {
			internal = "GE-Proton11-7-x86_64"
		}
		compat := fmt.Sprintf("\"compatibilitytools\"\n{\n  \"compat_tools\"\n  {\n    %s // Internal name of this tool\n    {\n      \"install_path\" \".\"\n      \"display_name\" %s\n      \"from_oslist\"  \"windows\"\n      \"to_oslist\"    \"linux\"\n    }\n  }\n}\n", vdfQuote(internal), vdfQuote(internal))
		if err := writeText(filepath.Join(dir, "compatibilitytool.vdf"), compat); err != nil {
			return err
		}
	}
	// The built-in tool folder name differs from its mapping name.
	return writeText(filepath.Join(steamRoot, "steamapps", "common", "Proton - Experimental", "toolmanifest.vdf"), "\"manifest\"\n{\n\t\"version\" \"2\"\n\t\"commandline\" \"/proton %verb%\"\n}\n")
}

// scanSandbox runs Go's Steam scan over the fixture install, pins the scan
// times and saves the result through Go's game database transaction.
func scanSandbox() error {
	scanned, err := steam.ScanLibraries("", nil)
	if err != nil {
		return err
	}
	if scanned == nil {
		return fmt.Errorf("the scan found no Steam install")
	}
	ids := make([]uint64, 0, len(scanned.Games))
	for appID := range scanned.Games {
		ids = append(ids, appID)
	}
	slices.Sort(ids)
	for index, appID := range ids {
		scanned.Games[appID].ScannedAt = pinnedTime.Add(time.Duration(index) * time.Second)
	}
	_, err = game.Transaction(func(database *game.Database) (bool, error) {
		*database = *scanned
		return true, nil
	})
	return err
}

// writeSandboxConfig saves the sandbox config: no rescans, no update
// checks, no manifest refresh, and a loopback manifest URL.
func writeSandboxConfig() error {
	configuration := config.Default()
	configuration.RescanOnStartup = false
	configuration.CheckUpdates = false
	configuration.AutoRefreshManifest = false
	configuration.DLLManifestURL = SandboxManifestURL
	return configuration.Save()
}

// seedManifestCache writes the repository manifest as the cache, fetched at
// the pinned time from the sandbox URL, so Go never tries to fetch.
func (w *fixtureWriter) seedManifestCache(c *fixtureCase) error {
	manifest, err := w.readRepositoryManifest()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cachedManifest{Manifest: *manifest, FetchedAt: pinnedTime, SourceURL: SandboxManifestURL}, "", "  ")
	if err != nil {
		return err
	}
	return writeBytes(c.path("cache", "spela", "manifest.json"), data)
}

func (w *fixtureWriter) buildMainSandbox(c *fixtureCase) error {
	if err := buildSteamInstall(c); err != nil {
		return err
	}
	if err := writeSandboxConfig(); err != nil {
		return err
	}
	if err := w.seedManifestCache(c); err != nil {
		return err
	}
	if err := scanSandbox(); err != nil {
		return err
	}
	if err := saveAllGames([]fieldValue{
		{profile.FieldProtonEnableWayland, true},
		{profile.FieldDLSSSRPreset, "auto"},
		{profile.FieldGPUShaderCache, true},
		{profile.FieldCPUGovernor, "performance"},
	}); err != nil {
		return err
	}
	if err := saveGame(9100010, "Glimmer Run", []fieldValue{
		{profile.FieldDLSSSRMode, "quality"},
		{profile.FieldDLSSSRPreset, "K"},
		{profile.FieldDLSSSROverride, true},
		{profile.FieldDLSSFGEnabled, true},
		{profile.FieldDLSSFGOverride, true},
		{profile.FieldDLSSMultiFrame, 3},
		{profile.FieldDLSSIndicator, true},
		{profile.FieldProtonEnableHDR, true},
		{profile.FieldProtonVKD3DHeap, true},
		{profile.FieldGPUPowerLimit, 250},
		{profile.FieldGPUThreadedOptimization, false},
		{profile.FieldGPUVRR, "automatic"},
		{profile.FieldGPUClockOffset, 150},
		{profile.FieldGPUMemoryOffset, 500},
		{profile.FieldGPUFanSpeed, 60},
		{profile.FieldCPUSMT, false},
		{profile.FieldOverlayEnabled, true},
		{profile.FieldOverlayPosition, "top-right"},
	}); err != nil {
		return err
	}
	// profile create: a profile with only its name.
	if _, err := profile.Create(9100020, &profile.Profile{Name: "Harbor Lights"}); err != nil {
		return err
	}
	if err := saveGame(9100070, "Frostline Arena", []fieldValue{
		{profile.FieldProtonEnableWayland, false},
		{profile.FieldProtonVKD3DHeap, true},
		{profile.FieldDLSSRRMode, "dlaa"},
		{profile.FieldDLSSRRPreset, "D"},
		{profile.FieldDLSSRROverride, true},
		{profile.FieldGPUShaderCachePath, "/var/cache/fixture-shaders/frostline"},
	}); err != nil {
		return err
	}
	// descriptor_heap on a GE build Go can resolve: Go prints a notice.
	if err := saveGame(9100060, "Paper Comet: Director's Cut", []fieldValue{{profile.FieldProtonVKD3DHeap, true}}); err != nil {
		return err
	}
	if err := denylist.Deny(9100070, "Frostline Arena", "kernel anti-cheat"); err != nil {
		return err
	}
	if err := denylist.Allow(1172470); err != nil {
		return err
	}
	database, err := game.LoadDatabase()
	if err != nil {
		return err
	}
	harbor := database.GetGame(9100020)
	if harbor == nil {
		return fmt.Errorf("scan missed Harbor Lights")
	}
	return createPinnedBackup(9100020, harbor.Name, harbor.DLLs)
}

func (w *fixtureWriter) writeSandboxes() error {
	c, err := w.enterCase("sandbox/main")
	if err != nil {
		return err
	}
	if err := w.buildMainSandbox(c); err != nil {
		return err
	}
	w.note("sandbox/main/", "The transcript sandbox: a fixture Steam install under `home/.steam/steam` plus `library2` (two libraries, PE stub DLLs, prefixes, a tool, a game with a pending update, compat tools and config.vdf), config.yaml with rescans, update checks and manifest refresh off and a loopback manifest URL, a seeded manifest cache, games.yaml from Go's own scan, All games and four game profiles, deny overrides and one backup", "`steam.ScanLibraries`, `game.Transaction`, `(*Config).Save`, `profile.Mutate`, `profile.MutateDefault`, `profile.Create`, `denylist.Deny`, `denylist.Allow`, `dll.CreateBackup`; Steam files and the manifest cache are hand-built")
	if err := w.profileOracle(c, "the sandbox profiles", 9100010, 9100020, 9100030, 9100060, 9100070); err != nil {
		return err
	}

	c, err = w.enterCase("sandbox/errors")
	if err != nil {
		return err
	}
	if err := w.buildMainSandbox(c); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100030.yaml"), "name: Quiet Orchard\ndlss: [broken\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100060.yaml"), "name: Paper Comet\ngpu:\n    vrr: sometimes\noverrides:\n    gpu.vrr: true\n"); err != nil {
		return err
	}
	w.note("sandbox/errors/", "The main sandbox plus a game profile that does not parse (9100030) and one with an invalid `gpu.vrr` (9100060), for error transcripts", "as `sandbox/main`, plus literal YAML")

	c, err = w.enterCase("sandbox/empty")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.path("home"), 0o755); err != nil {
		return err
	}
	if err := writeText(c.path("home", "fixture-home.txt"), "An empty home: no Steam install and no spela files.\n"); err != nil {
		return err
	}
	w.note("sandbox/empty/", "An empty home with no Steam install and no spela files, for empty-state transcripts", "nothing; Go writes only what the commands create")
	return nil
}

// writeE2ELibrary copies the setup of tests/e2e/mock_helper.go
// (setupTestEnvironment) with pinned times, written with yaml.v3 the same
// way the e2e harness does. The harness pins three real games; here they
// are fictional names and app ids (decision 43), with the same shape:
// two DLLs, one DLL and none, and an install folder whose case differs
// from the game's name.
func (w *fixtureWriter) writeE2ELibrary() error {
	c, err := w.enterCase("e2e/library")
	if err != nil {
		return err
	}
	stamp := pinnedTime.Format(time.RFC3339)
	fixtureRoot := c.Dir
	avenueDir := filepath.Join(fixtureRoot, "games", "Chrome Avenue 2081")
	crownDir := filepath.Join(fixtureRoot, "games", "Hollow Crown 3")
	throneDir := filepath.Join(fixtureRoot, "games", "ASHEN THRONE")
	for _, dir := range []string{filepath.Join(avenueDir, "bin", "x64"), filepath.Join(crownDir, "bin"), throneDir, c.path("home", ".steam", "steam", "steamapps")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	for path, content := range map[string]string{
		filepath.Join(avenueDir, "bin", "x64", "nvngx_dlss.dll"):      "DLSS 3.7.0",
		filepath.Join(avenueDir, "bin", "x64", "nvngx_dlssg.dll"):     "DLSSG 3.7.0",
		filepath.Join(crownDir, "bin", "nvngx_dlss.dll"):              "DLSS 3.5.0",
		filepath.Join(throneDir, "fixture-game.txt"):                  "Fixture install folder. The e2e harness leaves it empty.\n",
		c.path("home", ".steam", "steam", "steamapps", "fixture.txt"): "The e2e harness makes this folder empty.\n",
	} {
		if err := writeText(path, content); err != nil {
			return err
		}
	}
	gamesData := map[string]interface{}{
		"games": map[uint64]interface{}{
			9100110: map[string]interface{}{
				"app_id": 9100110, "name": "Chrome Avenue 2081", "install_dir": avenueDir,
				"prefix_path": filepath.Join(fixtureRoot, "compatdata", "9100110", "pfx"), "scanned_at": stamp,
				"library_path": filepath.Join(fixtureRoot, "games"),
				"dlls": []map[string]interface{}{
					{"path": filepath.Join(avenueDir, "bin", "x64", "nvngx_dlss.dll"), "name": "nvngx_dlss.dll", "type": "dlss", "version": "3.7.0"},
					{"path": filepath.Join(avenueDir, "bin", "x64", "nvngx_dlssg.dll"), "name": "nvngx_dlssg.dll", "type": "dlssg", "version": "3.7.0"},
				},
			},
			9100120: map[string]interface{}{
				"app_id": 9100120, "name": "Hollow Crown 3: Long Night", "install_dir": crownDir,
				"prefix_path": filepath.Join(fixtureRoot, "compatdata", "9100120", "pfx"), "scanned_at": stamp,
				"library_path": filepath.Join(fixtureRoot, "games"),
				"dlls": []map[string]interface{}{
					{"path": filepath.Join(crownDir, "bin", "nvngx_dlss.dll"), "name": "nvngx_dlss.dll", "type": "dlss", "version": "3.5.0"},
				},
			},
			9100130: map[string]interface{}{
				"app_id": 9100130, "name": "Ashen Throne", "install_dir": throneDir,
				"prefix_path": filepath.Join(fixtureRoot, "compatdata", "9100130", "pfx"), "scanned_at": stamp,
				"library_path": filepath.Join(fixtureRoot, "games"), "dlls": []interface{}{},
			},
		},
		"updated_at": stamp,
	}
	configData := map[string]interface{}{
		"log_level": "info", "shader_cache": filepath.Join(fixtureRoot, "nvidia"), "check_updates": false,
		"rescan_on_startup": false, "auto_update_dlls": false, "auto_refresh_manifest": false,
		"manifest_refresh_hours": 24, "compact_mode": false, "confirm_destructive": true,
	}
	defaultProfileData := map[string]interface{}{
		"dlss":   map[string]interface{}{"sr_mode": "balanced", "sr_preset": "default", "sr_override": false, "fg_enabled": false, "fg_override": false},
		"proton": map[string]interface{}{"enable_hdr": false, "enable_wayland": false, "enable_ngx_updater": false},
	}
	avenueProfileData := map[string]interface{}{
		"dlss":   map[string]interface{}{"sr_mode": "balanced", "sr_preset": "quality", "sr_override": false, "fg_enabled": true, "fg_override": true},
		"proton": map[string]interface{}{"enable_hdr": true, "enable_wayland": false, "enable_ngx_updater": false},
		"overrides": map[string]bool{
			"dlss.sr_mode": true, "dlss.sr_preset": true, "dlss.sr_override": true, "dlss.fg_enabled": true, "dlss.fg_override": true,
			"proton.enable_hdr": true, "proton.enable_wayland": true, "proton.enable_ngx_updater": true,
		},
	}
	for path, data := range map[string]interface{}{
		c.path("data", "spela", "games.yaml"):                 gamesData,
		c.path("config", "spela", "config.yaml"):              configData,
		c.path("config", "spela", "profiles", "default.yaml"): defaultProfileData,
		c.path("config", "spela", "profiles", "9100110.yaml"): avenueProfileData,
	} {
		encoded, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		if err := writeBytes(path, encoded); err != nil {
			return err
		}
	}
	manifestJSON := fmt.Sprintf(`{
  "version": "1.0",
  "updated_at": "%s",
  "repository": "jgabor/spela",
  "dlls": {
    "dlss": [
      {
        "version": "3.8.0",
        "filename": "nvngx_dlss_3.8.0.dll",
        "url": "http://localhost:12345/dlls/nvngx_dlss_3.8.0.dll",
        "sha256": "abcdef1234567890",
        "size": 1024,
        "release_date": "2026-05-22T21:00:00Z"
      }
    ],
    "dlssg": [
      {
        "version": "3.7.0",
        "filename": "nvngx_dlssg_3.7.0.dll",
        "url": "http://localhost:12345/dlls/nvngx_dlssg_3.7.0.dll",
        "sha256": "abcdef1234567890",
        "size": 1024,
        "release_date": "2026-05-22T21:00:00Z"
      }
    ]
  }
}`, stamp)
	if err := writeText(c.path("cache", "spela", "manifest.json"), manifestJSON); err != nil {
		return err
	}
	w.note("e2e/library/", "The Go e2e harness's three-game library in fictional form (Chrome Avenue 2081 with DLSS and DLSS-G 3.7.0, Hollow Crown 3 with DLSS 3.5.0, Ashen Throne with none), config.yaml, a default.yaml with no overrides key, 9100110.yaml and manifest.json", "a copy of `setupTestEnvironment` in `tests/e2e/mock_helper.go` (yaml.v3 `Marshal` of the same maps) with fictional names and app ids (decision 43), times pinned")
	if err := w.profileOracle(c, "the e2e profiles", 9100110, 9100120, 9100130); err != nil {
		return err
	}
	return writeJSON(c.path("oracle-games.json"), gamesOracleFor("the e2e game database", []string{"9100110", "Ashen Throne", "Hollow Crown 3: Long Night"}))
}
