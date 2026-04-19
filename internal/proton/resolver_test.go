package proton

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeConfigVDF writes a minimal Steam config.vdf to the fake steam
// root with the given CompatToolMapping entries (appid -> tool name).
// The key "0" represents the global default, per Steam's convention.
func writeConfigVDF(t *testing.T, steamRoot string, mapping map[string]string) {
	t.Helper()
	configDir := filepath.Join(steamRoot, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	var b []byte
	b = append(b, []byte("\"InstallConfigStore\"\n{\n\t\"Software\"\n\t{\n\t\t\"Valve\"\n\t\t{\n\t\t\t\"Steam\"\n\t\t\t{\n\t\t\t\t\"CompatToolMapping\"\n\t\t\t\t{\n")...)
	for appid, name := range mapping {
		b = append(b, []byte("\t\t\t\t\t\""+appid+"\"\n\t\t\t\t\t{\n\t\t\t\t\t\t\"name\"\t\t\""+name+"\"\n\t\t\t\t\t\t\"config\"\t\t\"\"\n\t\t\t\t\t\t\"priority\"\t\t\"250\"\n\t\t\t\t\t}\n")...)
	}
	b = append(b, []byte("\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n")...)
	if err := os.WriteFile(filepath.Join(configDir, "config.vdf"), b, 0o644); err != nil {
		t.Fatalf("write config.vdf: %v", err)
	}
}

// writeProtonBuild creates a fake Proton build directory under either
// compatibilitytools.d (community=true) or steamapps/common (community=false).
// If scriptContents is non-empty it is written as the top-level `proton` script.
func writeProtonBuild(t *testing.T, steamRoot, name string, community bool, scriptContents string) string {
	t.Helper()
	var parent string
	if community {
		parent = filepath.Join(steamRoot, "compatibilitytools.d", name)
	} else {
		parent = filepath.Join(steamRoot, "steamapps", "common", name)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("mkdir build: %v", err)
	}
	if scriptContents != "" {
		if err := os.WriteFile(filepath.Join(parent, "proton"), []byte(scriptContents), 0o755); err != nil {
			t.Fatalf("write proton script: %v", err)
		}
	}
	return parent
}

func TestResolveForAppID_PerGameOverride_Found(t *testing.T) {
	root := t.TempDir()
	writeConfigVDF(t, root, map[string]string{
		"1091500": "GE-Proton10-34", // cyberpunk 2077
		"0":       "proton_experimental",
	})
	wantPath := writeProtonBuild(t, root, "GE-Proton10-34", true, "#!/bin/sh\necho stub\n")

	build, err := ResolveForAppID(root, 1091500)
	if err != nil {
		t.Fatalf("ResolveForAppID: %v", err)
	}
	if build.Name != "GE-Proton10-34" {
		t.Errorf("Name = %q, want %q", build.Name, "GE-Proton10-34")
	}
	if build.Path != wantPath {
		t.Errorf("Path = %q, want %q", build.Path, wantPath)
	}
}

func TestResolveForAppID_GlobalDefault_Found(t *testing.T) {
	root := t.TempDir()
	writeConfigVDF(t, root, map[string]string{
		"0": "cachyos-10.0-20260410-slr",
	})
	wantPath := writeProtonBuild(t, root, "cachyos-10.0-20260410-slr", true, "#!/bin/sh\n")

	build, err := ResolveForAppID(root, 1091500)
	if err != nil {
		t.Fatalf("ResolveForAppID via global default: %v", err)
	}
	if build.Name != "cachyos-10.0-20260410-slr" {
		t.Errorf("Name = %q, want %q", build.Name, "cachyos-10.0-20260410-slr")
	}
	if build.Path != wantPath {
		t.Errorf("Path = %q, want %q", build.Path, wantPath)
	}
}

func TestResolveForAppID_NoMapping_ReturnsSentinel(t *testing.T) {
	root := t.TempDir()
	writeConfigVDF(t, root, map[string]string{
		"999": "some-other-tool",
	})

	_, err := ResolveForAppID(root, 1091500)
	if !errors.Is(err, ErrProtonNotResolved) {
		t.Fatalf("err = %v, want ErrProtonNotResolved", err)
	}
}

func TestResolveForAppID_MappingPresentButDirMissing_ReturnsSentinel(t *testing.T) {
	root := t.TempDir()
	writeConfigVDF(t, root, map[string]string{
		"1091500": "GE-Proton99-99", // referenced but never installed
	})

	_, err := ResolveForAppID(root, 1091500)
	if !errors.Is(err, ErrProtonNotResolved) {
		t.Fatalf("err = %v, want ErrProtonNotResolved", err)
	}
}

func TestResolveForAppID_BuiltInCommonFallback(t *testing.T) {
	// Built-in Proton (e.g. Proton Hotfix) sits in steamapps/common, not
	// compatibilitytools.d. Resolver must check both locations.
	root := t.TempDir()
	writeConfigVDF(t, root, map[string]string{
		"1091500": "Proton Hotfix",
	})
	wantPath := writeProtonBuild(t, root, "Proton Hotfix", false, "#!/bin/sh\n")

	build, err := ResolveForAppID(root, 1091500)
	if err != nil {
		t.Fatalf("ResolveForAppID: %v", err)
	}
	if build.Path != wantPath {
		t.Errorf("Path = %q, want %q", build.Path, wantPath)
	}
}

func TestResolveForAppID_EmptyRoot_ReturnsSentinel(t *testing.T) {
	_, err := ResolveForAppID("", 1091500)
	if !errors.Is(err, ErrProtonNotResolved) {
		t.Fatalf("err = %v, want ErrProtonNotResolved", err)
	}
}

func TestSupportsVKD3DHeap_MarkerPresent(t *testing.T) {
	root := t.TempDir()
	script := `#!/usr/bin/env python3
# Proton launch shim
import os
if os.environ.get("PROTON_VKD3D_HEAP") == "1":
    enable_vkd3d_heap()
`
	buildPath := writeProtonBuild(t, root, "cachyos-10.0-20260410-slr", true, script)

	got, err := SupportsVKD3DHeap(Build{Name: "cachyos", Path: buildPath})
	if err != nil {
		t.Fatalf("SupportsVKD3DHeap: %v", err)
	}
	if !got {
		t.Errorf("SupportsVKD3DHeap = false, want true when script contains PROTON_VKD3D_HEAP")
	}
}

func TestSupportsVKD3DHeap_MarkerAbsent(t *testing.T) {
	root := t.TempDir()
	script := `#!/usr/bin/env python3
# stock Valve Proton — no descriptor_heap awareness
import os
print(os.environ.get("PROTON_LOG"))
`
	buildPath := writeProtonBuild(t, root, "Proton 9.0 (Beta)", false, script)

	got, err := SupportsVKD3DHeap(Build{Name: "Proton 9.0 (Beta)", Path: buildPath})
	if err != nil {
		t.Fatalf("SupportsVKD3DHeap: %v", err)
	}
	if got {
		t.Errorf("SupportsVKD3DHeap = true, want false when marker absent")
	}
}

func TestSupportsVKD3DHeap_ScriptMissing_ReturnsFalseNoError(t *testing.T) {
	// Directory exists but there's no top-level `proton` script. Treated
	// as "unsupported" per PLAN: detection failures don't block launch.
	root := t.TempDir()
	buildPath := writeProtonBuild(t, root, "broken-build", true, "")

	got, err := SupportsVKD3DHeap(Build{Name: "broken-build", Path: buildPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("SupportsVKD3DHeap = true, want false when script absent")
	}
}

func TestSupportsVKD3DHeap_EmptyBuildPath(t *testing.T) {
	// Zero-value Build (e.g., from a caller that ignored a resolve error)
	// returns false cleanly.
	got, err := SupportsVKD3DHeap(Build{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("SupportsVKD3DHeap(zero) = true, want false")
	}
}
