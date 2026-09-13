//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
	"gopkg.in/yaml.v3"
)

func TestTUIProfileEditorKinds(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			environment, cleanup := SetupTestEnvironment(t)
			t.Cleanup(cleanup)
			profilePath := filepath.Join(environment.ConfigHome, "spela", "profiles", "1091500.yaml")
			defaultsPath := filepath.Join(environment.ConfigHome, "spela", "profiles", "default.yaml")
			originalDefaults := readEditorFixture(t, defaultsPath)
			session := startTUI(t, environment, size.width, size.height)
			artifacts := editorJourneyArtifacts(t, session, environment, size.width, size.height, "editor-kinds")
			openGameProfile(t, session)

			// Space cycles a finite choice in Browse. Saving remains explicit
			// and is also reachable through the visible Actions menu.
			focusProfileField(t, session, "Super resolution")
			beforeChoice := readEditorFixture(t, profilePath)
			requireScreen(t, session, []string{"Space"}, "Enter: Edit field")
			sendDisplayed(t, session, " ")
			waitProfileField(t, session, "Super resolution", "quality")
			requireScreen(t, session, nil, "Value before edit:", "> Input")
			captureEditorPhase(t, session, artifacts, "01-choice-draft")
			requireEditorFixtureUnchanged(t, profilePath, beforeChoice)
			chooseAction(t, session, "Save")
			waitSavedProfile(t, session, profilePath, beforeChoice)
			requireYAMLValue(t, profilePath, "quality", "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, true, "overrides", "dlss.sr_mode")
			captureEditorPhase(t, session, artifacts, "02-choice-saved")

			// Cycling past the final choice restores inheritance. The state
			// column shows its effective value, while the value is (default).
			beforeReset := readEditorFixture(t, profilePath)
			sendDisplayed(t, session, " ")
			waitProfileField(t, session, "Super resolution", "dlaa")
			sendDisplayed(t, session, " ")
			waitProfileField(t, session, "Super resolution", "(empty)")
			requireEditorFixtureUnchanged(t, profilePath, beforeReset)
			sendDisplayed(t, session, "ctrl-s")
			waitSavedProfile(t, session, profilePath, beforeReset)
			requireYAMLValue(t, profilePath, nil, "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, true, "overrides", "dlss.sr_mode")
			captureEditorPhase(t, session, artifacts, "03-choice-explicit-empty")
			beforeReset = readEditorFixture(t, profilePath)
			sendDisplayed(t, session, " ")
			waitProfileField(t, session, "Super resolution", "(default)")
			requireScreen(t, session, []string{"↳ Default: balanced"}, "> ◆ Super resolution")
			requireEditorFixtureUnchanged(t, profilePath, beforeReset)
			sendDisplayed(t, session, "ctrl-s")
			waitSavedProfile(t, session, profilePath, beforeReset)
			requireYAMLValue(t, profilePath, nil, "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, nil, "overrides", "dlss.sr_mode")
			requireYAMLValue(t, defaultsPath, "balanced", "dlss", "sr_mode")
			captureEditorPhase(t, session, artifacts, "03-choice-inherited")

			// Invalid integer input must remain available for recovery while
			// the previous file stays intact. Cancel/reopen corrects it using
			// displayed controls, without relying on an undisplayed delete key.
			focusProfileField(t, session, "Clock offset")
			requireScreen(t, session, []string{"Enter: Edit field"}, "Space:")
			beforeInteger := readEditorFixture(t, profilePath)
			beginProfileFieldEdit(t, session, "Clock offset", "0")
			typeIntoInput(t, session, "x")
			waitVisible(t, session, "0x")
			sendDisplayed(t, session, "ctrl-s")
			waitVisible(t, session, "Invalid value: invalid integer: 0x")
			requireInputOwnsKeys(t, session)
			requireScreen(t, session, []string{"Edit Clock offset", "> Input", "0x▏"}, "Profile saved!")
			requireEditorFixtureUnchanged(t, profilePath, beforeInteger)
			captureEditorPhase(t, session, artifacts, "04-integer-invalid")
			selectControl(t, session, "Cancel")
			waitProfileField(t, session, "Clock offset", "(default)")
			beginProfileFieldEdit(t, session, "Clock offset", "0")
			typeIntoInput(t, session, "01234")
			waitVisible(t, session, "001234▏")
			requireInputOwnsKeys(t, session)
			requireScreen(t, session, []string{"Edit Clock offset"}, "Invalid value:", "Actions ·")
			captureEditorPhase(t, session, artifacts, "05-integer-corrected")
			selectControl(t, session, "Save")
			waitSavedProfile(t, session, profilePath, beforeInteger)
			requireYAMLValue(t, profilePath, 1234, "gpu", "clock_offset")
			requireYAMLValue(t, profilePath, true, "overrides", "gpu.clock_offset")
			captureEditorPhase(t, session, artifacts, "06-integer-saved")

			// Digits, punctuation, spaces, and locale text are literal input.
			// A local Cancel must retain the path applied by the previous edit.
			focusProfileField(t, session, "Shader cache path")
			requireScreen(t, session, []string{"Enter: Edit field"}, "Space:")
			beforeText := readEditorFixture(t, profilePath)
			beginProfileFieldEdit(t, session, "Shader cache path", "")
			const cachePath = "01234 /åäö/[cache]"
			typeIntoInput(t, session, "01234")
			sendDisplayed(t, session, " ")
			typeIntoInput(t, session, "/åäö/[cache]")
			waitVisible(t, session, cachePath+"▏")
			requireInputOwnsKeys(t, session)
			captureEditorPhase(t, session, artifacts, "07-path-input")
			selectControl(t, session, "Apply")
			waitProfileField(t, session, "Shader cache path", "01234")
			requireEditorFixtureUnchanged(t, profilePath, beforeText)
			beginProfileFieldEdit(t, session, "Shader cache path", cachePath)
			typeIntoInput(t, session, " discarded")
			waitVisible(t, session, "discarded▏")
			selectControl(t, session, "Cancel")
			waitProfileField(t, session, "Shader cache path", "01234")
			requireEditorFixtureUnchanged(t, profilePath, beforeText)
			beginProfileFieldEdit(t, session, "Shader cache path", cachePath)
			requireScreen(t, session, []string{cachePath + "▏"}, "discarded")
			captureEditorPhase(t, session, artifacts, "08-path-cancel-retained-draft")
			selectControl(t, session, "Save")
			waitSavedProfile(t, session, profilePath, beforeText)
			requireYAMLValue(t, profilePath, cachePath, "gpu", "shader_cache_path")
			requireYAMLValue(t, profilePath, true, "overrides", "gpu.shader_cache_path")
			requireYAMLValue(t, profilePath, 1234, "gpu", "clock_offset")
			requireYAMLValue(t, profilePath, nil, "overrides", "dlss.sr_mode")
			requireEditorFixtureUnchanged(t, defaultsPath, originalDefaults)
			captureEditorPhase(t, session, artifacts, "09-path-saved")
			quitTUI(t, session, environment)
		})
	}
}

func TestTUIProfileSpaceCycles(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		for _, scope := range []string{"root", "game"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size.width, size.height, scope), func(t *testing.T) {
				environment, cleanup := SetupTestEnvironment(t)
				t.Cleanup(cleanup)
				defaultsPath := filepath.Join(environment.ConfigHome, "spela", "profiles", "default.yaml")
				gamePath := filepath.Join(environment.ConfigHome, "spela", "profiles", "1091500.yaml")
				activePath := defaultsPath
				if scope == "game" {
					// Inheritance can resolve to true while the field remains
					// (default). Its first Space must still create an explicit pin.
					var defaults, gameProfile profile.Profile
					if err := yaml.Unmarshal(readEditorFixture(t, defaultsPath), &defaults); err != nil {
						t.Fatal(err)
					}
					if err := yaml.Unmarshal(readEditorFixture(t, gamePath), &gameProfile); err != nil {
						t.Fatal(err)
					}
					defaults.Proton.EnableHDR = true
					gameProfile.Proton.EnableHDR = false
					delete(gameProfile.Overrides, "proton.enable_hdr")
					writeYAML(t, defaultsPath, &defaults)
					writeYAML(t, gamePath, &gameProfile)
					activePath = gamePath
				}
				originalDefaults := readEditorFixture(t, defaultsPath)
				originalGame := readEditorFixture(t, gamePath)
				before := readEditorFixture(t, activePath)
				session := startTUI(t, environment, size.width, size.height)
				artifacts := editorJourneyArtifacts(t, session, environment, size.width, size.height, "space-"+scope)
				if scope == "root" {
					sendDisplayed(t, session, "up")
					waitVisible(t, session, "> All games")
					sendDisplayed(t, session, "enter")
					waitVisible(t, session, "▸ Detail")
				} else {
					openGameProfile(t, session)
					waitProfileField(t, session, "HDR", "(default)")
					requireScreen(t, session, []string{"↳ Default: true"}, "> ◆ HDR")

					// The same displayed Space selects a game when List owns
					// focus. It must not change the profile behind that pane.
					sendDisplayed(t, session, "tab")
					waitVisible(t, session, "▸ List")
					sendDisplayed(t, session, " ")
					waitVisible(t, session, "Selected: 1 total, 1 visible")
					requireEditorFixtureUnchanged(t, activePath, before)
					sendDisplayed(t, session, " ")
					waitForScreen(t, session, "cleared List selection", func(screen string) bool {
						return !strings.Contains(screen, "Selected:")
					})
					sendDisplayed(t, session, "tab")
					waitVisible(t, session, "▸ Detail")
					waitProfileField(t, session, "HDR", "(default)")
					requireScreen(t, session, nil, "Unsaved changes")
				}

				for _, field := range []struct {
					label, artifact string
					values          []string
				}{
					{"HDR", "hdr", []string{"true", "false", "(default)"}},
					{"VKD3D heap", "vkd3d", []string{"true", "false", "(default)"}},
					{"Ray reconstruction", "ray", []string{"off", "ultra_performance", "performance", "balanced", "quality", "dlaa", "(empty)", "(default)"}},
					{"Multi-frame", "multi-frame", []string{"0", "1", "2", "3", "4", "(default)"}},
				} {
					focusProfileField(t, session, field.label)
					waitProfileField(t, session, field.label, "(default)")
					for index, expected := range field.values {
						requireScreen(t, session, []string{"Space"}, "Enter: Edit field")
						sendDisplayed(t, session, " ")
						waitProfileField(t, session, field.label, expected)
						requireScreen(t, session, nil, "Value before edit:", "> Input")
						if scope == "game" {
							if expected == "(default)" {
								requireScreen(t, session, nil, "> ◆ "+field.label)
							} else {
								requireScreen(t, session, []string{"> ◆ " + field.label})
							}
						}
						requireEditorFixtureUnchanged(t, activePath, before)
						if field.artifact == "hdr" || field.artifact == "vkd3d" || expected == "(default)" {
							captureEditorPhase(t, session, artifacts, fmt.Sprintf("%s-%02d", field.artifact, index+1))
						}
					}
					// A full cycle restores the original draft, including pins.
					requireScreen(t, session, nil, "Unsaved changes", "Ctrl+S")
				}
				requireEditorFixtureUnchanged(t, defaultsPath, originalDefaults)
				requireEditorFixtureUnchanged(t, gamePath, originalGame)
				quitTUI(t, session, environment)
			})
		}
	}
}

// The focused profile row is immediately followed by its Effect explanation.
// Reading this visible pair avoids confusing the selected Library game with
// the active field when both panes display a cursor at the wider size.
func selectedProfileField(screen string) string {
	lines := strings.Split(screen, "\n")
	for index, line := range lines {
		if index+1 >= len(lines) || !strings.Contains(lines[index+1], "Effect:") {
			continue
		}
		marker := strings.LastIndex(line, "> ")
		if marker < 0 {
			continue
		}
		row := strings.TrimPrefix(line[marker+2:], "◆ ")
		if border := strings.Index(row, "│"); border >= 0 {
			row = row[:border]
		}
		return strings.TrimSpace(row)
	}
	return ""
}

func focusProfileField(t *testing.T, session *Session, label string) {
	t.Helper()
	seen := make(map[string]bool)
	for range 64 {
		screen := requireScreen(t, session, []string{"▸ Detail"})
		if !strings.Contains(screen, "[Profile]") && !strings.Contains(screen, "All games (default profile)") {
			t.Fatalf("profile navigation requires the visible root or game Profile view:\n%s", screen)
		}
		selected := selectedProfileField(screen)
		if strings.HasPrefix(selected, label+"  ") {
			return
		}
		if selected == "" || seen[selected] {
			t.Fatalf("profile field %q was not reachable through visible Down navigation; selected %q:\n%s", label, selected, screen)
		}
		seen[selected] = true
		sendDisplayed(t, session, "down")
		waitForScreen(t, session, "next visible profile field", func(next string) bool {
			return selectedProfileField(next) != selected
		})
	}
	t.Fatalf("profile navigation did not reach %q", label)
}

func waitProfileField(t *testing.T, session *Session, label, value string) {
	t.Helper()
	waitForScreen(t, session, label+" in profile Browse", func(screen string) bool {
		selected := selectedProfileField(screen)
		if !strings.HasPrefix(selected, label+"  ") || strings.Contains(screen, "Value before edit:") {
			return false
		}
		columns := strings.Fields(strings.TrimPrefix(selected, label))
		return len(columns) > 0 && columns[0] == value
	})
}

func beginProfileFieldEdit(t *testing.T, session *Session, label, value string) {
	t.Helper()
	sendDisplayed(t, session, "enter")
	waitVisible(t, session, "Edit "+label)
	requireInputOwnsKeys(t, session)
	requireScreen(t, session, []string{"Value before edit: " + value, "> Input", "Apply", "Cancel", "Save"})
}

func readEditorFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireEditorFixtureUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()
	if after := readEditorFixture(t, path); !bytes.Equal(before, after) {
		t.Fatalf("unsaved editor changed %s:\nbefore:\n%s\nafter:\n%s", path, before, after)
	}
}

func waitSavedProfile(t *testing.T, session *Session, path string, before []byte) {
	t.Helper()
	waitForScreen(t, session, "profile save to complete", func(screen string) bool {
		after, err := os.ReadFile(path)
		return err == nil && !bytes.Equal(before, after) && strings.Contains(screen, "Profile saved!") && !strings.Contains(screen, "Value before edit:") && !strings.Contains(screen, "Unsaved changes")
	})
}

func captureEditorPhase(t *testing.T, session *Session, directory, phase string) {
	t.Helper()
	screen := requireScreen(t, session, nil)
	t.Logf("%s rendered screen:\n%s", phase, screen)
	if directory != "" {
		if err := os.WriteFile(filepath.Join(directory, phase+".txt"), []byte(screen), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func editorJourneyArtifacts(t *testing.T, session *Session, environment *TestEnvironment, width, height int, journey string) string {
	t.Helper()
	root := os.Getenv("SPELA_E2E_EDITOR_ARTIFACTS")
	if root == "" {
		return ""
	}
	directory := filepath.Join(root, fmt.Sprintf("%dx%d", width, height), journey)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, artifact := range []struct {
			name      string
			arguments []string
		}{
			{"last-screen.txt", []string{"show", session.name}},
			{"terminal.ansi", []string{"logs", session.name, "--ansi"}},
			{"status.txt", []string{"status", session.name}},
		} {
			output, err := session.command(artifact.arguments...).Output()
			if err != nil {
				t.Error(err)
				continue
			}
			if err := os.WriteFile(filepath.Join(directory, artifact.name), output, 0o600); err != nil {
				t.Error(err)
			}
		}
		for _, name := range []string{"1091500.yaml", "default.yaml"} {
			data, err := os.ReadFile(filepath.Join(environment.ConfigHome, "spela", "profiles", name))
			if err == nil {
				err = os.WriteFile(filepath.Join(directory, name), data, 0o600)
			}
			if err != nil {
				t.Error(err)
			}
		}
	})
	return directory
}
