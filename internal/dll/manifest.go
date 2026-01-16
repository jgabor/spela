package dll

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jgabor/spela/internal/xdg"
)

const (
	DefaultRepositoryURL = "https://raw.githubusercontent.com/jgabor/spela-dlls/main/manifest.json"
	ManifestCacheFile    = "manifest.json"
	ManifestMaxAge       = 24 * time.Hour
)

type Manifest struct {
	Version    string           `json:"version"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Repository string           `json:"repository"`
	DLLs       map[string][]DLL `json:"dlls"`
}

type DLL struct {
	Version     string    `json:"version"`
	Filename    string    `json:"filename"`
	URL         string    `json:"url"`
	SHA256      string    `json:"sha256"`
	Size        int64     `json:"size"`
	ReleaseDate time.Time `json:"release_date"`
	Notes       string    `json:"notes,omitempty"`
}

func LoadManifest() (*Manifest, error) {
	cachePath := xdg.CachePath(ManifestCacheFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func SaveManifest(manifest *Manifest) error {
	cachePath := xdg.CachePath(ManifestCacheFile)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0o644)
}

func FetchManifest(repositoryURL string) (*Manifest, error) {
	if repositoryURL == "" {
		repositoryURL = DefaultRepositoryURL
	}

	resp, err := http.Get(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

func UpdateManifest(repositoryURL string) (*Manifest, error) {
	manifest, err := FetchManifest(repositoryURL)
	if err != nil {
		return nil, err
	}

	if err := SaveManifest(manifest); err != nil {
		return nil, fmt.Errorf("failed to cache manifest: %w", err)
	}

	return manifest, nil
}

func GetManifest(forceUpdate bool, repositoryURL string) (*Manifest, error) {
	if !forceUpdate {
		manifest, err := LoadManifest()
		if err == nil && manifest != nil {
			if time.Since(manifest.UpdatedAt) < ManifestMaxAge {
				return manifest, nil
			}
		}
	}

	return UpdateManifest(repositoryURL)
}

func (m *Manifest) GetLatestDLL(name string) *DLL {
	dlls, ok := m.DLLs[name]
	if !ok || len(dlls) == 0 {
		return nil
	}
	return &dlls[0]
}

func (m *Manifest) GetDLLVersion(name, version string) *DLL {
	dlls, ok := m.DLLs[name]
	if !ok {
		return nil
	}

	for _, dll := range dlls {
		if dll.Version == version {
			return &dll
		}
	}

	return nil
}

func (m *Manifest) ListDLLNames() []string {
	var names []string
	for name := range m.DLLs {
		names = append(names, name)
	}
	return names
}
