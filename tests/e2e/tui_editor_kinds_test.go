//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			artifacts := editorJourneyArtifacts(t, session, environment, size.width, size.height)
			openGameProfile(t, session)

			// Choice Apply changes only the draft. Cancelling a later edit must
			// keep that earlier choice, and Save persists the explicit override.
			focusProfileField(t, session, "Super resolution")
			beforeChoice := readEditorFixture(t, profilePath)
			beginProfileFieldEdit(t, session, "Super resolution", "balanced")
			sendDisplayed(t, session, "right")
			waitVisible(t, session, "quality")
			captureEditorPhase(t, session, artifacts, "01-choice-input")
			selectControl(t, session, "Apply")
			waitProfileField(t, session, "Super resolution", "quality")
			requireEditorFixtureUnchanged(t, profilePath, beforeChoice)
			beginProfileFieldEdit(t, session, "Super resolution", "quality")
			sendDisplayed(t, session, "right")
			waitVisible(t, session, "dlaa")
			selectControl(t, session, "Cancel")
			waitProfileField(t, session, "Super resolution", "quality")
			requireEditorFixtureUnchanged(t, profilePath, beforeChoice)
			beginProfileFieldEdit(t, session, "Super resolution", "quality")
			selectControl(t, session, "Save")
			waitSavedProfile(t, session, profilePath, beforeChoice)
			requireYAMLValue(t, profilePath, "quality", "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, true, "overrides", "dlss.sr_mode")
			captureEditorPhase(t, session, artifacts, "02-choice-saved")

			// Reset returns to the effective default immediately, but removes
			// the game value and its pin from YAML only after an explicit Save.
			beforeReset := readEditorFixture(t, profilePath)
			chooseAction(t, session, "Reset field to inherit")
			waitProfileField(t, session, "Super resolution", "balanced")
			requireEditorFixtureUnchanged(t, profilePath, beforeReset)
			sendDisplayed(t, session, "ctrl-s")
			waitSavedProfile(t, session, profilePath, beforeReset)
			requireYAMLValue(t, profilePath, nil, "dlss", "sr_mode")
			requireYAMLValue(t, profilePath, nil, "overrides", "dlss.sr_mode")
			requireYAMLValue(t, defaultsPath, "balanced", "dlss", "sr_mode")
			beginProfileFieldEdit(t, session, "Super resolution", "balanced")
			captureEditorPhase(t, session, artifacts, "03-choice-inherited")
			selectControl(t, session, "Cancel")
			waitProfileField(t, session, "Super resolution", "balanced")

			// Invalid integer input must remain available for recovery while
			// the previous file stays intact. Cancel/reopen corrects it using
			// displayed controls, without relying on an undisplayed delete key.
			focusProfileField(t, session, "Clock offset")
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
			beforeText := readEditorFixture(t, profilePath)
			beginProfileFieldEdit(t, session, "Shader cache path", "")
			const cachePath = "01234 /åäö/[cache]"
			typeIntoInput(t, session, cachePath)
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
		screen := requireScreen(t, session, []string{"[Profile]", "▸ Detail"})
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
		return strings.HasPrefix(selected, label+"  ") && strings.Contains(selected, value) && !strings.Contains(screen, "Value before edit:")
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

func editorJourneyArtifacts(t *testing.T, session *Session, environment *TestEnvironment, width, height int) string {
	t.Helper()
	root := os.Getenv("SPELA_E2E_EDITOR_ARTIFACTS")
	if root == "" {
		return ""
	}
	directory := filepath.Join(root, fmt.Sprintf("%dx%d", width, height))
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
