//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/profile"
	"gopkg.in/yaml.v3"
)

const visibleTimeout = 10 * time.Second

func startTUI(t *testing.T, environment *TestEnvironment, width, height int) *Session {
	t.Helper()
	session, err := NewSession("spela-tui-e2e", width, height, testBinaryPath, []string{"tui"}, environment.Env, environment.TerminalControlRuntimeDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if screen, err := session.Capture(); err == nil {
			t.Logf("final rendered screen before cleanup:\n%s", screen)
		}
		if err := session.Close(); err != nil {
			t.Errorf("cleanup Terminal Control session: %v", err)
		}
	})
	if err := session.WaitForText("Library", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	return session
}

func requireScreen(t *testing.T, session *Session, contains []string, excludes ...string) string {
	t.Helper()
	screen, err := session.Capture()
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range contains {
		if !strings.Contains(screen, text) {
			t.Errorf("rendered screen does not contain %q:\n%s", text, screen)
		}
	}
	for _, text := range excludes {
		if strings.Contains(screen, text) {
			t.Errorf("rendered screen unexpectedly contains %q:\n%s", text, screen)
		}
	}
	return screen
}

func requireClosedSplitPaneBorders(t *testing.T, screen string) {
	t.Helper()
	lines := strings.Split(screen, "\n")
	detailTop := -1
	for index, line := range lines {
		line = strings.TrimRight(line, " ")
		if strings.Contains(line, "Detail") && strings.HasSuffix(line, "╮") {
			detailTop = index
			break
		}
	}
	if detailTop < 0 {
		t.Fatalf("Detail top border is missing:\n%s", screen)
	}
	for index, line := range lines[detailTop+1:] {
		line = strings.TrimRight(line, " ")
		if strings.HasPrefix(line, "╰") && strings.HasSuffix(line, "╯") {
			return
		}
		if !strings.HasSuffix(line, "│") {
			t.Fatalf("Detail right border is open on row %d:\n%s", detailTop+index+1, screen)
		}
	}
	t.Fatalf("Detail bottom border is missing:\n%s", screen)
}

func requireCleanLiveHeader(t *testing.T, screen string, width int) int {
	t.Helper()
	lines := strings.Split(strings.TrimRight(screen, "\n"), "\n")
	barRow := 7
	if width == 80 {
		barRow = 3
		if len(lines) == 0 || !strings.Contains(lines[0], "GPU ") || strings.Contains(screen, "███████╗") {
			t.Fatalf("minimum layout did not use the compact metrics header:\n%s", screen)
		}
	}
	if len(lines) <= barRow || !strings.Contains(lines[barRow], "Library") {
		t.Fatalf("live refresh displaced the header or workspace:\n%s", screen)
	}
	cpuLine := lineWithCPUHeader(strings.Join(lines[:barRow], "\n"))
	if cpuLine == "" || !regexp.MustCompile(`CPU:?\s+\d+%.*\s+\d+MHz\s*$`).MatchString(cpuLine) {
		t.Fatalf("malformed CPU header line: %q\n%s", cpuLine, screen)
	}
	for _, line := range lines {
		if len([]rune(line)) > width {
			t.Fatalf("line wrapped past %d columns: %q\n%s", width, line, screen)
		}
	}
	return strings.Index(cpuLine, "CPU")
}

// sendDisplayed refuses a command unless its key is visible in the current
// rendered screen. Text entry uses typeIntoInput and is not a command shortcut.
func sendDisplayed(t *testing.T, session *Session, key string) {
	t.Helper()
	screen := requireScreen(t, session, nil)
	labels := map[string][]string{
		"enter": {"Enter"}, "tab": {"Tab"}, "up": {"↑", "Arrows"}, "down": {"↓", "Arrows"},
		"left": {"←", "Arrows"}, "right": {"→", "Arrows"}, " ": {"Space", "space"}, "ctrl-s": {"Ctrl+S"},
	}
	visible := false
	if key == "0" {
		visible = regexp.MustCompile(`0[: ]+[Aa]ctions`).MatchString(screen)
	} else if len(key) == 1 && key >= "1" && key <= "4" {
		visible = regexp.MustCompile(`(?:\[` + key + `\]|` + key + `) (?:Library|DLL Catalog|Monitor|Settings)`).MatchString(screen)
	} else {
		for _, label := range labels[key] {
			visible = visible || strings.Contains(screen, label)
		}
	}
	if !visible {
		t.Fatalf("refusing undisplayed key %q:\n%s", key, screen)
	}
	if err := session.SendKeys(key); err != nil {
		t.Fatal(err)
	}
}

func waitForScreen(t *testing.T, session *Session, description string, predicate func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(visibleTimeout)
	var screen string
	for time.Now().Before(deadline) {
		var err error
		screen, err = session.Capture()
		if err != nil {
			t.Fatal(err)
		}
		if predicate(screen) {
			return screen
		}
	}
	t.Fatalf("timed out waiting for %s:\n%s", description, screen)
	return ""
}

func waitVisible(t *testing.T, session *Session, text string) string {
	t.Helper()
	if err := session.WaitForText(text, visibleTimeout); err != nil {
		t.Fatal(err)
	}
	return requireScreen(t, session, []string{text})
}

func selectedAction(screen string) string {
	inside := false
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, "Actions ·") {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if index := strings.Index(line, "▸ "); index >= 0 {
			label := line[index+len("▸ "):]
			if border := strings.Index(label, "│"); border >= 0 {
				label = label[:border]
			}
			return strings.TrimSpace(label)
		}
	}
	return ""
}

func focusAction(t *testing.T, session *Session, label string) {
	t.Helper()
	sendDisplayed(t, session, "0")
	waitVisible(t, session, "Actions ·")
	seen := make(map[string]bool)
	for range 64 {
		screen := requireScreen(t, session, nil)
		selected := selectedAction(screen)
		if selected == label || strings.HasPrefix(selected, label+" (") {
			t.Logf("selected visible action %q:\n%s", label, screen)
			return
		}
		if selected == "" || seen[selected] {
			t.Fatalf("action %q was not reachable through the rendered menu; selected %q:\n%s", label, selected, screen)
		}
		seen[selected] = true
		sendDisplayed(t, session, "down")
		waitForScreen(t, session, "next visible Actions row", func(screen string) bool { return selectedAction(screen) != selected })
	}
	t.Fatalf("action menu did not reach %q", label)
}

func chooseAction(t *testing.T, session *Session, label string) {
	t.Helper()
	focusAction(t, session, label)
	sendDisplayed(t, session, "enter")
	waitForScreen(t, session, "Actions menu to close", func(screen string) bool { return !strings.Contains(screen, "Actions ·") })
}

func quitTUI(t *testing.T, session *Session, environment *TestEnvironment) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(environment.TempDir, "runtime", "spela", "spela.pid")); err != nil {
		t.Fatalf("active TUI has no runtime lock: %v", err)
	}
	chooseAction(t, session, "Quit")
	waitForScreen(t, session, "Spela to release its runtime lock", func(string) bool {
		_, err := os.Stat(filepath.Join(environment.TempDir, "runtime", "spela", "spela.pid"))
		return os.IsNotExist(err)
	})
}

func selectControl(t *testing.T, session *Session, label string) {
	t.Helper()
	for range 8 {
		screen := requireScreen(t, session, []string{label})
		if strings.Contains(screen, "▸ "+label) || strings.Contains(screen, "> "+label) {
			sendDisplayed(t, session, "enter")
			return
		}
		sendDisplayed(t, session, "tab")
		waitForScreen(t, session, "next local control", func(next string) bool { return next != screen })
	}
	t.Fatalf("could not focus visible control %q", label)
}

func requireInputOwnsKeys(t *testing.T, session *Session) string {
	t.Helper()
	screen := requireScreen(t, session, []string{"Input", "Tab", "Enter"})
	if regexp.MustCompile(`0[: ]+[Aa]ctions|(?:\[1\]|1) Library`).MatchString(screen) {
		t.Fatalf("input advertises underlying Browse shortcuts:\n%s", screen)
	}
	return screen
}

func typeIntoInput(t *testing.T, session *Session, text string) {
	t.Helper()
	requireInputOwnsKeys(t, session)
	if err := session.SendText(text); err != nil {
		t.Fatal(err)
	}
}

func requireYAMLValue(t *testing.T, path string, expected any, keys ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	var current any = document
	for _, key := range keys {
		mapping, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s has no mapping for %v: %s", path, keys, data)
		}
		current = mapping[key]
	}
	if !reflect.DeepEqual(current, expected) {
		t.Fatalf("%s %v = %#v, want %#v", path, keys, current, expected)
	}
}

func requireProfileHDR(t *testing.T, path string, expected bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved profile.Profile
	if err := yaml.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Proton.EnableHDR != expected {
		t.Fatalf("%s HDR = %t, want %t", path, saved.Proton.EnableHDR, expected)
	}
}

func openGameProfile(t *testing.T, session *Session) {
	t.Helper()
	sendDisplayed(t, session, "enter")
	waitVisible(t, session, "▸ Detail")
	sendDisplayed(t, session, "right")
	waitVisible(t, session, "[Profile]")
}

func setBooleanEditor(t *testing.T, session *Session, save bool) {
	t.Helper()
	sendDisplayed(t, session, "enter")
	waitVisible(t, session, "Value before edit:")
	requireInputOwnsKeys(t, session)
	sendDisplayed(t, session, " ")
	if save {
		sendDisplayed(t, session, "ctrl-s")
	} else {
		sendDisplayed(t, session, "enter")
	}
}

func TestTUILiveHeaderSuccessiveWidthChanges(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			environment, cleanup := SetupTestEnvironment(t)
			t.Cleanup(cleanup)
			environment.Env = append(environment.Env, "SPELA_E2E_METRICS=successive-widths")
			height := 40
			if width == 80 {
				height = 24
			}
			session := startTUI(t, environment, width, height)

			if err := session.WaitForText("5365MHz", visibleTimeout); err != nil {
				t.Fatal(err)
			}
			first := requireScreen(t, session, []string{"5365MHz"}, "842MHz", "12345MHz")
			// The wide header has aligned columns; compact metrics flow inline.
			cpuColumn := requireCleanLiveHeader(t, first, width)

			if err := session.WaitForText("842MHz", visibleTimeout); err != nil {
				t.Fatal(err)
			}
			second := requireScreen(t, session, []string{"842MHz"}, "5365MHz", "12345MHz")
			if column := requireCleanLiveHeader(t, second, width); width >= 100 && column != cpuColumn {
				t.Fatalf("shorter sample moved CPU column from %d to %d:\n%s", cpuColumn, column, second)
			}

			if err := session.WaitForText("12345MHz", visibleTimeout); err != nil {
				t.Fatal(err)
			}
			third := requireScreen(t, session, []string{"12345MHz"}, "5365MHz", "842MHz")
			if column := requireCleanLiveHeader(t, third, width); width >= 100 && column != cpuColumn {
				t.Fatalf("longer sample moved CPU column from %d to %d:\n%s", cpuColumn, column, third)
			}
		})
	}
}

func TestTUIFreshMinimumLongPathBorderClosure(t *testing.T) {
	environment, cleanup := setupLongPathTestEnvironment(t)
	t.Cleanup(cleanup)
	session := startTUI(t, environment, 80, 24)
	requireScreen(t, session, []string{"Library", "Cyberpunk 2077", "▸ List"}, "▸ Detail")
	sendDisplayed(t, session, "enter")
	screen := waitVisible(t, session, "Install directory")
	requireScreen(t, session, []string{"▸ Detail", "Tab"}, "Ctrl+S", "Enter: Edit")
	requireClosedSplitPaneBorders(t, screen)
	sendDisplayed(t, session, "tab")
	waitVisible(t, session, "▸ List")
}

func TestTUIJourneyStandard(t *testing.T) {
	environment, cleanup := SetupTestEnvironment(t)
	t.Cleanup(cleanup)
	session := startTUI(t, environment, 120, 40)
	requireScreen(t, session, []string{"Library", "▸ List", "Cyberpunk 2077", "Tab"}, "Ctrl+S")

	chooseAction(t, session, "Search games")
	typeIntoInput(t, session, "Cyber")
	sendDisplayed(t, session, "enter")
	waitForScreen(t, session, "filtered Library in Browse", func(screen string) bool {
		return strings.Contains(screen, "Cyberpunk 2077") && !strings.Contains(screen, "Witcher") && !strings.Contains(screen, "Input")
	})
	requireScreen(t, session, []string{"Cyberpunk 2077"}, "Elden Ring")
	openGameProfile(t, session)
	setBooleanEditor(t, session, true)
	waitVisible(t, session, "Profile saved!")
	requireScreen(t, session, []string{"HDR", "false"}, "Unsaved changes", "Edit HDR")
	requireProfileHDR(t, filepath.Join(environment.ConfigHome, "spela", "profiles", "1091500.yaml"), false)
	requireYAMLValue(t, filepath.Join(environment.ConfigHome, "spela", "profiles", "1091500.yaml"), true, "overrides", "proton.enable_hdr")

	chooseAction(t, session, "Help")
	waitVisible(t, session, "Keyboard shortcuts")
	requireScreen(t, session, []string{"Tab", "Close"}, "Esc", "Ctrl+C", "? /", "0:Actions")
	selectControl(t, session, "Close")
	waitVisible(t, session, "[Profile]")

	// Quit through the visible menu before a second process reads the saved fixture.
	quitTUI(t, session, environment)
	restarted := startTUI(t, environment, 120, 40)
	openGameProfile(t, restarted)
	requireScreen(t, restarted, []string{"HDR", "false"}, "Unsaved changes")
}

func TestTUIDefaultDraftReopenResetAndDiscard(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			environment, cleanup := SetupTestEnvironment(t)
			t.Cleanup(cleanup)
			height := 40
			if width == 80 {
				height = 24
			}
			session := startTUI(t, environment, width, height)
			sendDisplayed(t, session, "up")
			waitVisible(t, session, "> All games")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			setBooleanEditor(t, session, false)
			waitVisible(t, session, "Unsaved changes")
			sendDisplayed(t, session, "2")
			waitForScreen(t, session, "DLL Catalog List", func(screen string) bool {
				return strings.Contains(screen, "▸ List") && strings.Contains(screen, "◆ 2 DLL Catalog") && strings.Contains(screen, "> DLSS")
			})
			sendDisplayed(t, session, "1")
			waitVisible(t, session, "▸ List")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			requireScreen(t, session, []string{"Unsaved changes", "HDR", "true"})
			chooseAction(t, session, "Reset field to system default")
			waitForScreen(t, session, "reset root draft", func(screen string) bool {
				return !strings.Contains(screen, "Unsaved changes") && strings.Contains(screen, "HDR")
			})
			setBooleanEditor(t, session, false)
			waitVisible(t, session, "Unsaved changes")
			chooseAction(t, session, "Discard draft")
			waitVisible(t, session, "Enter: Cancel")
			selectControl(t, session, "Discard draft")
			waitForScreen(t, session, "discarded root draft", func(screen string) bool {
				return !strings.Contains(screen, "Unsaved changes") && !strings.Contains(screen, "This changes the draft") && strings.Contains(screen, "HDR")
			})
			requireProfileHDR(t, filepath.Join(environment.ConfigHome, "spela", "profiles", "default.yaml"), false)
		})
	}
}

func TestTUIUnsavedQuitCancelAndSave(t *testing.T) {
	environment, cleanup := SetupTestEnvironment(t)
	t.Cleanup(cleanup)
	session := startTUI(t, environment, 80, 24)
	sendDisplayed(t, session, "up")
	waitVisible(t, session, "> All games")
	sendDisplayed(t, session, "enter")
	waitVisible(t, session, "▸ Detail")
	setBooleanEditor(t, session, false)
	waitVisible(t, session, "Unsaved changes")
	chooseAction(t, session, "Quit")
	waitVisible(t, session, "Save and continue")
	requireScreen(t, session, []string{"▸ Cancel", "Enter: Cancel", "Discard and continue"}, "0: Actions", "1 Library")
	sendDisplayed(t, session, "enter")
	waitForScreen(t, session, "cancelled Quit and preserved root draft", func(screen string) bool {
		return strings.Contains(screen, "HDR") && strings.Contains(screen, "Unsaved changes") && !strings.Contains(screen, "Save and continue")
	})
	requireProfileHDR(t, filepath.Join(environment.ConfigHome, "spela", "profiles", "default.yaml"), false)
	chooseAction(t, session, "Quit")
	waitVisible(t, session, "Save and continue")
	selectControl(t, session, "Save and continue")
	waitForScreen(t, session, "saved Quit to release its runtime lock", func(string) bool {
		_, err := os.Stat(filepath.Join(environment.TempDir, "runtime", "spela", "spela.pid"))
		return os.IsNotExist(err)
	})
	requireProfileHDR(t, filepath.Join(environment.ConfigHome, "spela", "profiles", "default.yaml"), true)
	restarted := startTUI(t, environment, 80, 24)
	sendDisplayed(t, restarted, "up")
	waitVisible(t, restarted, "> All games")
	sendDisplayed(t, restarted, "enter")
	waitVisible(t, restarted, "▸ Detail")
	requireScreen(t, restarted, []string{"HDR", "true"}, "Unsaved changes")
}

func TestTUISettingsEditorSaveAndHints(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			environment, cleanup := SetupTestEnvironment(t)
			t.Cleanup(cleanup)
			height := 40
			if width == 80 {
				height = 24
			}
			session := startTUI(t, environment, width, height)
			sendDisplayed(t, session, "4")
			waitVisible(t, session, "Show hints")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			setBooleanEditor(t, session, true)
			waitVisible(t, session, "saved")
			requireYAMLValue(t, filepath.Join(environment.ConfigHome, "spela", "config.yaml"), false, "show_hints")
			chooseAction(t, session, "Help")
			waitVisible(t, session, "Keyboard shortcuts")
			selectControl(t, session, "Close")
			waitVisible(t, session, "Show hints")
			sendDisplayed(t, session, "tab")
			waitVisible(t, session, "▸ List")
			for group := 0; group < 10; group++ {
				screen := requireScreen(t, session, nil)
				if strings.Contains(screen, "Steam path") {
					break
				}
				sendDisplayed(t, session, "right")
				waitForScreen(t, session, "next Settings group", func(next string) bool { return next != screen })
			}
			waitVisible(t, session, "Steam path")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Input")
			steamPath := filepath.Join(environment.TempDir, "steam 01234 [ö]")
			if err := os.MkdirAll(steamPath, 0o755); err != nil {
				t.Fatal(err)
			}
			typeIntoInput(t, session, steamPath)
			waitVisible(t, session, "01234 [ö]")
			requireInputOwnsKeys(t, session)
			sendDisplayed(t, session, "ctrl-s")
			waitVisible(t, session, "saved")
			requireYAMLValue(t, filepath.Join(environment.ConfigHome, "spela", "config.yaml"), steamPath, "steam_path")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Input")
			typeIntoInput(t, session, " cancelled")
			selectControl(t, session, "Cancel")
			waitForScreen(t, session, "cancelled Settings field", func(screen string) bool {
				return !strings.Contains(screen, "> Input") && !strings.Contains(screen, "Value before edit:")
			})
			requireYAMLValue(t, filepath.Join(environment.ConfigHome, "spela", "config.yaml"), steamPath, "steam_path")
			quitTUI(t, session, environment)
			restarted := startTUI(t, environment, width, height)
			chooseAction(t, restarted, "Help")
			waitVisible(t, restarted, "Keyboard shortcuts")
			selectControl(t, restarted, "Close")
		})
	}
}

func TestTUIReadOnlyDestinations(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			environment, cleanup := SetupTestEnvironment(t)
			t.Cleanup(cleanup)
			height := 40
			if width == 80 {
				height = 24
			}
			session := startTUI(t, environment, width, height)
			sendDisplayed(t, session, "3")
			waitVisible(t, session, "▸ GPU")
			requireScreen(t, session, []string{"▸ List"}, "▸ CPU", "▸ Alerts")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			requireScreen(t, session, []string{"GPU", "Tab"}, "Ctrl+S", "Enter: Edit field")
			sendDisplayed(t, session, "tab")
			waitVisible(t, session, "▸ List")
			sendDisplayed(t, session, "down")
			waitVisible(t, session, "▸ CPU")
			requireScreen(t, session, []string{"▸ List"}, "▸ GPU", "▸ Alerts")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "▸ Detail")
			requireScreen(t, session, []string{"CPU"}, "Ctrl+S", "Enter: Edit field")
			sendDisplayed(t, session, "2")
			waitVisible(t, session, "▸ List")
			sendDisplayed(t, session, "right")
			waitVisible(t, session, "Deployment")
			focusAction(t, session, "Update all stale deployments")
			requireScreen(t, session, []string{"no stale cached deployments", "Close", "Tab"}, "Enter", "Ctrl+S")
			selectControl(t, session, "Close")
			waitVisible(t, session, "Deployment")
		})
	}
}

func libraryListText(screen string) string {
	var rows []string
	for _, line := range strings.Split(screen, "\n") {
		cells := strings.Split(line, "│")
		if len(cells) > 1 {
			rows = append(rows, cells[1])
		}
	}
	return strings.Join(rows, "\n")
}

func TestTUIFiltersSortAndVisibleSelection(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			environment, cleanup := SetupTestEnvironment(t)
			t.Cleanup(cleanup)
			height := 40
			if width == 80 {
				height = 24
			}
			session := startTUI(t, environment, width, height)
			sendDisplayed(t, session, " ")
			waitVisible(t, session, "Selected: 1 total, 1 visible")
			requireScreen(t, session, []string{"Enter: Batch actions"}, "Enter: Open selection")
			chooseAction(t, session, "Search games")
			typeIntoInput(t, session, "Elden")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Selected: 1 total, 0 visible")
			chooseAction(t, session, "Select all filtered games")
			waitVisible(t, session, "Selected: 2 total, 1 visible")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "No available updates")
			emptyBatch := requireScreen(t, session, []string{"Batch action (1 visible games)", "Close", "Tab"}, "Enter:")
			t.Logf("Batch with no targets:\n%s", emptyBatch)
			sendDisplayed(t, session, "tab")
			waitVisible(t, session, "Enter: close")
			sendDisplayed(t, session, "enter")
			waitVisible(t, session, "Selected: 2 total, 1 visible")
			chooseAction(t, session, "Clear selected filtered games")
			waitVisible(t, session, "Selected: 1 total, 0 visible")
			chooseAction(t, session, "Clear filters (reset sort to A-Z)")
			waitVisible(t, session, "Selected: 1 total, 1 visible")
			chooseAction(t, session, "Clear selected filtered games")
			waitForScreen(t, session, "cleared selection", func(screen string) bool { return !strings.Contains(screen, "Selected:") })
			chooseAction(t, session, "Sort: Z-A")
			screen := waitVisible(t, session, "Scope [Z-A]")
			list := libraryListText(screen)
			if witcher, elden, cyberpunk := strings.Index(list, "Witcher"), strings.Index(list, "Elden"), strings.Index(list, "Cyberpunk"); witcher < 0 || elden <= witcher || cyberpunk <= elden {
				t.Fatalf("Z-A sort did not order the visible games:\n%s", screen)
			}
			chooseAction(t, session, "Toggle DLL filter")
			waitForScreen(t, session, "DLL-filtered Library", func(screen string) bool {
				list := libraryListText(screen)
				return strings.Contains(list, "●DLLs") && !strings.Contains(list, "Elden")
			})
			chooseAction(t, session, "Toggle profile filter")
			waitForScreen(t, session, "profile-filtered Library", func(screen string) bool {
				list := libraryListText(screen)
				return strings.Contains(list, "◆Profile") && strings.Contains(list, "Cyberpunk") && !strings.Contains(list, "Witcher")
			})
			chooseAction(t, session, "Clear filters (reset sort to A-Z)")
			screen = waitForScreen(t, session, "cleared filters and default sort", func(screen string) bool {
				list := libraryListText(screen)
				return strings.Contains(list, "Elden") && strings.Contains(list, "Witcher") && !strings.Contains(list, "[Z-A]") && !strings.Contains(list, "◆Profile")
			})
			list = libraryListText(screen)
			if cyberpunk, elden, witcher := strings.Index(list, "Cyberpunk"), strings.Index(list, "Elden"), strings.Index(list, "Witcher"); cyberpunk < 0 || elden <= cyberpunk || witcher <= elden {
				t.Fatalf("clear filters did not restore A-Z order:\n%s", screen)
			}
		})
	}
}

func TestTUIEmptyLibraryRescanRecovery(t *testing.T) {
	environment, cleanup := SetupTestEnvironment(t)
	t.Cleanup(cleanup)
	databasePath := filepath.Join(environment.DataHome, "spela", "games.yaml")
	writeYAML(t, databasePath, map[string]any{"games": map[string]any{}})
	steamPath := filepath.Join(environment.TempDir, "home", ".steam", "steam")
	libraryFolders := "\"libraryfolders\"\n{\n  \"0\"\n  {\n    \"path\" \"" + steamPath + "\"\n    \"apps\"\n    {\n    }\n  }\n}\n"
	writeDummyFile(t, filepath.Join(steamPath, "steamapps", "libraryfolders.vdf"), libraryFolders)
	session := startTUI(t, environment, 80, 24)
	waitVisible(t, session, "No games found")
	waitVisible(t, session, "Rescan complete: 0 games found")
	requireScreen(t, session, []string{"Actions", "Settings for paths"}, "Enter: Open selection")
	chooseAction(t, session, "Rescan games")
	waitVisible(t, session, "Rescan complete: 0 games found")

	installPath := filepath.Join(steamPath, "steamapps", "common", "Fixture scan game")
	if err := os.MkdirAll(installPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "\"AppState\"\n{\n  \"appid\" \"4567890\"\n  \"name\" \"Fixture scan game\"\n  \"installdir\" \"Fixture scan game\"\n  \"StateFlags\" \"4\"\n}\n"
	writeDummyFile(t, filepath.Join(steamPath, "steamapps", "appmanifest_4567890.acf"), manifest)
	chooseAction(t, session, "Rescan games")
	waitVisible(t, session, "Fixture scan game")
	requireScreen(t, session, []string{"Enter: Open selection"}, "No games found")
	data, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), installPath) {
		t.Fatalf("rescan did not persist the temporary game path: %s", data)
	}
}

func TestTUIJourneysConstrained(t *testing.T) {
	t.Run("DLL confirmation defaults to Cancel", func(t *testing.T) {
		environment, cleanup := SetupTestEnvironment(t)
		t.Cleanup(cleanup)
		session := startTUI(t, environment, 80, 24)
		openGameProfile(t, session)
		sendDisplayed(t, session, "right")
		waitVisible(t, session, "DLL versions")
		requireScreen(t, session, []string{"DLLs", "▸ Detail"}, "Ctrl+S", "Enter: Edit field")
		dllPath := filepath.Join(environment.TempDir, "games", "Cyberpunk 2077", "bin", "x64", "nvngx_dlss.dll")
		before, err := os.ReadFile(dllPath)
		if err != nil {
			t.Fatal(err)
		}
		chooseAction(t, session, "Install DLL: Cyberpunk 2077")
		waitVisible(t, session, "Select a supported DLL family")
		sendDisplayed(t, session, "enter")
		waitVisible(t, session, "Select DLSS version")
		sendDisplayed(t, session, "enter")
		waitVisible(t, session, "Confirm DLL install")
		requireScreen(t, session, []string{"Cancel", "Confirm", "Enter", "Tab"}, "Esc", "Enter/Y")
		sendDisplayed(t, session, "tab")
		waitVisible(t, session, "Tab Buttons")
		for range 20 {
			screen := requireScreen(t, session, nil)
			if strings.Contains(screen, "Backup:") {
				break
			}
			sendDisplayed(t, session, "down")
			waitForScreen(t, session, "confirmation details to scroll", func(next string) bool { return next != screen })
		}
		waitVisible(t, session, "Backup:")
		sendDisplayed(t, session, "tab")
		waitVisible(t, session, "Activate")
		sendDisplayed(t, session, "enter")
		waitVisible(t, session, "cancelled, no operation ran")
		after, err := os.ReadFile(dllPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Fatal("cancelled DLL journey changed the fixture")
		}
	})

	t.Run("empty search and recovery", func(t *testing.T) {
		environment, cleanup := SetupTestEnvironment(t)
		t.Cleanup(cleanup)
		session := startTUI(t, environment, 80, 24)
		chooseAction(t, session, "Search games")
		typeIntoInput(t, session, "missing 01234 [/] title")
		waitVisible(t, session, "No games match")
		requireInputOwnsKeys(t, session)
		selectControl(t, session, "Cancel")
		waitVisible(t, session, "Cyberpunk 2077")
		chooseAction(t, session, "Search games")
		typeIntoInput(t, session, "missing title")
		sendDisplayed(t, session, "enter")
		waitVisible(t, session, "No games match")
		chooseAction(t, session, "Clear filters (reset sort to A-Z)")
		waitVisible(t, session, "Cyberpunk 2077")
	})

	t.Run("startup error", func(t *testing.T) {
		environment, cleanup := SetupTestEnvironment(t)
		t.Cleanup(cleanup)
		if err := os.WriteFile(filepath.Join(environment.DataHome, "spela", "games.yaml"), []byte("games: [invalid"), 0o644); err != nil {
			t.Fatal(err)
		}
		session, err := NewSession("spela-tui-e2e-error", 80, 24, testBinaryPath, []string{"tui"}, environment.Env, environment.TerminalControlRuntimeDirectory)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if screen, err := session.Capture(); err == nil {
				t.Logf("final startup-error screen:\n%s", screen)
			}
			if err := session.Close(); err != nil {
				t.Errorf("cleanup Terminal Control session: %v", err)
			}
		})
		waitVisible(t, session, "failed")
	})
}
