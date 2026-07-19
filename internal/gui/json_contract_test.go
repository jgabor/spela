//go:build dev || production || bindings

package gui

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
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
		{"profile", ProfileInfo{}, []string{"clockOffset", "enableHdr", "enableNgxUpdater", "enableWayland", "fgEnabled", "fgIndicator", "fgOverride", "governor", "indicator", "inheritedFromDefault", "memoryOffset", "multiFrame", "overlayEnabled", "overlayPosition", "overlayShowCpu", "overlayShowFps", "overlayShowFrametime", "overlayShowGpu", "overlayShowVram", "overlayToggleKey", "powerMizer", "rrMode", "rrOverride", "rrPreset", "semantics", "shaderCache", "shaderCachePath", "smt", "srMode", "srOverride", "srPreset", "threadedOptimization", "vkd3dHeap"}},
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
