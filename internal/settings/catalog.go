// Package settings defines the canonical settings section and option catalog
// shared by TUI and GUI. Section order matches nav.SettingsSection.
package settings

import "github.com/jgabor/spela/internal/nav"

// Kind identifies how an option is edited in the UI.
type Kind int

const (
	KindBool Kind = iota
	KindEnum
	KindPath
	KindInt
)

// Option describes one editable config field.
type Option struct {
	Key         string // snake_case config key (TUI)
	JSONKey     string // camelCase key (GUI / Wails)
	Label       string
	Description string
	Kind        Kind
	Choices     []string
}

// Section groups options under one nav.SettingsSection.
type Section struct {
	ID      nav.SettingsSection
	Title   string
	Options []Option
}

// Catalog returns settings sections in nav.SettingsSection order.
func Catalog() []Section {
	if cachedCatalog == nil {
		cachedCatalog = buildCatalog()
	}
	return cachedCatalog
}

var cachedCatalog []Section

func buildCatalog() []Section {
	return []Section{
		{
			ID:    nav.SettingsDisplay,
			Title: nav.SettingsSectionLabels[nav.SettingsDisplay],
			Options: []Option{
				{Key: "theme", JSONKey: "theme", Label: "Theme", Description: "Match the system theme or force dark mode.", Kind: KindEnum, Choices: []string{"default", "dark"}},
				{Key: "show_hints", JSONKey: "showHints", Label: "Show hints", Description: "Show keyboard hints in the footer and dialogs.", Kind: KindBool, Choices: []string{"true", "false"}},
				{Key: "compact_mode", JSONKey: "compactMode", Label: "Compact mode", Description: "Use tighter spacing in lists and panels.", Kind: KindBool, Choices: []string{"true", "false"}},
				{Key: "confirm_destructive", JSONKey: "confirmDestructive", Label: "Confirm destructive", Description: "Ask before destructive actions like restores.", Kind: KindBool, Choices: []string{"true", "false"}},
			},
		},
		{
			ID:    nav.SettingsStartup,
			Title: nav.SettingsSectionLabels[nav.SettingsStartup],
			Options: []Option{
				{Key: "rescan_on_startup", JSONKey: "rescanOnStartup", Label: "Re-scan on startup", Description: "Scan for games whenever Spela launches.", Kind: KindBool, Choices: []string{"true", "false"}},
				{Key: "auto_update_dlls", JSONKey: "autoUpdateDLLs", Label: "Auto-update DLLs", Description: "Update DLLs automatically when the app starts.", Kind: KindBool, Choices: []string{"true", "false"}},
				{Key: "check_updates", JSONKey: "checkUpdates", Label: "Check for updates", Description: "Look for new Spela releases at startup.", Kind: KindBool, Choices: []string{"true", "false"}},
			},
		},
		{
			ID:    nav.SettingsPaths,
			Title: nav.SettingsSectionLabels[nav.SettingsPaths],
			Options: []Option{
				{Key: "steam_path", JSONKey: "steamPath", Label: "Steam path", Description: "Custom Steam installation path.", Kind: KindPath},
				{Key: "dll_cache_path", JSONKey: "dllCachePath", Label: "DLL cache path", Description: "Override where downloaded DLLs are stored.", Kind: KindPath},
				{Key: "backup_path", JSONKey: "backupPath", Label: "Backup path", Description: "Location for DLL and save backups.", Kind: KindPath},
			},
		},
		{
			ID:    nav.SettingsDLLPolicy,
			Title: nav.SettingsSectionLabels[nav.SettingsDLLPolicy],
			Options: []Option{
				{Key: "auto_refresh_manifest", JSONKey: "autoRefreshManifest", Label: "Auto-refresh manifest", Description: "Refresh the DLL manifest automatically.", Kind: KindBool, Choices: []string{"true", "false"}},
				{Key: "manifest_refresh_hours", JSONKey: "manifestRefreshHours", Label: "Refresh interval", Description: "How often to refresh the manifest (hours).", Kind: KindInt, Choices: []string{"1", "6", "12", "24", "48", "168"}},
				{Key: "preferred_dll_source", JSONKey: "preferredDLLSource", Label: "DLL source", Description: "Preferred source for DLL downloads.", Kind: KindEnum, Choices: []string{"techpowerup", "github"}},
			},
		},
		{
			ID:    nav.SettingsLogging,
			Title: nav.SettingsSectionLabels[nav.SettingsLogging],
			Options: []Option{
				{Key: "log_level", JSONKey: "logLevel", Label: "Log level", Description: "Control logging verbosity for troubleshooting.", Kind: KindEnum, Choices: []string{"debug", "info", "warn", "error"}},
			},
		},
	}
}

// SectionByID returns the catalog section for id, or nil when out of range.
func SectionByID(id nav.SettingsSection) *Section {
	catalog := Catalog()
	for i := range catalog {
		if catalog[i].ID == id {
			return &catalog[i]
		}
	}
	return nil
}
