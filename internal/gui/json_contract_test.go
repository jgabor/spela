//go:build dev || production || bindings

package gui

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

func TestWailsJSONKeyContracts(t *testing.T) {
	configType := reflect.TypeOf(ConfigInfo{})
	if configType.Name() != "ConfigInfo" || configType.PkgPath() != "github.com/jgabor/spela/internal/gui" {
		t.Fatalf("ConfigInfo Wails source model = %s.%s, want gui.ConfigInfo", configType.PkgPath(), configType.Name())
	}

	tests := []struct {
		name  string
		value any
		keys  []string
	}{
		{"config", ConfigInfo{}, []string{"additionalLibraryPaths", "autoRefreshManifest", "autoUpdateDLLs", "backupPath", "checkUpdates", "compactMode", "confirmDestructive", "dllCachePath", "dllManifestURL", "logLevel", "manifestRefreshHours", "preferredDLLSource", "rescanOnStartup", "shaderCache", "showHints", "steamPath", "theme"}},
		{"game", GameInfo{}, []string{"appId", "dlls", "hasProfile", "installDir", "name", "prefixPath"}},
		{"dll", DLLInfo{}, []string{"dllType", "name", "path", "version"}},
		{"dll update", DLLUpdateInfo{}, []string{"currentVersion", "hasUpdate", "latestVersion", "name"}},
		{"profile patch", ProfilePatch{}, []string{"field", "operation", "value"}},
		{"profile field semantics", ProfileFieldSemantics{}, []string{"field", "impact", "restore", "source"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]any
			if err := json.Unmarshal(data, &object); err != nil {
				t.Fatal(err)
			}
			keys := reflect.ValueOf(object).MapKeys()
			got := make([]string, len(keys))
			for index, key := range keys {
				got[index] = key.String()
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, test.keys) {
				t.Fatalf("Wails JSON keys = %v, want %v", got, test.keys)
			}
		})
	}
}

func TestProfileViewJSONKeysRemainCompatible(t *testing.T) {
	view := profileView(&profile.Profile{}, nil, false)
	got := make([]string, 0, len(view))
	for key := range view {
		got = append(got, key)
	}
	sort.Strings(got)
	want := []string{"clockOffset", "enableHdr", "enableNgxUpdater", "enableWayland", "fgEnabled", "fgIndicator", "fgOverride", "governor", "indicator", "inheritedFromDefault", "memoryOffset", "multiFrame", "overlayEnabled", "overlayPosition", "overlayShowCpu", "overlayShowFps", "overlayShowFrametime", "overlayShowGpu", "overlayShowVram", "overlayToggleKey", "powerMizer", "rrMode", "rrOverride", "rrPreset", "semantics", "shaderCache", "shaderCachePath", "smt", "srMode", "srOverride", "srPreset", "threadedOptimization", "vkd3dHeap", "vrr"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profile view JSON keys = %v, want %v", got, want)
	}
}

func TestWailsProfilePatchSignatureUsesBatchAndPlainMap(t *testing.T) {
	method := reflect.TypeOf((*App).PatchProfile)
	if method.In(2).Kind() != reflect.Slice || method.In(2).Elem() != reflect.TypeOf(ProfilePatch{}) {
		t.Fatalf("PatchProfile patches = %s, want []ProfilePatch", method.In(2))
	}
	if method.Out(0).Kind() != reflect.Map || method.Out(0).Name() != "" {
		t.Fatalf("PatchProfile result = %s, want unnamed map", method.Out(0))
	}
}
