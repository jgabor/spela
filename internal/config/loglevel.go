package config

import "github.com/jgabor/spela/internal/logging"

// ApplyLogLevel applies the configured log level to the shared logger. An
// unknown level is reported and the current level stays.
func (c *Config) ApplyLogLevel() {
	if err := logging.SetLevel(string(c.LogLevel)); err != nil {
		logging.Warn("ignoring configured log level", "error", err)
	}
}

// ApplyLogLevel loads the configuration and applies its log level. A missing
// or unreadable configuration keeps the default level, because the commands
// that depend on the configuration already report that failure themselves.
func ApplyLogLevel() {
	configuration, err := Load()
	if err != nil {
		return
	}
	configuration.ApplyLogLevel()
}
