package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/xdg"
)

func profilesDir() string {
	return xdg.ConfigPath("profiles")
}

func profilePath(appID uint64) string {
	return filepath.Join(profilesDir(), strconv.FormatUint(appID, 10)+".yaml")
}

func EnsureProfilesDir() error {
	return os.MkdirAll(profilesDir(), 0o755)
}

func Load(appID uint64) (*Profile, error) {
	path := profilePath(appID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

func Save(appID uint64, p *Profile) error {
	if err := EnsureProfilesDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}

	return os.WriteFile(profilePath(appID), data, 0o644)
}

func Delete(appID uint64) error {
	path := profilePath(appID)
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func List() (map[uint64]*Profile, error) {
	dir := profilesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return make(map[uint64]*Profile), nil
		}
		return nil, err
	}

	profiles := make(map[uint64]*Profile)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		idStr := name[:len(name)-len(ext)]
		appID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}

		p, err := Load(appID)
		if err != nil || p == nil {
			continue
		}

		profiles[appID] = p
	}

	return profiles, nil
}

func Exists(appID uint64) bool {
	_, err := os.Stat(profilePath(appID))
	return err == nil
}
