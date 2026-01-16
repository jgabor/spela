package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jgabor/spela/internal/xdg"
)

const (
	GitHubReleasesURL = "https://api.github.com/repos/jgabor/spela/releases/latest"
	CheckInterval     = 24 * time.Hour
)

type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type UpdateInfo struct {
	Available    bool
	Current      string
	Latest       string
	ReleaseURL   string
	ReleaseNotes string
}

func CheckForUpdate(currentVersion string) (*UpdateInfo, error) {
	resp, err := http.Get(GitHubReleasesURL)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to check for updates: HTTP %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentClean := strings.TrimPrefix(currentVersion, "v")

	return &UpdateInfo{
		Available:    latestVersion != currentClean && currentClean != "dev",
		Current:      currentVersion,
		Latest:       release.TagName,
		ReleaseURL:   release.HTMLURL,
		ReleaseNotes: release.Body,
	}, nil
}

type UpdateState struct {
	LastCheck time.Time `json:"last_check"`
	Dismissed string    `json:"dismissed,omitempty"`
}

func LoadUpdateState() (*UpdateState, error) {
	path := xdg.DataPath("update_state.json")
	data, err := xdg.ReadFile(path)
	if err != nil {
		return &UpdateState{}, nil
	}

	var state UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return &UpdateState{}, nil
	}

	return &state, nil
}

func SaveUpdateState(state *UpdateState) error {
	path := xdg.DataPath("update_state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return xdg.WriteFile(path, data)
}

func ShouldCheckForUpdate() bool {
	state, _ := LoadUpdateState()
	return time.Since(state.LastCheck) > CheckInterval
}

func CheckAndNotify(currentVersion string) (*UpdateInfo, error) {
	if !ShouldCheckForUpdate() {
		return nil, nil
	}

	info, err := CheckForUpdate(currentVersion)
	if err != nil {
		return nil, err
	}

	state := &UpdateState{
		LastCheck: time.Now(),
	}
	_ = SaveUpdateState(state)

	if !info.Available {
		return nil, nil
	}

	return info, nil
}
