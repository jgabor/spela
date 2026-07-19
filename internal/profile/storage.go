package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/xdg"
)

const defaultProfileFilename = "default.yaml"

func profilesDir() string {
	return xdg.ConfigPath("profiles")
}

func defaultProfilePath() string {
	return filepath.Join(profilesDir(), defaultProfileFilename)
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

	p, err := unmarshalProfileYAML(data)
	if err != nil {
		return nil, err
	}

	// Legacy profiles (saved before per-field inheritance existed) have no
	// Overrides map. Reconstruct it by comparing each field to the current
	// default: equal values collapse to inherited, differing values become
	// explicit overrides.
	if p.Overrides == nil {
		defaults, err := LoadDefault()
		if err != nil {
			return nil, fmt.Errorf("load default profile for migration: %w", err)
		}
		migrateInheritance(p, defaults)
	}

	return p, nil
}

func LoadDefault() (*Profile, error) {
	data, err := os.ReadFile(defaultProfilePath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	return unmarshalProfileYAML(data)
}

func Save(appID uint64, p *Profile) error {
	return save(profilePath(appID), p)
}

func SaveDefault(p *Profile) error {
	return save(defaultProfilePath(), p)
}

func save(path string, p *Profile) error {
	if err := EnsureProfilesDir(); err != nil {
		return err
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".profile-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

// Mutate serializes a fresh read-modify-write transaction across processes.
// The callback must not call Mutate or MutateDefault. A missing profile is
// supplied as an empty profile; defaults are loaded fresh for pin/reset logic.
func Mutate(appID uint64, callback func(current, defaults *Profile) error) error {
	_, err := mutate(appID, true, callback)
	return err
}

// MutateExisting is Mutate without implicit profile creation.
func MutateExisting(appID uint64, callback func(current, defaults *Profile) error) (bool, error) {
	return mutate(appID, false, callback)
}

func mutate(appID uint64, create bool, callback func(current, defaults *Profile) error) (bool, error) {
	exists := false
	err := withMutationLock(func() error {
		current, err := Load(appID)
		if err != nil {
			return err
		}
		if current == nil && !create {
			return nil
		}
		exists = current != nil
		defaults, err := LoadDefault()
		if err != nil {
			return fmt.Errorf("load default profile: %w", err)
		}
		current = current.Clone()
		if err := callback(current, defaults); err != nil {
			return err
		}
		return Save(appID, current)
	})
	return exists, err
}

// MutateDefault is the default-profile form of Mutate. Its callback must not recurse.
func MutateDefault(callback func(current *Profile) error) error {
	return withMutationLock(func() error {
		current, err := LoadDefault()
		if err != nil {
			return err
		}
		current = current.Clone()
		if err := callback(current); err != nil {
			return err
		}
		return SaveDefault(current)
	})
}

func withMutationLock(callback func() error) error {
	if err := EnsureProfilesDir(); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(profilesDir(), ".mutation.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	return callback()
}

func Delete(appID uint64) error {
	return withMutationLock(func() error {
		err := os.Remove(profilePath(appID))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	})
}

// Create writes a profile only when no profile exists in the locked transaction.
func Create(appID uint64, p *Profile) (bool, error) {
	created := false
	err := withMutationLock(func() error {
		if Exists(appID) {
			return nil
		}
		created = true
		return Save(appID, p)
	})
	return created, err
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
