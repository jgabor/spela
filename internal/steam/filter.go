package steam

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Known Steam tool AppIDs that should not appear in game lists.
var knownToolAppIDs = map[uint64]bool{
	// Proton versions
	1493710: true, // Proton Experimental
	2805730: true, // Proton 9.0
	2348590: true, // Proton 8.0
	2180100: true, // Proton Hotfix
	1887720: true, // Proton 7.0
	1580130: true, // Proton 6.3
	1245040: true, // Proton 5.0
	858280:  true, // Proton 4.2
	1054830: true, // Proton 4.11
	961940:  true, // Proton 3.16
	930400:  true, // Proton 3.7

	// Steam Linux Runtimes
	1628350: true, // Steam Linux Runtime - Sniper
	1070560: true, // Steam Linux Runtime - Soldier
	1391110: true, // Steam Linux Runtime - Scout

	// Steamworks and SDK tools
	228980: true, // Steamworks Common Redistributables

	// Steam platform tools
	250820:  true, // SteamVR
	1007:    true, // Steam Client
	1158310: true, // Steam Streaming Speakers
	1260320: true, // Steam Input Configurator
	1675200: true, // Steam Linux Runtime - Medic (pressure-vessel debug)
	1826330: true, // Steam Proton EasyAntiCheat Runtime
	2394010: true, // Steam for Chrome OS
	2180060: true, // Steam Play None
	1161040: true, // Steam Linux Media Server
	1245020: true, // Proton BattlEye Runtime
	1826340: true, // Steam Runtime EasyAntiCheat
	3244930: true, // Steam Linux Runtime - Heavy
	3305210: true, // Steam Linux Runtime - Beryllium
	3283470: true, // Steam Linux Runtime 2.0
	3283410: true, // Steam Linux Runtime 3.0
	3316210: true, // Proton 10.0 (or later versions)
	3026190: true, // Proton Next
}

// Name patterns that indicate a tool rather than a game.
var toolNamePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^proton\s`),
	regexp.MustCompile(`(?i)^steam\s+linux\s+runtime`),
	regexp.MustCompile(`(?i)^steamworks`),
	regexp.MustCompile(`(?i)redistributable`),
	regexp.MustCompile(`(?i)^steam\s+controller`),
}

// IsTool returns true if the given app appears to be a Steam tool
// (Proton, Runtime, SDK) rather than a game.
func IsTool(appID uint64, name, installDir string) bool {
	if knownToolAppIDs[appID] {
		return true
	}

	if hasToolManifest(installDir) {
		return true
	}

	if matchesToolNamePattern(name) {
		return true
	}

	return false
}

// hasToolManifest checks if the install directory contains a toolmanifest.vdf,
// which is present in Proton and other Steam tools.
func hasToolManifest(installDir string) bool {
	manifestPath := filepath.Join(installDir, "toolmanifest.vdf")
	info, err := os.Stat(manifestPath)
	return err == nil && !info.IsDir()
}

// matchesToolNamePattern checks if the name matches known tool patterns.
func matchesToolNamePattern(name string) bool {
	name = strings.TrimSpace(name)
	for _, pattern := range toolNamePatterns {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}
