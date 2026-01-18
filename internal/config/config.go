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
	LogLevel      LogLevel `yaml:"log_level"`
	DefaultPreset string   `yaml:"default_preset"`
	ShaderCache   string   `yaml:"shader_cache"`
	CheckUpdates  bool     `yaml:"check_updates"`
	ShowHints     bool     `yaml:"show_hints"`

	// Startup behavior
	RescanOnStartup bool `yaml:"rescan_on_startup"`
	AutoUpdateDLLs  bool `yaml:"auto_update_dlls"`

	// Paths
	SteamPath              string   `yaml:"steam_path,omitempty"`
	AdditionalLibraryPaths []string `yaml:"additional_library_paths,omitempty"`
	DLLCachePath           string   `yaml:"dll_cache_path,omitempty"`
	BackupPath             string   `yaml:"backup_path,omitempty"`

	// DLL management
	DLLManifestURL       string `yaml:"dll_manifest_url,omitempty"`
	AutoRefreshManifest  bool   `yaml:"auto_refresh_manifest"`
	ManifestRefreshHours int    `yaml:"manifest_refresh_hours"`
	PreferredDLLSource   string `yaml:"preferred_dll_source,omitempty"`

	// Display
	Theme              string `yaml:"theme,omitempty"`
	CompactMode        bool   `yaml:"compact_mode"`
	ConfirmDestructive bool   `yaml:"confirm_destructive"`
}

func Default() *Config {
	return &Config{
		LogLevel:      LogLevelInfo,
		DefaultPreset: "balanced",
		ShaderCache:   xdg.CachePath("nvidia"),
		CheckUpdates:  true,
		ShowHints:     true,

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
