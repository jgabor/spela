package main

import (
	"github.com/jgabor/spela/internal/config"
)

// writeConfigCases writes config.yaml through config.Save, and the shapes Go
// can no longer write as literal YAML that Go still reads.
func (w *fixtureWriter) writeConfigCases() error {
	// Defaults, as Go writes them on the first save.
	c, err := w.enterCase("config/default")
	if err != nil {
		return err
	}
	if err := config.Default().Save(); err != nil {
		return err
	}
	if err := w.configOracle(c, "config.Default().Save(): Go writes every key, defaults included"); err != nil {
		return err
	}
	w.note("config/default/", "Default config as Go saves it; shader_cache is the computed `<cache>/nvidia`", "`config.Default()`, `(*Config).Save`")

	// Every key changed from its default.
	c, err = w.enterCase("config/every-key")
	if err != nil {
		return err
	}
	every := config.Default()
	every.LogLevel = config.LogLevelDebug
	every.ShaderCache = c.path("shader-cache")
	every.CheckUpdates = false
	every.RescanOnStartup = false
	every.AutoUpdateDLLs = true
	every.SteamPath = c.path("home", ".local", "share", "Steam")
	every.AdditionalLibraryPaths = []string{c.path("library-a"), c.path("library b")}
	every.DLLCachePath = c.path("dll-cache")
	every.BackupPath = c.path("dll-backups")
	every.DLLManifestURL = "https://manifest.example.invalid/manifest.json"
	every.AutoRefreshManifest = false
	every.ManifestRefreshHours = 168
	every.Theme = "dark"
	every.CompactMode = true
	every.ConfirmDestructive = false
	if err := every.Save(); err != nil {
		return err
	}
	if err := w.configOracle(c, "every key set to a non-default value, including the GUI theme and compact_mode"); err != nil {
		return err
	}
	w.note("config/every-key/", "Every config key set away from its default, including `theme`, `compact_mode`, a two-entry library path list with a space", "`(*Config).Save`")

	// Explicit zero and empty values.
	c, err = w.enterCase("config/explicit-zeros")
	if err != nil {
		return err
	}
	zeros := config.Default()
	zeros.ShaderCache = ""
	zeros.ManifestRefreshHours = 0
	zeros.Theme = ""
	zeros.AdditionalLibraryPaths = []string{}
	zeros.LogLevel = ""
	if err := zeros.Save(); err != nil {
		return err
	}
	if err := w.configOracle(c, "empty shader_cache and log_level, refresh hours 0, empty theme (omitempty drops it, so Go reads back the default theme)"); err != nil {
		return err
	}
	w.note("config/explicit-zeros/", "Explicit empty and zero values: `shader_cache: \"\"`, `log_level: \"\"`, `manifest_refresh_hours: 0`, and the `omitempty` keys Go leaves out", "`(*Config).Save`")

	// Hand-written partial file: missing keys take the defaults.
	c, err = w.enterCase("config/partial")
	if err != nil {
		return err
	}
	if err := writeText(c.path("config", "spela", "config.yaml"), "check_updates: false\nadditional_library_paths:\n  - "+c.path("games")+"\n"); err != nil {
		return err
	}
	if err := w.configOracle(c, "hand-written file with two keys and 2-space indent; Go merges it over the defaults"); err != nil {
		return err
	}
	w.note("config/partial/", "Hand-written config with only two keys and 2-space indentation; the oracle shows the defaults Go fills in", "literal YAML, read by `config.Load`")

	// Retired, unknown and invalid values Go still loads.
	c, err = w.enterCase("config/retired-keys")
	if err != nil {
		return err
	}
	legacy := "log_level: verbose\n" +
		"shader_cache: " + c.path("cache", "spela", "nvidia") + "\n" +
		"check_updates: true\n" +
		"rescan_on_startup: true\n" +
		"auto_update_dlls: false\n" +
		"auto_refresh_manifest: true\n" +
		"manifest_refresh_hours: 24\n" +
		"theme: light\n" +
		"compact_mode: false\n" +
		"confirm_destructive: true\n" +
		"show_hints: true\n" +
		"preferred_dll_source: techpowerup\n" +
		"fancy_mode: 3\n"
	if err := writeText(c.path("config", "spela", "config.yaml"), legacy); err != nil {
		return err
	}
	if err := w.configOracle(c, "retired keys show_hints and preferred_dll_source, an unknown key, and an unknown log level: Go loads it and ignores the unknown keys; the log level fails Validate"); err != nil {
		return err
	}
	w.note("config/retired-keys/", "Retired `show_hints` and `preferred_dll_source`, an unknown key, `theme: light` and an unknown log level `verbose`", "literal YAML (Go's struct cannot write these keys), read by `config.Load`")

	// A config that does not parse.
	c, err = w.enterCase("config/broken")
	if err != nil {
		return err
	}
	if err := writeText(c.path("config", "spela", "config.yaml"), "log_level: info\ncheck_updates: [not, a, bool\n"); err != nil {
		return err
	}
	if err := w.configOracle(c, "YAML that does not parse: config.Load fails with this error"); err != nil {
		return err
	}
	w.note("config/broken/", "A config.yaml that does not parse; the oracle holds Go's error", "literal YAML, read by `config.Load`")

	// A config value of the wrong type.
	c, err = w.enterCase("config/wrong-type")
	if err != nil {
		return err
	}
	if err := writeText(c.path("config", "spela", "config.yaml"), "manifest_refresh_hours: daily\n"); err != nil {
		return err
	}
	if err := w.configOracle(c, "a string where Go wants an int: config.Load fails"); err != nil {
		return err
	}
	w.note("config/wrong-type/", "A config value of the wrong type (`manifest_refresh_hours: daily`)", "literal YAML, read by `config.Load`")
	return nil
}

func (w *fixtureWriter) configOracle(c *fixtureCase, note string) error {
	oracle, err := configOracleFor(c, note)
	if err != nil {
		return err
	}
	return writeJSON(c.path("oracle.json"), oracle)
}
