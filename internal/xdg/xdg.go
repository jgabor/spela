package xdg

import (
	"os"
	"path/filepath"
)

const appName = "spela"

func ConfigHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, appName)
	}
	return filepath.Join(os.Getenv("HOME"), ".config", appName)
}

func DataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, appName)
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", appName)
}

func CacheHome() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, appName)
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", appName)
}

func RuntimeDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, appName)
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}
	return filepath.Join("/tmp", "runtime-"+user, appName)
}

func EnsureConfigHome() (string, error) {
	dir := ConfigHome()
	return dir, os.MkdirAll(dir, 0o755)
}

func EnsureDataHome() (string, error) {
	dir := DataHome()
	return dir, os.MkdirAll(dir, 0o755)
}

func EnsureCacheHome() (string, error) {
	dir := CacheHome()
	return dir, os.MkdirAll(dir, 0o755)
}

func ConfigPath(elem ...string) string {
	return filepath.Join(append([]string{ConfigHome()}, elem...)...)
}

func DataPath(elem ...string) string {
	return filepath.Join(append([]string{DataHome()}, elem...)...)
}

func CachePath(elem ...string) string {
	return filepath.Join(append([]string{CacheHome()}, elem...)...)
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
