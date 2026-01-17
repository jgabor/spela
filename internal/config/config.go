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
}

func Default() *Config {
	return &Config{
		LogLevel:      LogLevelInfo,
		DefaultPreset: "balanced",
		ShaderCache:   xdg.CachePath("nvidia"),
		CheckUpdates:  true,
		ShowHints:     true,
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
