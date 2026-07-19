package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/jgabor/spela/internal/nav"
)

// Kind identifies the value representation used by settings presentations.
type Kind int

const (
	KindBool Kind = iota
	KindEnum
	KindPath
	KindInt
	KindStringList
)

// Visibility records the existing public surfaces on which an option is editable.
type Visibility uint8

const (
	VisibilityCLI Visibility = 1 << iota
	VisibilityTUI
	VisibilityGUI
	VisibilityWails
)

// Includes reports whether all requested surfaces are present.
func (visibility Visibility) Includes(requested Visibility) bool {
	return visibility&requested == requested
}

// Option is the canonical description and validated string operation for one Config field.
type Option struct {
	Key         string
	JSONKey     string
	Section     nav.SettingsSection
	Label       string
	Description string
	Kind        Kind
	Choices     []string
	Visibility  Visibility
	get         func(*Config) string
	set         func(*Config, string) error
}

// Get returns the option's canonical string value.
func (option Option) Get(configuration *Config) string { return option.get(configuration) }

// Set parses, validates, and applies a canonical string value.
func (option Option) Set(configuration *Config, value string) error {
	return option.set(configuration, value)
}

// Section groups visible options under one settings navigation section.
type Section struct {
	ID      nav.SettingsSection
	Title   string
	Options []Option
}

var options = []Option{
	enumOption("log_level", "logLevel", nav.SettingsLogging, "Log level", "Control logging verbosity for troubleshooting.", []string{"debug", "info", "warn", "error"}, VisibilityCLI|VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) string { return string(c.LogLevel) }, func(c *Config, value string) { c.LogLevel = LogLevel(value) }, "log level"),
	stringOption("shader_cache", "shaderCache", nav.SettingsPaths, "Shader cache", "Global NVIDIA shader cache path.", KindPath, VisibilityCLI|VisibilityWails, func(c *Config) string { return c.ShaderCache }, func(c *Config, value string) { c.ShaderCache = value }),
	boolOption("check_updates", "checkUpdates", nav.SettingsStartup, "Check for updates", "Look for new Spela releases at startup.", VisibilityCLI|VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) bool { return c.CheckUpdates }, func(c *Config, value bool) { c.CheckUpdates = value }),
	boolOption("show_hints", "showHints", nav.SettingsDisplay, "Show hints", "Show keyboard hints in the footer and dialogs.", VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) bool { return c.ShowHints }, func(c *Config, value bool) { c.ShowHints = value }),
	boolOption("rescan_on_startup", "rescanOnStartup", nav.SettingsStartup, "Re-scan on startup", "Scan for games whenever Spela launches.", VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) bool { return c.RescanOnStartup }, func(c *Config, value bool) { c.RescanOnStartup = value }),
	boolOption("auto_update_dlls", "autoUpdateDLLs", nav.SettingsStartup, "Auto-update DLLs", "Update DLLs automatically when the app starts.", VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) bool { return c.AutoUpdateDLLs }, func(c *Config, value bool) { c.AutoUpdateDLLs = value }),
	stringOption("steam_path", "steamPath", nav.SettingsPaths, "Steam path", "Custom Steam installation path.", KindPath, VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) string { return c.SteamPath }, func(c *Config, value string) { c.SteamPath = value }),
	listOption("additional_library_paths", "additionalLibraryPaths", nav.SettingsPaths, "Additional library paths", "Additional Steam library roots.", VisibilityWails, func(c *Config) []string { return c.AdditionalLibraryPaths }, func(c *Config, value []string) { c.AdditionalLibraryPaths = value }),
	stringOption("dll_cache_path", "dllCachePath", nav.SettingsPaths, "DLL cache path", "Override where downloaded DLLs are stored.", KindPath, VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) string { return c.DLLCachePath }, func(c *Config, value string) { c.DLLCachePath = value }),
	stringOption("backup_path", "backupPath", nav.SettingsPaths, "Backup path", "Location for DLL and save backups.", KindPath, VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) string { return c.BackupPath }, func(c *Config, value string) { c.BackupPath = value }),
	stringOption("dll_manifest_url", "dllManifestURL", nav.SettingsDLLPolicy, "DLL manifest URL", "Override the DLL manifest endpoint.", KindPath, VisibilityWails, func(c *Config) string { return c.DLLManifestURL }, func(c *Config, value string) { c.DLLManifestURL = value }),
	boolOption("auto_refresh_manifest", "autoRefreshManifest", nav.SettingsDLLPolicy, "Auto-refresh manifest", "Refresh the DLL manifest automatically.", VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) bool { return c.AutoRefreshManifest }, func(c *Config, value bool) { c.AutoRefreshManifest = value }),
	intOption("manifest_refresh_hours", "manifestRefreshHours", nav.SettingsDLLPolicy, "Refresh interval", "How often to refresh the manifest (hours).", []string{"1", "6", "12", "24", "48", "168"}, VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) int { return c.ManifestRefreshHours }, func(c *Config, value int) { c.ManifestRefreshHours = value }),
	enumOption("preferred_dll_source", "preferredDLLSource", nav.SettingsDLLPolicy, "DLL source", "Preferred source for DLL downloads.", []string{"techpowerup", "github"}, VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) string { return c.PreferredDLLSource }, func(c *Config, value string) { c.PreferredDLLSource = value }, "DLL source"),
	enumOption("theme", "theme", nav.SettingsDisplay, "Theme", "Use the default, dark, or light theme.", []string{"default", "dark", "light"}, VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) string { return c.Theme }, func(c *Config, value string) { c.Theme = value }, "theme"),
	boolOption("compact_mode", "compactMode", nav.SettingsDisplay, "Compact mode", "Use tighter spacing in lists and panels.", VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) bool { return c.CompactMode }, func(c *Config, value bool) { c.CompactMode = value }),
	boolOption("confirm_destructive", "confirmDestructive", nav.SettingsDisplay, "Confirm destructive", "Ask before destructive actions like restores.", VisibilityTUI|VisibilityGUI|VisibilityWails, func(c *Config) bool { return c.ConfirmDestructive }, func(c *Config, value bool) { c.ConfirmDestructive = value }),
}

// Options returns every Config field descriptor in Config declaration order.
func Options() []Option { return options }

// OptionByKey finds an option by its YAML/CLI key.
func OptionByKey(key string) *Option {
	for index := range options {
		if options[index].Key == key {
			return &options[index]
		}
	}
	return nil
}

// OptionByJSONKey finds an option by its Wails JSON key.
func OptionByJSONKey(key string) *Option {
	for index := range options {
		if options[index].JSONKey == key {
			return &options[index]
		}
	}
	return nil
}

// Sections returns the catalog editable on the requested surface.
func Sections(visibility Visibility) []Section {
	sections := make([]Section, len(nav.SettingsSectionLabels))
	for index, title := range nav.SettingsSectionLabels {
		sections[index] = Section{ID: nav.SettingsSection(index), Title: title}
	}
	for _, option := range options {
		if option.Visibility.Includes(visibility) {
			section := &sections[int(option.Section)]
			section.Options = append(section.Options, option)
		}
	}
	for index := range sections {
		sort.SliceStable(sections[index].Options, func(left, right int) bool {
			return settingsOrder[sections[index].Options[left].Key] < settingsOrder[sections[index].Options[right].Key]
		})
	}
	return sections
}

var settingsOrder = map[string]int{
	"theme": 0, "show_hints": 1, "compact_mode": 2, "confirm_destructive": 3,
	"rescan_on_startup": 0, "auto_update_dlls": 1, "check_updates": 2,
	"steam_path": 0, "dll_cache_path": 1, "backup_path": 2,
	"auto_refresh_manifest": 0, "manifest_refresh_hours": 1, "preferred_dll_source": 2,
	"log_level": 0,
}

// Validate checks all values governed by the option contract without changing configuration.
func Validate(configuration *Config) error {
	validated := configuration.Clone()
	for _, option := range options {
		if err := option.Set(validated, option.Get(configuration)); err != nil {
			return err
		}
	}
	return nil
}

func boolOption(key, jsonKey string, section nav.SettingsSection, label, description string, visibility Visibility, get func(*Config) bool, assign func(*Config, bool)) Option {
	return Option{
		Key: key, JSONKey: jsonKey, Section: section, Label: label, Description: description, Kind: KindBool, Choices: []string{"true", "false"}, Visibility: visibility,
		get: func(c *Config) string { return strconv.FormatBool(get(c)) },
		set: func(c *Config, value string) error {
			switch value {
			case "true", "1":
				assign(c, true)
			case "false", "0":
				assign(c, false)
			default:
				return fmt.Errorf("invalid boolean for %s: %s", key, value)
			}
			return nil
		},
	}
}

func stringOption(key, jsonKey string, section nav.SettingsSection, label, description string, kind Kind, visibility Visibility, get func(*Config) string, assign func(*Config, string)) Option {
	return Option{Key: key, JSONKey: jsonKey, Section: section, Label: label, Description: description, Kind: kind, Visibility: visibility, get: get, set: func(c *Config, value string) error { assign(c, value); return nil }}
}

func enumOption(key, jsonKey string, section nav.SettingsSection, label, description string, choices []string, visibility Visibility, get func(*Config) string, assign func(*Config, string), errorLabel string) Option {
	option := stringOption(key, jsonKey, section, label, description, KindEnum, visibility, get, assign)
	option.Choices = choices
	option.set = func(c *Config, value string) error {
		for _, choice := range choices {
			if value == choice {
				assign(c, value)
				return nil
			}
		}
		return fmt.Errorf("unsupported %s: %s", errorLabel, value)
	}
	return option
}

func intOption(key, jsonKey string, section nav.SettingsSection, label, description string, choices []string, visibility Visibility, get func(*Config) int, assign func(*Config, int)) Option {
	return Option{
		Key: key, JSONKey: jsonKey, Section: section, Label: label, Description: description, Kind: KindInt, Choices: choices, Visibility: visibility,
		get: func(c *Config) string { return strconv.Itoa(get(c)) },
		set: func(c *Config, value string) error {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid integer for %s: %s", key, value)
			}
			assign(c, parsed)
			return nil
		},
	}
}

func listOption(key, jsonKey string, section nav.SettingsSection, label, description string, visibility Visibility, get func(*Config) []string, assign func(*Config, []string)) Option {
	return Option{
		Key: key, JSONKey: jsonKey, Section: section, Label: label, Description: description, Kind: KindStringList, Visibility: visibility,
		get: func(c *Config) string { encoded, _ := json.Marshal(get(c)); return string(encoded) },
		set: func(c *Config, value string) error {
			var parsed []string
			if err := json.Unmarshal([]byte(value), &parsed); err != nil {
				return fmt.Errorf("invalid string list for %s: %w", key, err)
			}
			assign(c, parsed)
			return nil
		},
	}
}
