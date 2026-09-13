//go:build e2e

package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"gopkg.in/yaml.v3"
)

func TestTUIDirectDLLControls(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			environment, cleanup := SetupTestEnvironment(t)
			t.Cleanup(cleanup)
			payloads := prepareDirectDLLFixture(t, environment)
			gameDirectory := filepath.Join(environment.TempDir, "games", "Cyberpunk 2077", "bin", "x64")
			path := filepath.Join(gameDirectory, "nvngx_dlss.dll")
			otherPath := filepath.Join(gameDirectory, "nvngx_dlssg.dll")
			hiddenPath := filepath.Join(environment.TempDir, "games", "The Witcher 3", "bin", "nvngx_dlss.dll")
			original := readEditorFixture(t, path)
			otherOriginal := readEditorFixture(t, otherPath)
			hiddenOriginal := readEditorFixture(t, hiddenPath)
			session := startTUI(t, environment, size.width, size.height)
			artifacts := editorJourneyArtifacts(t, session, environment, size.width, size.height, "direct-dll-controls")

			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			waitVisible(t, session, "[Overview]")
			sendDisplayed(t, session, "right")
			waitVisible(t, session, "[Profile]")
			sendDisplayed(t, session, "right")
			waitVisible(t, session, "DLL actions: Cyberpunk 2077")
			waitVisible(t, session, "Update DLLs (1 file)")
			requireScreen(t, session, []string{"▸ Install DLL", "Restore originals (0 files)", "no backup for this game", "Enter"}, "Enter: Edit field", "Ctrl+S")
			captureEditorPhase(t, session, artifacts, "01-direct-controls-hints-off")

			// Inactive Detail must offer a focus path, not its own active keys.
			sendDisplayed(t, session, "tab")
			waitVisible(t, session, "▸ List")
			if size.width == 120 {
				requireScreen(t, session, []string{"DLL actions: Cyberpunk 2077", "Tab: Focus DLL controls"}, "↑/↓ Choose action", "Enter: Install DLL", "Enter Install DLL", "▸ Install DLL")
			} else {
				requireScreen(t, session, nil, "DLL actions: Cyberpunk 2077")
			}
			captureEditorPhase(t, session, artifacts, "02-inactive-detail")
			sendDisplayed(t, session, "tab")
			waitVisible(t, session, "▸ Detail")

			// Update opens directly from its visible row. Enter defaults to Cancel.
			sendDisplayed(t, session, "down")
			waitVisible(t, session, "▸ Update DLLs (1 file)")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Confirm DLL update")
			requireScreen(t, session, []string{"1 DLL target(s)", "nvngx_dlss.dll", "[ Cancel ]", "[ Confirm ]"}, "The Witcher 3: Wild Hunt ·")
			captureEditorPhase(t, session, artifacts, "03-update-confirmation")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "DLL update cancelled")
			requireEditorFixtureUnchanged(t, path, original)
			closeDirectDLLResult(t, session)
			waitVisible(t, session, "▸ Update DLLs")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Confirm DLL update")
			sendDisplayed(t, session, "right")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "DLL update: 1 updated, 0 current, 0 failed")
			requireDLLBytes(t, path, payloads["dlss"])
			requireDLLBytes(t, otherPath, otherOriginal)
			requireDLLBytes(t, hiddenPath, hiddenOriginal)
			captureEditorPhase(t, session, artifacts, "04-update-completed")
			closeDirectDLLResult(t, session)

			// Restore originals restores the backup; cancellation leaves the update.
			sendDisplayed(t, session, "down")
			waitVisible(t, session, "▸ Restore originals (2 files)")
			requireScreen(t, session, []string{"Newly added DLLs stay installed", "Enter"})
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Confirm DLL restore")
			captureEditorPhase(t, session, artifacts, "05-restore-confirmation")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "DLL restore cancelled")
			requireDLLBytes(t, path, payloads["dlss"])
			closeDirectDLLResult(t, session)
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Confirm DLL restore")
			sendDisplayed(t, session, "right")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "DLL restore completed")
			requireDLLBytes(t, path, original)
			requireDLLBytes(t, otherPath, otherOriginal)
			requireDLLBytes(t, hiddenPath, hiddenOriginal)
			captureEditorPhase(t, session, artifacts, "06-originals-restored")
			closeDirectDLLResult(t, session)

			// Install is also directly reachable for a game with no detected DLL.
			sendDisplayed(t, session, "tab")
			waitVisible(t, session, "▸ List")
			sendDisplayed(t, session, "down")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			waitVisible(t, session, "[Overview]")
			sendDisplayed(t, session, "right")
			waitVisible(t, session, "[Profile]")
			sendDisplayed(t, session, "right")
			waitVisible(t, session, "DLL actions: Elden Ring")
			requireScreen(t, session, []string{"▸ Install DLL", "no stale DLLs", "no backup"})
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Select a supported DLL family")
			waitVisible(t, session, "> DLSS")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Select DLSS version")
			waitVisible(t, session, "> 3.8.0")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Confirm DLL install")
			requireScreen(t, session, []string{"Elden Ring ·", "1 DLL target(s)"}, "Cyberpunk 2077 ·", "The Witcher 3: Wild Hunt ·")
			captureEditorPhase(t, session, artifacts, "07-install-confirmation")
			sendDisplayed(t, session, "right")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "DLL install completed")
			installed := filepath.Join(environment.TempDir, "games", "ELDEN RING", "nvngx_dlss.dll")
			requireDLLBytes(t, installed, payloads["dlss"])
			requireDLLBytes(t, path, original)
			requireDLLBytes(t, hiddenPath, hiddenOriginal)
			closeDirectDLLResult(t, session)
			sendDisplayed(t, session, "down")
			sendDisplayed(t, session, "down")
			waitVisible(t, session, "▸ Restore originals (0 files)")
			requireScreen(t, session, []string{"no backup for this game"}, "Enter:", "Enter ")
			captureEditorPhase(t, session, artifacts, "08-no-originals-to-restore")
			if artifacts != "" {
				writeDirectDLLHashes(t, filepath.Join(artifacts, "final-file-hashes.json"), path, otherPath, hiddenPath, installed)
			}
			chooseAction(t, session, "Quit")
		})
	}
}

func closeDirectDLLResult(t *testing.T, session *Session) {
	t.Helper()
	waitVisible(t, session, "Tab Close")
	sendDisplayed(t, session, "tab")
	waitVisible(t, session, "Enter Close")
	sendDisplayed(t, session, "enter")
	waitVisible(t, session, "DLL actions:")
}

func prepareDirectDLLFixture(t *testing.T, environment *TestEnvironment) map[string][]byte {
	t.Helper()
	configurationPath := filepath.Join(environment.ConfigHome, "spela", "config.yaml")
	var configuration map[string]interface{}
	if err := yaml.Unmarshal(readEditorFixture(t, configurationPath), &configuration); err != nil {
		t.Fatal(err)
	}
	configuration["show_hints"] = false
	writeYAML(t, configurationPath, configuration)
	manifestPath := filepath.Join(environment.CacheHome, "spela", "manifest.json")
	var manifest dll.Manifest
	if err := json.Unmarshal(readEditorFixture(t, manifestPath), &manifest); err != nil {
		t.Fatal(err)
	}
	payloads := make(map[string][]byte)
	for family, entries := range manifest.DLLs {
		for index := range entries {
			entries[index].Filename = "nvngx_" + family + ".dll"
			payload := []byte("direct DLL fixture " + family + " " + entries[index].Version)
			hash := sha256.Sum256(payload)
			entries[index].SHA256 = hex.EncodeToString(hash[:])
			entries[index].Size = int64(len(payload))
			path := filepath.Join(environment.CacheHome, "spela", "dlls", family, entries[index].Version+".dll")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			writeDummyFile(t, path, string(payload))
			if index == 0 {
				payloads[family] = payload
			}
		}
		manifest.DLLs[family] = entries
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		err = os.WriteFile(manifestPath, data, 0o600)
	}
	if err != nil {
		t.Fatal(err)
	}
	return payloads
}

func requireDLLBytes(t *testing.T, path string, expected []byte) {
	t.Helper()
	if actual := readEditorFixture(t, path); !bytes.Equal(actual, expected) {
		t.Fatalf("unexpected DLL payload at %s: got %q, want %q", path, actual, expected)
	}
}

func writeDirectDLLHashes(t *testing.T, target string, paths ...string) {
	t.Helper()
	hashes := make(map[string]string, len(paths))
	for _, path := range paths {
		hash := sha256.Sum256(readEditorFixture(t, path))
		hashes[path] = hex.EncodeToString(hash[:])
	}
	data, err := json.MarshalIndent(hashes, "", "  ")
	if err == nil {
		err = os.WriteFile(target, []byte(strings.TrimSpace(string(data))+"\n"), 0o600)
	}
	if err != nil {
		t.Fatal(err)
	}
}
