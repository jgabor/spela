//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

			// A finite choice opens a picker. Typing jumps to a choice, Enter
			// applies it, and the change is written at once.
			focusProfileField(t, session, "Super resolution")
			beforeChoice := readEditorFixture(t, profilePath)
			requireScreen(t, session, []string{"Space", "Enter: Edit value"}, "Ctrl+S")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Edit Super resolution")
			requireScreen(t, session, []string{"Follow All games (balanced)", "Type to jump", "Esc cancel", "> balanced"}, "Value before edit:")
			captureEditorPhase(t, session, artifacts, "01-choice-picker")
			requireEditorFixtureUnchanged(t, profilePath, beforeChoice)
			typeIntoPicker(t, session, "q")
			waitVisible(t, session, "> quality")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Saved: Super resolution quality")
			waitSavedProfile(t, session, profilePath, beforeChoice)
			requireYAMLValue(t, profilePath, "quality", "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, true, "overrides", "dlss.sr_mode")
			waitProfileField(t, session, "Super resolution", "quality")
			captureEditorPhase(t, session, artifacts, "02-choice-saved")

			// Space follows All games and the next Space puts the value back.
			beforeFollow := readEditorFixture(t, profilePath)
			sendDisplayed(t, session, " ")
			waitVisible(t, session, "Saved: Super resolution follows All games")
			waitSavedProfile(t, session, profilePath, beforeFollow)
			waitProfileRow(t, session, "Super resolution", "balanced", "from All games")
			requireYAMLValue(t, profilePath, nil, "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, nil, "overrides", "dlss.sr_mode")
			captureEditorPhase(t, session, artifacts, "03-choice-follows")
			beforeBack := readEditorFixture(t, profilePath)
			sendDisplayed(t, session, " ")
			waitVisible(t, session, "Saved: Super resolution quality")
			waitSavedProfile(t, session, profilePath, beforeBack)
			requireYAMLValue(t, profilePath, "quality", "dlss", "sr_mode")

			// The explicit empty value is a choice of its own.
			beforeNone := readEditorFixture(t, profilePath)
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Edit Super resolution")
			typeIntoPicker(t, session, "n")
			waitVisible(t, session, "> none")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Saved: Super resolution none")
			waitSavedProfile(t, session, profilePath, beforeNone)
			requireYAMLValue(t, profilePath, nil, "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, true, "overrides", "dlss.sr_mode")
			captureEditorPhase(t, session, artifacts, "04-choice-explicit-empty")

			// Esc leaves a picker without a change.
			beforeEscape := readEditorFixture(t, profilePath)
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Edit Super resolution")
			typeIntoPicker(t, session, "d")
			sendDisplayed(t, session, "escape")
			waitProfileField(t, session, "Super resolution", "none")
			requireEditorFixtureUnchanged(t, profilePath, beforeEscape)

			// Picking the follow row clears the value.
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Edit Super resolution")
			typeIntoPicker(t, session, "f")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Saved: Super resolution follows All games")
			waitSavedProfile(t, session, profilePath, beforeEscape)
			requireYAMLValue(t, profilePath, nil, "overrides", "dlss.sr_mode")
			requireYAMLValue(t, defaultsPath, "balanced", "dlss", "sr_mode")

			// Invalid integer input stays in the editor with its error while the
			// file is untouched. Cancel and reopen to correct it.
			focusProfileField(t, session, "Clock offset")
			requireScreen(t, session, []string{"Enter: Edit value"})
			beforeInteger := readEditorFixture(t, profilePath)
			beginProfileFieldEdit(t, session, "Clock offset", "0")
			requireScreen(t, session, []string{"Typing replaces the value", "Backspace delete"})
			typeIntoInput(t, session, "x")
			waitVisible(t, session, "x▏")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Invalid value: invalid integer: x")
			requireInputOwnsKeys(t, session)
			requireScreen(t, session, []string{"Edit Clock offset", "> Input", "x▏"}, "Saved: Clock offset")
			requireEditorFixtureUnchanged(t, profilePath, beforeInteger)
			captureEditorPhase(t, session, artifacts, "05-integer-invalid")
			selectControl(t, session, "Cancel")
			waitProfileField(t, session, "Clock offset", "none")
			beginProfileFieldEdit(t, session, "Clock offset", "0")
			typeIntoInput(t, session, "01234")
			waitVisible(t, session, "01234▏")
			requireInputOwnsKeys(t, session)
			requireScreen(t, session, []string{"Edit Clock offset"}, "Invalid value:", "Actions ·")
			captureEditorPhase(t, session, artifacts, "06-integer-corrected")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Saved: Clock offset 1234 MHz")
			waitSavedProfile(t, session, profilePath, beforeInteger)
			requireYAMLValue(t, profilePath, 1234, "gpu", "clock_offset")
			requireYAMLValue(t, profilePath, true, "overrides", "gpu.clock_offset")
			captureEditorPhase(t, session, artifacts, "07-integer-saved")

			// Space follows All games and puts the number back.
			sendDisplayed(t, session, " ")
			waitVisible(t, session, "Saved: Clock offset follows All games")
			waitForScreen(t, session, "clock offset follows", func(screen string) bool {
				data, err := os.ReadFile(profilePath)
				return err == nil && !strings.Contains(string(data), "gpu.clock_offset")
			})
			sendDisplayed(t, session, " ")
			waitVisible(t, session, "Saved: Clock offset 1234 MHz")
			requireYAMLValue(t, profilePath, 1234, "gpu", "clock_offset")

			// Digits, punctuation, spaces, and locale text are literal input.
			focusProfileField(t, session, "Shader cache path")
			requireScreen(t, session, []string{"Enter: Edit value"})
			beforeText := readEditorFixture(t, profilePath)
			beginProfileFieldEdit(t, session, "Shader cache path", "")
			const cachePath = "01234 /åäö/[cache]"
			typeIntoInput(t, session, "01234")
			sendDisplayed(t, session, " ")
			typeIntoInput(t, session, "/åäö/[cache]")
			waitVisible(t, session, cachePath+"▏")
			requireInputOwnsKeys(t, session)
			captureEditorPhase(t, session, artifacts, "08-path-input")
			selectControl(t, session, "Apply")
			waitVisible(t, session, "Saved: Shader cache path")
			waitSavedProfile(t, session, profilePath, beforeText)
			requireYAMLValue(t, profilePath, cachePath, "gpu", "shader_cache_path")
			requireYAMLValue(t, profilePath, true, "overrides", "gpu.shader_cache_path")
			requireYAMLValue(t, profilePath, 1234, "gpu", "clock_offset")

			// Esc abandons an edit and keeps the saved path.
			afterText := readEditorFixture(t, profilePath)
			beginProfileFieldEdit(t, session, "Shader cache path", cachePath)
			typeIntoInput(t, session, " discarded")
			waitVisible(t, session, "discarded▏")
			sendDisplayed(t, session, "escape")
			waitForScreen(t, session, "abandoned path edit", func(screen string) bool {
				return !strings.Contains(screen, "Value before edit:") && !strings.Contains(screen, "discarded")
			})
			requireEditorFixtureUnchanged(t, profilePath, afterText)
			requireEditorFixtureUnchanged(t, defaultsPath, originalDefaults)
			captureEditorPhase(t, session, artifacts, "09-path-saved")
			quitTUI(t, session, environment)
		})
	}
}

func TestTUIProfileSpaceTogglesFollow(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		for _, scope := range []string{"root", "game"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size.width, size.height, scope), func(t *testing.T) {
				environment, cleanup := SetupTestEnvironment(t)
				t.Cleanup(cleanup)
				defaultsPath := filepath.Join(environment.ConfigHome, "spela", "profiles", "default.yaml")
				gamePath := filepath.Join(environment.ConfigHome, "spela", "profiles", "1091500.yaml")
				activePath := defaultsPath
				if scope == "game" {
					// All games sets HDR On while the game follows it: the first Space
					// must set the opposite on the game.
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
					waitProfileField(t, session, "HDR", "On")
					requireScreen(t, session, []string{"from All games"}, "> ◆ HDR")

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
					waitProfileField(t, session, "HDR", "On")
				}

				for _, field := range []struct {
					label, artifact, key string
					initial, other       string
				}{
					{"HDR", "hdr", "proton.enable_hdr", map[bool]string{true: "On", false: "Off"}[scope == "game"], map[bool]string{true: "Off", false: "On"}[scope == "game"]},
					{"VKD3D heap", "vkd3d", "proton.vkd3d_heap", "Off", "On"},
				} {
					focusProfileField(t, session, field.label)
					waitProfileField(t, session, field.label, field.initial)
					fileBefore := readEditorFixture(t, activePath)
					for round := 0; round < 2; round++ {
						// Following to an own value takes the opposite.
						requireScreen(t, session, []string{"Space"}, "Ctrl+S", "Value before edit:")
						sendDisplayed(t, session, " ")
						source := "set here"
						if scope == "game" {
							source = "this game"
						}
						waitProfileRow(t, session, field.label, field.other, source)
						waitForScreen(t, session, field.label+" written", func(string) bool {
							return !bytes.Equal(fileBefore, readEditorFixture(t, activePath))
						})
						requireYAMLValue(t, activePath, true, "overrides", field.key)
						waitVisible(t, session, "Saved: "+field.label)
						requireScreen(t, session, nil, "Unsaved changes", "Value before edit:")
						captureEditorPhase(t, session, artifacts, fmt.Sprintf("%s-%d-own", field.artifact, round+1))

						// The same key follows again and the file returns to its start.
						sendDisplayed(t, session, " ")
						source = "system default"
						if scope == "game" && field.label == "HDR" {
							source = "from All games"
						}
						waitProfileRow(t, session, field.label, field.initial, source)
						waitForScreen(t, session, field.label+" follows again in the file", func(string) bool {
							return !overrideSet(activePath, field.key)
						})
					}
				}
				// Only the open profile was written.
				if scope == "game" {
					requireEditorFixtureUnchanged(t, defaultsPath, originalDefaults)
				} else {
					requireEditorFixtureUnchanged(t, gamePath, originalGame)
				}
				quitTUI(t, session, environment)
			})
		}
	}
}

// overrideSet reports whether a profile file pins a field.
func overrideSet(path, field string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var document struct {
		Overrides map[string]bool `yaml:"overrides"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return true
	}
	return document.Overrides[field]
}

// The focused profile row carries a source column (system default, from All
// games, this game, set here). Requiring it avoids confusing the selected
// Library game with the active field when both panes display a cursor at the
// wider size.
func selectedProfileField(screen string) string {
	sources := []string{"system default", "from All games", "this game", "set here"}
	for _, line := range strings.Split(screen, "\n") {
		marker := strings.LastIndex(line, "> ")
		if marker < 0 {
			continue
		}
		row := strings.TrimPrefix(line[marker+2:], "◆ ")
		if border := strings.Index(row, "│"); border >= 0 {
			row = row[:border]
		}
		if !slices.ContainsFunc(sources, func(source string) bool { return strings.Contains(row, source) }) {
			continue
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
		if !strings.Contains(screen, "Own settings") && !strings.Contains(screen, "Games follow these values") {
			t.Fatalf("profile navigation requires the visible root or game profile page:\n%s", screen)
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

// waitProfileRow waits for the focused row to show a value and a source. Values
// alone repeat during a rotation (a pinned Off looks like an inherited Off).
func waitProfileRow(t *testing.T, session *Session, label, value, source string) {
	t.Helper()
	waitForScreen(t, session, label+" "+value+" from "+source, func(screen string) bool {
		selected := selectedProfileField(screen)
		if !strings.HasPrefix(selected, label+"  ") || strings.Contains(screen, "Value before edit:") {
			return false
		}
		columns := strings.Fields(strings.TrimPrefix(selected, label))
		return len(columns) > 0 && columns[0] == value && strings.Contains(selected, source)
	})
}

func beginProfileFieldEdit(t *testing.T, session *Session, label, value string) {
	t.Helper()
	sendDisplayed(t, session, "enter")
	waitVisible(t, session, "Edit "+label)
	requireInputOwnsKeys(t, session)
	requireScreen(t, session, []string{"Value before edit: " + value, "> Input", "Apply", "Cancel"}, "Cancel  Save", "Ctrl+S")
}

// typeIntoPicker types letters to jump within the open picker. They are not
// text for an input, so the helper only requires that the picker is shown.
func typeIntoPicker(t *testing.T, session *Session, text string) {
	t.Helper()
	requireScreen(t, session, []string{"Type to jump"})
	if err := session.SendText(text); err != nil {
		t.Fatal(err)
	}
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
		return err == nil && !bytes.Equal(before, after) && strings.Contains(screen, "Saved: ") && !strings.Contains(screen, "Value before edit:") && !strings.Contains(screen, "Unsaved changes")
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
