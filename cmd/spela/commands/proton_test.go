package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

// withTempXDG redirects XDG_CONFIG_HOME / XDG_DATA_HOME into a temp dir
// and restores the original env on cleanup. Returns the temp dir root.
func withTempXDG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	return dir
}

// seedGame writes a tiny games.yaml with the given name+appid so
// game.LoadDatabase() can find it during runProtonSet.
func seedGame(t *testing.T, name string, appID uint64) {
	t.Helper()
	db := &game.Database{
		Games: map[uint64]*game.Game{
			appID: {AppID: appID, Name: name},
		},
	}
	if err := db.Save(); err != nil {
		t.Fatalf("save database: %v", err)
	}
}

// TestRunProtonSet_VKD3DHeap_Persists covers the happy path for the CLI
// flag handler: `--vkd3d-heap=true` writes `vkd3d_heap: true` into the
// profile YAML on disk.
func TestRunProtonSet_VKD3DHeap_Persists(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	// Reset shared state that init() wired up at test-binary load.
	protonSetHDR = ""
	protonSetWayland = ""
	protonSetNGXUpdater = ""
	protonSetVKD3DHeap = "true"
	t.Cleanup(func() { protonSetVKD3DHeap = "" })

	if err := runProtonSet(protonSetCmd, []string{"1091500"}); err != nil {
		t.Fatalf("runProtonSet: %v", err)
	}

	p, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("profile.Load: %v", err)
	}
	if p == nil {
		t.Fatal("expected profile to exist after set")
	}
	if !p.Proton.VKD3DHeap {
		t.Errorf("expected VKD3DHeap=true, got false")
	}
}

// TestRunProtonSet_VKD3DHeap_InvalidValue covers the failure path for
// the CLI flag handler: a non-boolean value must surface a clear error
// rather than silently mapping to false.
func TestRunProtonSet_VKD3DHeap_InvalidValue(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	protonSetHDR = ""
	protonSetWayland = ""
	protonSetNGXUpdater = ""
	protonSetVKD3DHeap = "maybe"
	t.Cleanup(func() { protonSetVKD3DHeap = "" })

	err := runProtonSet(protonSetCmd, []string{"1091500"})
	if err == nil {
		t.Fatal("expected error for invalid --vkd3d-heap value, got nil")
	}
	if !strings.Contains(err.Error(), "vkd3d-heap") {
		t.Errorf("expected error to mention flag, got %q", err.Error())
	}

	// Ensure the profile was not persisted with a bogus value.
	if p, _ := profile.Load(1091500); p != nil && p.Proton.VKD3DHeap {
		t.Error("profile should not have been written on invalid value")
	}
}

// TestRunProtonShow_RendersNoticeWhenIncompatible verifies that the CLI
// show path threads the compatibility notice through to stdout when
// vkd3d_heap is enabled and the injected notice source reports a problem.
func TestRunProtonShow_RendersNoticeWhenIncompatible(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	// Seed a profile with VKD3DHeap=true so runProtonShow calls the notice
	// helper.
	p := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	if err := profile.Save(1091500, p); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	// Swap the notice source. Restore on cleanup.
	originalNotice := protonCompatibilityNotice
	protonCompatibilityNotice = func(appID uint64) string {
		return "⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: 570.86.0)"
	}
	t.Cleanup(func() { protonCompatibilityNotice = originalNotice })

	protonShowJSON = false

	out := captureStdout(t, func() {
		if err := runProtonShow(protonShowCmd, []string{"1091500"}); err != nil {
			t.Fatalf("runProtonShow: %v", err)
		}
	})

	if !strings.Contains(out, "VKD3D heap:") {
		t.Errorf("expected output to include VKD3D heap line, got:\n%s", out)
	}
	if !strings.Contains(out, "requires NVIDIA driver 580.94.16") {
		t.Errorf("expected notice in output, got:\n%s", out)
	}
}

// TestRunProtonShow_NoNoticeWhenToggleDisabled verifies that the notice
// helper is not even consulted when vkd3d_heap is false, and thus nothing
// renders under the field.
func TestRunProtonShow_NoNoticeWhenToggleDisabled(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	p := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: false}}
	if err := profile.Save(1091500, p); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	called := false
	originalNotice := protonCompatibilityNotice
	protonCompatibilityNotice = func(appID uint64) string {
		called = true
		return "⚠ should never render"
	}
	t.Cleanup(func() { protonCompatibilityNotice = originalNotice })

	protonShowJSON = false

	out := captureStdout(t, func() {
		if err := runProtonShow(protonShowCmd, []string{"1091500"}); err != nil {
			t.Fatalf("runProtonShow: %v", err)
		}
	})

	if called {
		t.Error("notice source should not be called when VKD3DHeap is false")
	}
	if strings.Contains(out, "should never render") {
		t.Errorf("unexpected notice in output:\n%s", out)
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// whatever was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = orig
	return <-done
}
