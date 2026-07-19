//go:build dev || production || bindings

package gui

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
)

func TestGUIBoundaryProfilePassUsesGameProfile(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadProfile = func(appID uint64) (*profile.Profile, error) {
		if appID != 1091500 {
			t.Fatalf("unexpected appID: %d", appID)
		}
		p := &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}
		p.MarkOverride(profile.FieldProtonEnableHDR)
		return p, nil
	}
	boundary.loadDefaultProfile = func() (*profile.Profile, error) {
		return nil, nil
	}

	info := boundary.profileView(appID(1091500))
	if info == nil {
		t.Fatal("expected profile info")
	}
	if info["enableHdr"] != true || info["inheritedFromDefault"] != false {
		t.Fatalf("expected game profile HDR override, got %+v", info)
	}
}

func TestGUIBoundaryProfileFailReturnsNil(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadProfile = func(uint64) (*profile.Profile, error) {
		return nil, errors.New("profile read failed")
	}

	if info := boundary.profileView(appID(1091500)); info != nil {
		t.Fatalf("expected nil profile on load failure, got %+v", info)
	}
}

func TestGUIBoundaryProfileSemanticsPassResolvesInheritedValues(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	gameProfile := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: false}}
	gameProfile.MarkOverride(profile.FieldProtonVKD3DHeap)
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return gameProfile, nil }
	boundary.loadDefaultProfile = func() (*profile.Profile, error) {
		return &profile.Profile{
			Proton: profile.ProtonSettings{EnableHDR: true, VKD3DHeap: true},
			GPU:    profile.GPUSettings{ClockOffset: 100},
		}, nil
	}

	info := boundary.profileView(appID(1091500))
	if info == nil {
		t.Fatal("expected profile info")
	}
	if info["enableHdr"] != true || info["vkd3dHeap"] != false {
		t.Fatalf("expected resolved HDR default and VKD3D override, got %+v", info)
	}
	semantics := semanticsByField(info["semantics"].([]ProfileFieldSemantics))
	assertProfileSemantic(t, semantics[profile.FieldProtonEnableHDR], "default", "compatibility", "ephemeral_launch_environment")
	assertProfileSemantic(t, semantics[profile.FieldProtonVKD3DHeap], "override", "compatibility", "ephemeral_launch_environment")
	assertProfileSemantic(t, semantics[profile.FieldGPUClockOffset], "default", "system_state", "restorable_mutation")

	boundary.loadDefaultProfile = func() (*profile.Profile, error) {
		return &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: false, VKD3DHeap: true}}, nil
	}
	reloaded := boundary.profileView(appID(1091500))
	if reloaded == nil || reloaded["enableHdr"] != false {
		t.Fatalf("expected live inherited HDR default to reload as false, got %+v", reloaded)
	}
	assertProfileSemantic(t, semanticsByField(reloaded["semantics"].([]ProfileFieldSemantics))[profile.FieldProtonEnableHDR], "default", "compatibility", "ephemeral_launch_environment")
}

func TestGUIBoundaryDefaultProfileSemanticsPass(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadDefaultProfile = func() (*profile.Profile, error) {
		return &profile.Profile{
			Proton: profile.ProtonSettings{EnableHDR: true, VKD3DHeap: true},
			GPU:    profile.GPUSettings{ClockOffset: 100},
		}, nil
	}

	info := boundary.profileView(nil)
	if info == nil {
		t.Fatal("expected default profile info")
	}
	items := info["semantics"].([]ProfileFieldSemantics)
	if len(items) == 0 {
		t.Fatal("expected default profile semantics")
	}
	semantics := semanticsByField(items)
	assertProfileSemantic(t, semantics[profile.FieldProtonEnableHDR], "default", "compatibility", "ephemeral_launch_environment")
	assertProfileSemantic(t, semantics[profile.FieldGPUClockOffset], "default", "system_state", "restorable_mutation")
}

func TestGUIBoundaryOneFieldPatchPreservesHiddenOverrides(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	defaults := &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true, VKD3DHeap: true}}
	current := &profile.Profile{
		Proton:  profile.ProtonSettings{VKD3DHeap: false},
		GPU:     profile.GPUSettings{PowerLimit: 275},
		Overlay: profile.OverlaySettings{Enabled: true},
	}
	current.MarkOverride(profile.FieldProtonVKD3DHeap)
	current.MarkOverride(profile.FieldGPUPowerLimit)
	current.MarkOverride(profile.FieldOverlayEnabled)
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return current, nil }
	boundary.loadDefaultProfile = func() (*profile.Profile, error) { return defaults, nil }
	var saved *profile.Profile
	boundary.mutateProfile = func(appID uint64, callback func(*profile.Profile, *profile.Profile) error) error {
		if appID != 1091500 {
			t.Fatalf("unexpected appID: %d", appID)
		}
		saved = current.Clone()
		if err := callback(saved, defaults); err != nil {
			return err
		}
		return nil
	}

	if err := boundary.patchGameProfile(1091500, []ProfilePatch{{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: false}}); err != nil {
		t.Fatal(err)
	}
	if saved == nil {
		t.Fatal("expected saved profile")
	}
	if !saved.IsOverridden(profile.FieldProtonEnableHDR) || !saved.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Fatalf("expected changed HDR and existing VKD3D overrides, got %+v", saved.Overrides)
	}
	if saved.IsOverridden(profile.FieldGPUClockOffset) {
		t.Fatalf("unchanged inherited clock offset should not become an override: %+v", saved.Overrides)
	}
	if !saved.IsOverridden(profile.FieldGPUPowerLimit) || saved.GPU.PowerLimit != 275 {
		t.Fatalf("hidden power limit changed: value=%d overrides=%+v", saved.GPU.PowerLimit, saved.Overrides)
	}
	if !saved.IsOverridden(profile.FieldOverlayEnabled) {
		t.Fatalf("unrendered existing overlay override should be preserved: %+v", saved.Overrides)
	}
	resolved := saved.ResolveForApply(defaults)
	if resolved.Proton.EnableHDR || resolved.Proton.VKD3DHeap {
		t.Fatalf("expected saved effective false/false, got HDR=%v VKD3D=%v", resolved.Proton.EnableHDR, resolved.Proton.VKD3DHeap)
	}
	if !resolved.Overlay.Enabled {
		t.Fatal("expected unrendered overlay override to remain effective")
	}
}

func TestGUIProfilePatchSetPinResetAndErrors(t *testing.T) {
	current := &profile.Profile{}
	defaults := &profile.Profile{GPU: profile.GPUSettings{ClockOffset: 100}}
	if err := applyProfilePatch(current, defaults, ProfilePatch{Field: profile.FieldGPUClockOffset, Operation: "pin"}, false); err != nil {
		t.Fatal(err)
	}
	if current.GPU.ClockOffset != 100 || !current.IsOverridden(profile.FieldGPUClockOffset) {
		t.Fatalf("pin = %+v", current)
	}
	if err := applyProfilePatch(current, defaults, ProfilePatch{Field: profile.FieldGPUClockOffset, Operation: "set", Value: float64(0)}, false); err != nil {
		t.Fatal(err)
	}
	if current.GPU.ClockOffset != 0 || !current.IsOverridden(profile.FieldGPUClockOffset) {
		t.Fatal("explicit zero set changed")
	}
	if err := applyProfilePatch(current, defaults, ProfilePatch{Field: profile.FieldGPUClockOffset, Operation: "reset"}, false); err != nil {
		t.Fatal(err)
	}
	if current.IsOverridden(profile.FieldGPUClockOffset) {
		t.Fatal("reset retained override")
	}
	for _, patch := range []ProfilePatch{{Field: "unknown", Operation: "set"}, {Field: profile.FieldGPUClockOffset, Operation: "unknown"}} {
		if err := applyProfilePatch(current, defaults, patch, false); err == nil {
			t.Fatalf("patch %+v succeeded", patch)
		}
	}
	if err := applyProfilePatch(current, defaults, ProfilePatch{Field: profile.FieldGPUClockOffset, Operation: "pin"}, true); err == nil {
		t.Fatal("root pin succeeded")
	}
}

func TestGUIProfilePatchBatchIsAtomicOnLateFailure(t *testing.T) {
	original := &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}
	original.MarkOverride(profile.FieldProtonEnableHDR)
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return original, nil }
	boundary.loadDefaultProfile = func() (*profile.Profile, error) { return &profile.Profile{}, nil }
	saves := 0
	boundary.mutateProfile = func(_ uint64, callback func(*profile.Profile, *profile.Profile) error) error {
		current := original.Clone()
		if err := callback(current, &profile.Profile{}); err != nil {
			return err
		}
		saves++
		return nil
	}
	err := boundary.patchGameProfile(1, []ProfilePatch{
		{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: false},
		{Field: profile.FieldDLSSSRMode, Operation: "set", Value: "invalid"},
	})
	if err == nil {
		t.Fatal("invalid late patch succeeded")
	}
	if saves != 0 {
		t.Fatalf("atomic failure saved %d times", saves)
	}
	if !original.Proton.EnableHDR || !original.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatalf("atomic failure mutated loaded profile: %+v", original)
	}
}

func TestGUIProfilePatchBatchSavesAllFieldsOnce(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadProfile = func(uint64) (*profile.Profile, error) { return &profile.Profile{}, nil }
	boundary.loadDefaultProfile = func() (*profile.Profile, error) { return &profile.Profile{}, nil }
	saves := 0
	var saved *profile.Profile
	boundary.mutateProfile = func(_ uint64, callback func(*profile.Profile, *profile.Profile) error) error {
		value := &profile.Profile{}
		if err := callback(value, &profile.Profile{}); err != nil {
			return err
		}
		saves++
		saved = value
		return nil
	}
	if err := boundary.patchGameProfile(1, []ProfilePatch{
		{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: false},
		{Field: profile.FieldGPUClockOffset, Operation: "set", Value: float64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	if saves != 1 || saved == nil || !saved.IsOverridden(profile.FieldProtonEnableHDR) || !saved.IsOverridden(profile.FieldGPUClockOffset) {
		t.Fatalf("batch save = count %d, profile %+v", saves, saved)
	}
}

func semanticsByField(items []ProfileFieldSemantics) map[string]ProfileFieldSemantics {
	out := make(map[string]ProfileFieldSemantics, len(items))
	for _, item := range items {
		out[item.Field] = item
	}
	return out
}

func appID(value uint64) *uint64 { return &value }

func assertProfileSemantic(t *testing.T, item ProfileFieldSemantics, source, impact, restore string) {
	t.Helper()
	if item.Source != source || item.Impact != impact || item.Restore != restore {
		t.Fatalf("expected source=%s impact=%s restore=%s, got %+v", source, impact, restore, item)
	}
}

func TestGUIBoundaryCompatibilityPassUsesDomainNotice(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadConfig = func() (*config.Config, error) {
		return &config.Config{SteamPath: "/steam/root"}, nil
	}
	boundary.compatibilityNotice = func(appID uint64, deps proton.NoticeDeps) string {
		if appID != 1091500 {
			t.Fatalf("unexpected appID: %d", appID)
		}
		if deps.SteamRoot != "/steam/root" {
			t.Fatalf("expected configured Steam root, got %q", deps.SteamRoot)
		}
		return "⚠ descriptor_heap requires Proton-CachyOS 9.0+"
	}

	got := boundary.vkd3dHeapCompatibilityNotice(1091500)
	if !strings.Contains(got, "descriptor_heap requires") {
		t.Fatalf("expected domain compatibility notice, got %q", got)
	}
}

func TestGUIBoundaryLaunchFailExplainsSteamLaunchOptions(t *testing.T) {
	app := &App{database: &game.Database{Games: map[uint64]*game.Game{
		1091500: {AppID: 1091500, Name: "Cyberpunk 2077"},
	}}}

	err := app.LaunchGame(1091500)
	if err == nil {
		t.Fatal("expected direct launch rejection")
	}
	message := err.Error()
	if !strings.Contains(message, "cannot track the game lifetime") || !strings.Contains(message, "spela %command%") {
		t.Fatalf("expected Steam launch-option guidance, got %q", message)
	}
}

func TestGUILoggingPassUsesCentralizedHandler(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	gamesPath := filepath.Join(dataHome, "spela", "games.yaml")
	if err := os.MkdirAll(gamesPath, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	restore := logging.SetHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(restore)

	app := NewApp()
	app.startup(context.Background())

	logs := buf.String()
	if !strings.Contains(logs, "failed to load game database") || !strings.Contains(logs, "level=ERROR") {
		t.Fatalf("expected GUI startup log through centralized handler, got %q", logs)
	}
}
