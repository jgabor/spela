//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Bubble Tea uses CSI Z when updating a live screen. A terminal emulator that
// ignores it creates stale metric suffixes even when the application frame is
// correct. Keep the compatibility check next to the regression it enables.
func TestTerminalControlCursorBackwardTabCompatibility(t *testing.T) {
	command := exec.Command("termctrl", "show", "--cols", "100", "--rows", "4", "--input", "-")
	command.Stdin = strings.NewReader("\x1b[2J\x1b[2;87H\x1b[ZB\x1b[3;87H\x1b[1ZC")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Terminal Control cursor compatibility check: %v: %s", err, output)
	}
	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	if len(lines) != 3 || lines[1] != strings.Repeat(" ", 80)+"B" || lines[2] != strings.Repeat(" ", 80)+"C" {
		t.Fatalf("termctrl ignored cursor backward tab (CSI Z); use a compatible Terminal Control build (verified with 1.2.1, 0.3.1 is incompatible):\n%s", output)
	}
}

// This deliberately resizes one live session. Independent fresh sessions and
// changes in metric string length do not exercise terminal resize invalidation.
func TestTUIRepeatedLiveResizePreservesFrames(t *testing.T) {
	environment, cleanup := SetupTestEnvironment(t)
	t.Cleanup(cleanup)
	environment.Env = append(environment.Env, "SPELA_E2E_METRICS=successive-widths")
	session := startTUI(t, environment, 120, 40)
	if directory := os.Getenv("SPELA_E2E_RESIZE_ARTIFACTS"); directory != "" {
		t.Cleanup(func() {
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Error(err)
				return
			}
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
		})
	}
	if err := session.WaitForText("5365MHz", visibleTimeout); err != nil {
		t.Fatal(err)
	}

	phases := []struct {
		width, height int
		destination   string
	}{
		{120, 40, "1"},
		{80, 24, "4"},
		{120, 40, "3"},
		{72, 20, ""},
		{120, 40, "2"},
		{80, 24, "1"},
		{100, 32, "3"},
		{80, 24, "4"},
		{72, 20, ""},
		{80, 24, "2"},
		{120, 40, "1"},
	}
	for index, phase := range phases {
		if err := session.Resize(phase.width, phase.height); err != nil {
			t.Fatal(err)
		}
		if phase.destination == "" {
			if err := session.WaitForText(fmt.Sprintf("Current size: %dx%d", phase.width, phase.height), visibleTimeout); err != nil {
				t.Fatal(err)
			}
			screen := requireScreen(t, session, []string{"Resize terminal"}, "Library", "MHz")
			t.Logf("resize %d to %dx%d, suspended workspace:\n%s", index, phase.width, phase.height, screen)
			continue
		}
		if err := session.WaitForText("Library", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		// Inspect the current frame before using its displayed destination key.
		sendDisplayed(t, session, phase.destination)
		screen, err := session.Capture()
		if err != nil {
			t.Fatal(err)
		}
		// Wait for a new visible sample, so the assertion includes an incremental
		// repaint after the resize, not merely the first full redraw.
		nextFrequency := "5365MHz"
		if strings.Contains(screen, "5365MHz") {
			nextFrequency = "842MHz"
		} else if strings.Contains(screen, "842MHz") {
			nextFrequency = "12345MHz"
		}
		if err := session.WaitForText(nextFrequency, visibleTimeout); err != nil {
			t.Fatal(err)
		}
		screen = requireScreen(t, session, []string{"Library", "DLL Catalog", "Monitor", "Settings", nextFrequency}, "Resize terminal")
		requireUncorruptedResizeFrame(t, screen, phase.width, phase.height)
		t.Logf("resize %d to %dx%d, destination %s:\n%s", index, phase.width, phase.height, phase.destination, screen)
	}
}

func requireUncorruptedResizeFrame(t *testing.T, screen string, width, height int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(screen, "\n"), "\n")
	if len(lines) > height {
		t.Fatalf("rendered %d rows after resize to %dx%d:\n%s", len(lines), width, height, screen)
	}
	for _, line := range lines {
		if len([]rune(line)) > width {
			t.Fatalf("line exceeds %d columns after resize: %q\n%s", width, line, screen)
		}
	}
	headerRows := min(6, len(lines))
	header := strings.Join(lines[:headerRows], "\n")
	frequencies := regexp.MustCompile(`\b(?:5365|842|12345)MHz\b`).FindAllString(header, -1)
	if len(frequencies) != 1 {
		t.Fatalf("header has missing, duplicated, or corrupted frequency after resize:\n%s", screen)
	}
	if !regexp.MustCompile(`CPU:?\s+\d+%.*\s+(?:5365|842|12345)MHz\s*$`).MatchString(lineWithCPUHeader(header)) {
		t.Fatalf("CPU line is malformed after resize:\n%s", screen)
	}
	if frequencies[0] != "5365MHz" && strings.Contains(header, "Power limit") {
		t.Fatalf("cleared metric alert remains after resize:\n%s", screen)
	}
	if strings.Contains(header, "GPU:") && width >= 100 {
		for _, label := range []string{"VRAM:", "RAM:"} {
			line := ""
			for _, candidate := range lines[:headerRows] {
				if strings.Contains(candidate, " "+label) {
					line = candidate
					break
				}
			}
			if !regexp.MustCompile(`\d+\.\d+/\d+\.\d+ GB\s*$`).MatchString(line) {
				t.Fatalf("%s line retained stale cells after resize: %q\n%s", label, line, screen)
			}
		}
	}
	if len(regexp.MustCompile(`(?:\[1\]|1) Library`).FindAllString(screen, -1)) != 1 {
		t.Fatalf("destination bar is missing or duplicated after resize:\n%s", screen)
	}
}

func lineWithCPUHeader(screen string) string {
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, "CPU:") || strings.Contains(line, "CPU ") {
			return line
		}
	}
	return ""
}
