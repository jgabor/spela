// Package release knows the running Spela version and checks, at most once a
// day, whether a newer release is tagged on GitHub.
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/xdg"
)

// Version is the running Spela version. The spela command sets it from its
// build-time version before any interface starts.
var Version = "dev"

// TagsURL lists the repository tags; each release is a vX.Y.Z tag. GitHub
// releases lag the tags, so the tags are the release record.
var TagsURL = "https://api.github.com/repos/jgabor/spela/tags?per_page=100"

// CheckInterval is the time between two lookups, successful or not.
const CheckInterval = 24 * time.Hour

const (
	stateFile    = "update-check.json"
	checkTimeout = 5 * time.Second
	maxBodyBytes = 1 << 20
)

var httpClient = &http.Client{Timeout: checkTimeout}

var releaseVersion = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

// IsRelease reports whether version is a plain release (1.2.3 or v1.2.3).
// Development builds, git describe output, and AUR -git versions are not.
func IsRelease(version string) bool { return releaseVersion.MatchString(version) }

// Notice describes a newer release than the running one.
type Notice struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

// Text is the one-line notice shown by the TUI and GUI.
func (n Notice) Text() string {
	return fmt.Sprintf("Spela %s is available (you have %s)", withPrefix(n.Latest), withPrefix(n.Current))
}

type checkState struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
}

// Check returns a notice when a newer release than current is known. It asks
// GitHub only when the last attempt is older than CheckInterval, records every
// attempt so an offline machine does not wait at each start, and reuses the
// last known release otherwise. A development build is never checked.
func Check(ctx context.Context, current string, now time.Time) (*Notice, error) {
	if !IsRelease(current) {
		return nil, nil
	}
	state, _ := loadState()
	var lookupErr error
	if state.CheckedAt.IsZero() || now.Sub(state.CheckedAt) >= CheckInterval || now.Before(state.CheckedAt) {
		latest, err := latestTag(ctx)
		lookupErr = err
		state.CheckedAt = now
		if err == nil {
			state.Latest = latest
		}
		if err := saveState(state); err != nil && lookupErr == nil {
			lookupErr = err
		}
	}
	if IsRelease(state.Latest) && dll.IsNewer(current, state.Latest) {
		return &Notice{Current: current, Latest: state.Latest}, lookupErr
	}
	return nil, lookupErr
}

func latestTag(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, TagsURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("look up releases: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("look up releases: HTTP %d", response.StatusCode)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxBodyBytes)).Decode(&tags); err != nil {
		return "", fmt.Errorf("read releases: %w", err)
	}
	latest := ""
	for _, tag := range tags {
		if IsRelease(tag.Name) && (latest == "" || dll.IsNewer(latest, tag.Name)) {
			latest = tag.Name
		}
	}
	if latest == "" {
		return "", errors.New("read releases: no release tags")
	}
	return latest, nil
}

func statePath() string { return xdg.CachePath(stateFile) }

func loadState() (checkState, error) {
	var state checkState
	data, err := os.ReadFile(statePath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return state, nil
		}
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return checkState{}, err
	}
	return state, nil
}

func saveState(state checkState) error {
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+stateFile+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary.Name()) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}

func withPrefix(version string) string {
	if version != "" && version[0] != 'v' {
		return "v" + version
	}
	return version
}
