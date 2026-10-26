package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

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
)

// Includes reports whether all requested surfaces are present.
func (visibility Visibility) Includes(requested Visibility) bool {
	return visibility&requested == requested
}

// Option is the canonical description and validated string operation for one Config field.
type Option struct {
	Key         string
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

// ListSeparator separates entries in the single-line text form of a list option.
const ListSeparator = ":"

// Display returns the value as users edit it. Lists show one entry per
// ListSeparator; every other option shows its canonical value.
func (option Option) Display(configuration *Config) string {
	value := option.get(configuration)
	if option.Kind != KindStringList {
		return value
	}
	var entries []string
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return value
	}
	return strings.Join(entries, ListSeparator)
}

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
	enumOption("log_level", nav.SettingsLogging, "Log level", "Control logging verbosity for troubleshooting.", []string{"debug", "info", "warn", "error"}, VisibilityCLI|VisibilityTUI, func(c *Config) string { return string(c.LogLevel) }, func(c *Config, value string) { c.LogLevel = LogLevel(value) }, "log level"),
	folderOption("shader_cache", "Default shader cache path", "Folder for the NVIDIA shader cache when a profile turns Shader cache on and leaves its path empty. Empty uses Spela's cache folder.", VisibilityCLI|VisibilityTUI, func(c *Config) string { return c.ShaderCache }, func(c *Config, value string) { c.ShaderCache = value }),
	boolOption("check_updates", nav.SettingsStartup, "Check for updates", "At startup, ask GitHub once a day whether a newer Spela release exists and show a notice. Development builds never ask.", VisibilityCLI|VisibilityTUI, func(c *Config) bool { return c.CheckUpdates }, func(c *Config, value bool) { c.CheckUpdates = value }),
	boolOption("rescan_on_startup", nav.SettingsStartup, "Re-scan on startup", "Scan for games when Spela next starts. Requires restart.", VisibilityTUI, func(c *Config) bool { return c.RescanOnStartup }, func(c *Config, value bool) { c.RescanOnStartup = value }),
	boolOption("auto_update_dlls", nav.SettingsStartup, "Auto-update DLLs", "At startup, update DLLs older than the manifest's latest version. Running and deny-listed games and DLLs of unknown version are skipped.", VisibilityTUI, func(c *Config) bool { return c.AutoUpdateDLLs }, func(c *Config, value bool) { c.AutoUpdateDLLs = value }),
	stringOption("steam_path", nav.SettingsPaths, "Steam path", "Use this Steam installation for future manual scans and when Spela next starts.", KindPath, VisibilityTUI, func(c *Config) string { return c.SteamPath }, func(c *Config, value string) { c.SteamPath = value }),
	listOption("additional_library_paths", nav.SettingsPaths, "Additional library paths", "Additional Steam library folders, separated by colons.", VisibilityTUI, func(c *Config) []string { return c.AdditionalLibraryPaths }, func(c *Config, value []string) { c.AdditionalLibraryPaths = value }),
	folderOption("dll_cache_path", "DLL cache path", "Folder for downloaded DLLs. Empty uses Spela's cache folder. Earlier downloads are not moved.", VisibilityTUI, func(c *Config) string { return c.DLLCachePath }, func(c *Config, value string) { c.DLLCachePath = value }),
	folderOption("backup_path", "Backup path", "Folder for new DLL backup files. Empty uses Spela's data folder. Earlier backups stay restorable.", VisibilityTUI, func(c *Config) string { return c.BackupPath }, func(c *Config, value string) { c.BackupPath = value }),
	manifestURLOption(),
	boolOption("auto_refresh_manifest", nav.SettingsDLLPolicy, "Auto-refresh manifest", "Fetch a new DLL manifest when the cached one is older than the refresh interval. Off keeps the cached manifest.", VisibilityTUI, func(c *Config) bool { return c.AutoRefreshManifest }, func(c *Config, value bool) { c.AutoRefreshManifest = value }),
	intOption("manifest_refresh_hours", nav.SettingsDLLPolicy, "Refresh interval", "Hours before the cached DLL manifest counts as old.", []string{"1", "6", "12", "24", "48", "168"}, VisibilityTUI, func(c *Config) int { return c.ManifestRefreshHours }, func(c *Config, value int) { c.ManifestRefreshHours = value }),
	// theme was edited only in the removed GUI. It stays so Validate still checks stored values.
	enumOption("theme", nav.SettingsDisplay, "Theme", "Use the default, dark, or light theme.", []string{"default", "dark", "light"}, 0, func(c *Config) string { return c.Theme }, func(c *Config, value string) { c.Theme = value }, "theme"),
	boolOption("compact_mode", nav.SettingsDisplay, "Compact mode", "Use tighter spacing. The TUI starts in the Compact layout; Actions still switch layout for the session.", VisibilityTUI, func(c *Config) bool { return c.CompactMode }, func(c *Config, value bool) { c.CompactMode = value }),
	boolOption("confirm_destructive", nav.SettingsDisplay, "Confirm destructive actions", "Ask before updating DLLs. Restoring and installing DLLs always ask.", VisibilityTUI, func(c *Config) bool { return c.ConfirmDestructive }, func(c *Config, value bool) { c.ConfirmDestructive = value }),
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
	"theme": 0, "compact_mode": 2, "confirm_destructive": 3,
	"rescan_on_startup": 0, "auto_update_dlls": 1, "check_updates": 2,
	"steam_path": 0, "additional_library_paths": 1, "dll_cache_path": 2, "backup_path": 3, "shader_cache": 4,
	"auto_refresh_manifest": 0, "manifest_refresh_hours": 1, "dll_manifest_url": 2,
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

func boolOption(key string, section nav.SettingsSection, label, description string, visibility Visibility, get func(*Config) bool, assign func(*Config, bool)) Option {
	return Option{
		Key: key, Section: section, Label: label, Description: description, Kind: KindBool, Choices: []string{"true", "false"}, Visibility: visibility,
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

func stringOption(key string, section nav.SettingsSection, label, description string, kind Kind, visibility Visibility, get func(*Config) string, assign func(*Config, string)) Option {
	return Option{Key: key, Section: section, Label: label, Description: description, Kind: kind, Visibility: visibility, get: get, set: func(c *Config, value string) error { assign(c, value); return nil }}
}

// folderOption is a Paths option that holds an absolute folder or nothing.
func folderOption(key, label, description string, visibility Visibility, get func(*Config) string, assign func(*Config, string)) Option {
	option := stringOption(key, nav.SettingsPaths, label, description, KindPath, visibility, get, assign)
	option.set = func(c *Config, value string) error {
		if err := validateFolder(key, value); err != nil {
			return err
		}
		assign(c, value)
		return nil
	}
	return option
}

// manifestURLOption holds the DLL manifest source: an https URL or nothing.
func manifestURLOption() Option {
	option := stringOption("dll_manifest_url", nav.SettingsDLLPolicy, "DLL manifest URL", "Where Spela gets the list of DLLs and checksums it installs. Empty uses Spela's own manifest. A new URL is fetched at next use.", KindPath, VisibilityTUI, func(c *Config) string { return c.DLLManifestURL }, func(c *Config, value string) { c.DLLManifestURL = value })
	option.set = func(c *Config, value string) error {
		if value != "" {
			if err := ValidateManifestURL(value); err != nil {
				return err
			}
		}
		c.DLLManifestURL = value
		return nil
	}
	return option
}

func enumOption(key string, section nav.SettingsSection, label, description string, choices []string, visibility Visibility, get func(*Config) string, assign func(*Config, string), errorLabel string) Option {
	option := stringOption(key, section, label, description, KindEnum, visibility, get, assign)
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

func intOption(key string, section nav.SettingsSection, label, description string, choices []string, visibility Visibility, get func(*Config) int, assign func(*Config, int)) Option {
	return Option{
		Key: key, Section: section, Label: label, Description: description, Kind: KindInt, Choices: choices, Visibility: visibility,
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

func listOption(key string, section nav.SettingsSection, label, description string, visibility Visibility, get func(*Config) []string, assign func(*Config, []string)) Option {
	return Option{
		Key: key, Section: section, Label: label, Description: description, Kind: KindStringList, Visibility: visibility,
		get: func(c *Config) string { encoded, _ := json.Marshal(get(c)); return string(encoded) },
		set: func(c *Config, value string) error {
			parsed := []string{}
			if strings.HasPrefix(strings.TrimSpace(value), "[") {
				if err := json.Unmarshal([]byte(value), &parsed); err != nil {
					return fmt.Errorf("invalid string list for %s: %w", key, err)
				}
			} else {
				for _, entry := range strings.Split(value, ListSeparator) {
					if entry = strings.TrimSpace(entry); entry != "" {
						parsed = append(parsed, entry)
					}
				}
			}
			assign(c, parsed)
			return nil
		},
	}
}
