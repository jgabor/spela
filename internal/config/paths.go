package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"

	"github.com/jgabor/spela/internal/xdg"
)

// DLLCacheDir returns the folder that holds downloaded DLLs: the configured
// dll_cache_path, or the XDG cache folder when it is empty or unusable.
func DLLCacheDir() string {
	return configuredPath(func(c *Config) string { return c.DLLCachePath }, xdg.CachePath("dlls"))
}

// BackupFilesDir returns the folder that receives new DLL backup files: the
// configured backup_path, or the XDG data folder when it is empty or unusable.
// Backup metadata always stays in the XDG data folder and records absolute
// file paths, so changing this folder never hides earlier backups.
func BackupFilesDir() string {
	return configuredPath(func(c *Config) string { return c.BackupPath }, DefaultBackupDir())
}

// DefaultBackupDir returns the XDG data folder that holds backup metadata.
func DefaultBackupDir() string {
	return xdg.DataPath("backups")
}

// ShaderCacheDir returns the shader cache folder used when a profile turns the
// shader cache on without its own path: the configured shader_cache, or the
// XDG cache folder when it is empty or unusable.
func ShaderCacheDir() string {
	return configuredPath(func(c *Config) string { return c.ShaderCache }, xdg.CachePath("nvidia"))
}

// configuredPath loads the configuration on every call so a saved change
// applies to the next operation without a restart. An unreadable
// configuration or a relative path falls back to the default.
func configuredPath(get func(*Config) string, fallback string) string {
	configuration, err := Load()
	if err != nil {
		return fallback
	}
	if path := get(configuration); path != "" && filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return fallback
}

func validateFolder(key, value string) error {
	if value != "" && !filepath.IsAbs(value) {
		return fmt.Errorf("%s must be an absolute path: %s", key, value)
	}
	return nil
}

// ValidateManifestURL accepts an https URL, or an http URL to this machine
// for a local manifest server. The manifest decides which DLLs and checksums
// Spela installs, so it never travels over plain http to another host.
func ValidateManifestURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return errors.New("dll_manifest_url must be an https URL")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if host := parsed.Hostname(); host == "localhost" || net.ParseIP(host).IsLoopback() {
			return nil
		}
	}
	return errors.New("dll_manifest_url must be an https URL")
}
