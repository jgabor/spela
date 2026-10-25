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
	"slices"
	"time"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/xdg"
)

// httpClient is used for all HTTP requests in the dll package.
// The client timeout bounds both response headers and body transfer so a
// stalled download cannot hold a DLL mutation indefinitely.
var httpClient = &http.Client{
	Transport: &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
	},
	Timeout: 2 * time.Minute,
}

const (
	DefaultManifestURL = "https://raw.githubusercontent.com/jgabor/spela/main/data/manifest.json"
	ManifestCacheFile  = "manifest.json"
	ManifestMaxAge     = 24 * time.Hour
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

// cachedManifest is the cache file: the manifest plus when and where it was
// fetched. The extra fields sit beside the manifest fields, so the file still
// reads as a plain manifest.
type cachedManifest struct {
	Manifest
	FetchedAt time.Time `json:"fetched_at,omitzero"`
	SourceURL string    `json:"source_url,omitempty"`
}

func LoadManifest() (*Manifest, error) {
	cached, err := loadCachedManifest()
	if err != nil || cached == nil {
		return nil, err
	}
	return &cached.Manifest, nil
}

func loadCachedManifest() (*cachedManifest, error) {
	cachePath := xdg.CachePath(ManifestCacheFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var cached cachedManifest
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}
	// A cache written before fetch times were recorded was written when it
	// was fetched.
	if cached.FetchedAt.IsZero() {
		if info, err := os.Stat(cachePath); err == nil {
			cached.FetchedAt = info.ModTime()
		}
	}

	return &cached, nil
}

// SaveManifest caches a manifest from the default source as fetched now.
func SaveManifest(manifest *Manifest) error {
	return saveCachedManifest(&cachedManifest{Manifest: *manifest, FetchedAt: time.Now(), SourceURL: DefaultManifestURL})
}

func saveCachedManifest(cached *cachedManifest) error {
	cachePath := xdg.CachePath(ManifestCacheFile)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomically(cachePath, data, 0o644)
}

func FetchManifest(manifestURL string) (*Manifest, error) {
	if manifestURL == "" {
		manifestURL = DefaultManifestURL
	}

	resp, err := httpClient.Get(manifestURL)
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

func UpdateManifest(manifestURL string) (*Manifest, error) {
	if manifestURL == "" {
		manifestURL = DefaultManifestURL
	}
	manifest, err := FetchManifest(manifestURL)
	if err != nil {
		return nil, err
	}

	if err := saveCachedManifest(&cachedManifest{Manifest: *manifest, FetchedAt: time.Now(), SourceURL: manifestURL}); err != nil {
		return nil, fmt.Errorf("failed to cache manifest: %w", err)
	}

	return manifest, nil
}

// manifestPolicy is the manifest source and refresh rule from Settings.
type manifestPolicy struct {
	url         string
	autoRefresh bool
	maxAge      time.Duration
}

// currentManifestPolicy reads Settings on every call so a saved change applies
// to the next manifest use. A non-empty manifestURL overrides the configured
// source. An unreadable configuration, an unusable URL, or a refresh interval
// below one hour falls back to the default.
func currentManifestPolicy(manifestURL string) manifestPolicy {
	policy := manifestPolicy{url: DefaultManifestURL, autoRefresh: true, maxAge: ManifestMaxAge}
	if configuration, err := config.Load(); err == nil {
		policy.autoRefresh = configuration.AutoRefreshManifest
		if configuration.ManifestRefreshHours > 0 {
			policy.maxAge = time.Duration(configuration.ManifestRefreshHours) * time.Hour
		}
		// No warning here: the TUI owns the terminal, and the editors
		// already reject an unusable URL.
		if configured := configuration.DLLManifestURL; configured != "" && config.ValidateManifestURL(configured) == nil {
			policy.url = configured
		}
	}
	if manifestURL != "" {
		policy.url = manifestURL
	}
	return policy
}

// GetManifest returns the cached manifest or fetches a new one. It fetches
// when forced, when nothing is cached, when the manifest source changed, or
// when automatic refresh is on and the cache is older than the refresh
// interval. A failed fetch falls back to the cache.
func GetManifest(forceUpdate bool, manifestURL string) (*Manifest, error) {
	policy := currentManifestPolicy(manifestURL)
	var cached *cachedManifest
	if !forceUpdate {
		var err error
		cached, err = loadCachedManifest()
		if err != nil {
			return nil, err
		}
		if cached != nil && cachedManifestCurrent(cached, policy) {
			return &cached.Manifest, nil
		}
	}

	manifest, err := UpdateManifest(policy.url)
	if err != nil && cached != nil {
		return &cached.Manifest, nil
	}
	return manifest, err
}

func cachedManifestCurrent(cached *cachedManifest, policy manifestPolicy) bool {
	// A cache written before sources were recorded came from the default.
	source := cached.SourceURL
	if source == "" {
		source = DefaultManifestURL
	}
	if source != policy.url {
		return false
	}
	if !policy.autoRefresh {
		return true
	}
	return !cached.FetchedAt.IsZero() && time.Since(cached.FetchedAt) < policy.maxAge
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
	names := make([]string, 0, len(m.DLLs))
	for name := range m.DLLs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
