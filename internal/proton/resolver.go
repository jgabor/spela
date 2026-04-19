package proton

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jgabor/spela/internal/steam"
)

// ErrProtonNotResolved is the sentinel returned when a Proton build
// cannot be identified for an AppID. Distinct from "resolved but
// unsupported" — callers should treat this as "feature detection
// didn't run", not "feature is off".
var ErrProtonNotResolved = errors.New("proton build not resolved")

// Build represents a resolved Proton installation on disk.
//
// Name is the Steam-level identifier for the tool (e.g. "GE-Proton10-34",
// "proton_experimental", "cachyos-10.0-20260330-slr"). It is always
// non-empty when ResolveForAppID returns nil error.
//
// Path is the absolute path to the build directory that contains the
// top-level `proton` launch script Steam invokes.
type Build struct {
	Name string
	Path string
}

// ResolveForAppID walks Steam's compat-tool resolution chain for the
// given AppID and returns the Proton build Steam will launch the game
// with. Resolution order:
//
//  1. Per-game override in config/config.vdf under CompatToolMapping[<appid>]
//  2. Global default under CompatToolMapping["0"]
//  3. No mapping → ErrProtonNotResolved
//
// Once a name is resolved, the build directory is located under:
//
//   - <steam>/compatibilitytools.d/<name>   (user-installed / community)
//   - <steam>/steamapps/common/<name>       (built-in Proton, name match)
//
// The steamRoot argument is the Steam install root (the directory that
// contains config/ and steamapps/). Pass steam.FindSteamPath() in
// production; tests construct a fake root under t.TempDir().
func ResolveForAppID(steamRoot string, appID uint64) (Build, error) {
	if steamRoot == "" {
		return Build{}, fmt.Errorf("%w: empty steam root", ErrProtonNotResolved)
	}

	name, err := lookupCompatTool(steamRoot, appID)
	if err != nil {
		return Build{}, err
	}

	path, err := locateBuildDir(steamRoot, name)
	if err != nil {
		return Build{}, err
	}

	return Build{Name: name, Path: path}, nil
}

// SupportsVKD3DHeap reports whether the given Proton build ships the
// PROTON_VKD3D_HEAP code path. Detection is a literal grep of the
// top-level `proton` launch script for the string "PROTON_VKD3D_HEAP".
//
// Returns false (no error) when the script is absent: the directory
// exists but isn't recognizably a Proton build, which for our purposes
// is equivalent to "unsupported". Returns false with an error only on
// genuine filesystem failure (permission denied, I/O error).
func SupportsVKD3DHeap(build Build) (bool, error) {
	if build.Path == "" {
		return false, nil
	}
	scriptPath := filepath.Join(build.Path, "proton")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read proton script %s: %w", scriptPath, err)
	}
	// Literal-string match. Brittle by design — the PLAN risks section
	// notes we'll swap to toolmanifest parsing if this becomes noisy.
	return containsBytes(data, []byte("PROTON_VKD3D_HEAP")), nil
}

func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// lookupCompatTool reads config/config.vdf and returns the compat-tool
// name for the given AppID, falling back to the global default under
// key "0". Returns ErrProtonNotResolved with descriptive context if no
// mapping is found or the file cannot be parsed.
func lookupCompatTool(steamRoot string, appID uint64) (string, error) {
	configPath := filepath.Join(steamRoot, "config", "config.vdf")
	f, err := os.Open(configPath)
	if err != nil {
		return "", fmt.Errorf("%w: open %s: %v", ErrProtonNotResolved, configPath, err)
	}
	defer func() { _ = f.Close() }()

	root, err := steam.ParseVDF(f)
	if err != nil {
		return "", fmt.Errorf("%w: parse %s: %v", ErrProtonNotResolved, configPath, err)
	}

	mapping := root.
		GetNode("InstallConfigStore").
		GetNode("Software").
		GetNode("Valve").
		GetNode("Steam").
		GetNode("CompatToolMapping")
	if mapping == nil {
		return "", fmt.Errorf("%w: no CompatToolMapping in %s", ErrProtonNotResolved, configPath)
	}

	// Try per-game first, then global default.
	for _, key := range []string{strconv.FormatUint(appID, 10), "0"} {
		entry := mapping.GetNode(key)
		if entry == nil {
			continue
		}
		if name := entry.GetString("name"); name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("%w: no mapping for appid %d or global default", ErrProtonNotResolved, appID)
}

// locateBuildDir finds the on-disk directory for a compat-tool name,
// checking compatibilitytools.d (community) then steamapps/common
// (built-in Proton). Returns ErrProtonNotResolved if neither exists.
func locateBuildDir(steamRoot, name string) (string, error) {
	candidates := []string{
		filepath.Join(steamRoot, "compatibilitytools.d", name),
		filepath.Join(steamRoot, "steamapps", "common", name),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("%w: build dir for %q not found under %s", ErrProtonNotResolved, name, steamRoot)
}
