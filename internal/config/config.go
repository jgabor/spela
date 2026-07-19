package config

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/xdg"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type Config struct {
	LogLevel     LogLevel `yaml:"log_level" json:"logLevel"`
	ShaderCache  string   `yaml:"shader_cache" json:"shaderCache"`
	CheckUpdates bool     `yaml:"check_updates" json:"checkUpdates"`
	ShowHints    bool     `yaml:"show_hints" json:"showHints"`

	// Startup behavior
	RescanOnStartup bool `yaml:"rescan_on_startup" json:"rescanOnStartup"`
	AutoUpdateDLLs  bool `yaml:"auto_update_dlls" json:"autoUpdateDLLs"`

	// Paths
	SteamPath              string   `yaml:"steam_path,omitempty" json:"steamPath"`
	AdditionalLibraryPaths []string `yaml:"additional_library_paths,omitempty" json:"additionalLibraryPaths"`
	DLLCachePath           string   `yaml:"dll_cache_path,omitempty" json:"dllCachePath"`
	BackupPath             string   `yaml:"backup_path,omitempty" json:"backupPath"`

	// DLL management
	DLLManifestURL       string `yaml:"dll_manifest_url,omitempty" json:"dllManifestURL"`
	AutoRefreshManifest  bool   `yaml:"auto_refresh_manifest" json:"autoRefreshManifest"`
	ManifestRefreshHours int    `yaml:"manifest_refresh_hours" json:"manifestRefreshHours"`
	PreferredDLLSource   string `yaml:"preferred_dll_source,omitempty" json:"preferredDLLSource"`

	// Display
	Theme              string `yaml:"theme,omitempty" json:"theme"`
	CompactMode        bool   `yaml:"compact_mode" json:"compactMode"`
	ConfirmDestructive bool   `yaml:"confirm_destructive" json:"confirmDestructive"`
}

func Default() *Config {
	return &Config{
		LogLevel:     LogLevelInfo,
		ShaderCache:  xdg.CachePath("nvidia"),
		CheckUpdates: true,
		ShowHints:    true,

		RescanOnStartup: true,
		AutoUpdateDLLs:  false,

		AutoRefreshManifest:  true,
		ManifestRefreshHours: 24,
		PreferredDLLSource:   "techpowerup",

		Theme:              "default",
		CompactMode:        false,
		ConfirmDestructive: true,
	}
}

func Load() (*Config, error) {
	path := xdg.ConfigPath("config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default(), nil
		}
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Save() error {
	if _, err := xdg.EnsureConfigHome(); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	path := xdg.ConfigPath("config.yaml")
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) Clone() *Config {
	clone := *c
	if c.AdditionalLibraryPaths != nil {
		clone.AdditionalLibraryPaths = make([]string, len(c.AdditionalLibraryPaths))
		copy(clone.AdditionalLibraryPaths, c.AdditionalLibraryPaths)
	}
	return &clone
}
