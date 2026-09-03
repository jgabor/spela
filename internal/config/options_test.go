package config

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jgabor/spela/internal/nav"
)

func TestOptionsClassifyEveryConfigField(t *testing.T) {
	want := []struct {
		key        string
		jsonKey    string
		section    nav.SettingsSection
		kind       Kind
		visibility Visibility
	}{
		{"log_level", "logLevel", nav.SettingsLogging, KindEnum, VisibilityCLI | VisibilityGUI | VisibilityWails},
		{"shader_cache", "shaderCache", nav.SettingsPaths, KindPath, VisibilityCLI | VisibilityWails},
		{"check_updates", "checkUpdates", nav.SettingsStartup, KindBool, VisibilityCLI | VisibilityGUI | VisibilityWails},
		{"show_hints", "showHints", nav.SettingsDisplay, KindBool, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"rescan_on_startup", "rescanOnStartup", nav.SettingsStartup, KindBool, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"auto_update_dlls", "autoUpdateDLLs", nav.SettingsStartup, KindBool, VisibilityGUI | VisibilityWails},
		{"steam_path", "steamPath", nav.SettingsPaths, KindPath, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"additional_library_paths", "additionalLibraryPaths", nav.SettingsPaths, KindStringList, VisibilityWails},
		{"dll_cache_path", "dllCachePath", nav.SettingsPaths, KindPath, VisibilityGUI | VisibilityWails},
		{"backup_path", "backupPath", nav.SettingsPaths, KindPath, VisibilityGUI | VisibilityWails},
		{"dll_manifest_url", "dllManifestURL", nav.SettingsDLLPolicy, KindPath, VisibilityWails},
		{"auto_refresh_manifest", "autoRefreshManifest", nav.SettingsDLLPolicy, KindBool, VisibilityGUI | VisibilityWails},
		{"manifest_refresh_hours", "manifestRefreshHours", nav.SettingsDLLPolicy, KindInt, VisibilityGUI | VisibilityWails},
		{"preferred_dll_source", "preferredDLLSource", nav.SettingsDLLPolicy, KindEnum, VisibilityGUI | VisibilityWails},
		{"theme", "theme", nav.SettingsDisplay, KindEnum, VisibilityGUI | VisibilityWails},
		{"compact_mode", "compactMode", nav.SettingsDisplay, KindBool, VisibilityGUI | VisibilityWails},
		{"confirm_destructive", "confirmDestructive", nav.SettingsDisplay, KindBool, VisibilityTUI | VisibilityGUI | VisibilityWails},
	}
	if reflect.TypeOf(Config{}).NumField() != len(want) || len(Options()) != len(want) {
		t.Fatalf("Config fields = %d, options = %d, want %d", reflect.TypeOf(Config{}).NumField(), len(Options()), len(want))
	}
	for index, expected := range want {
		option := Options()[index]
		if option.Key != expected.key || option.JSONKey != expected.jsonKey || option.Section != expected.section || option.Kind != expected.kind || option.Visibility != expected.visibility {
			t.Errorf("option[%d] = {%q %q %d %d %d}, want %+v", index, option.Key, option.JSONKey, option.Section, option.Kind, option.Visibility, expected)
		}
		if option.Label == "" || option.Description == "" || OptionByKey(option.Key) != &Options()[index] || OptionByJSONKey(option.JSONKey) == nil {
			t.Errorf("option[%d] has incomplete operations or presentation metadata", index)
		}
	}
	if OptionByKey("missing") != nil || OptionByJSONKey("missing") != nil {
		t.Fatal("unknown option unexpectedly resolved")
	}
}

func TestOptionGetSetRoundTripAndExplicitZero(t *testing.T) {
	original := &Config{
		LogLevel: LogLevelError, ShaderCache: "", CheckUpdates: false, ShowHints: false,
		RescanOnStartup: false, AutoUpdateDLLs: true, SteamPath: "", AdditionalLibraryPaths: []string{},
		DLLCachePath: "", BackupPath: "", DLLManifestURL: "", AutoRefreshManifest: false,
		ManifestRefreshHours: 0, PreferredDLLSource: "github", Theme: "light", CompactMode: true, ConfirmDestructive: false,
	}
	copy := Default()
	for _, option := range Options() {
		if err := option.Set(copy, option.Get(original)); err != nil {
			t.Fatalf("Set(%s) error = %v", option.Key, err)
		}
	}
	if !reflect.DeepEqual(copy, original) {
		t.Fatalf("option round trip = %#v, want %#v", copy, original)
	}
	if err := Validate(original); err != nil {
		t.Fatalf("Validate(explicit zero) error = %v", err)
	}
}

func TestOptionValidationAndCatalogParity(t *testing.T) {
	configuration := Default()
	for _, test := range []struct{ key, value, want string }{
		{"log_level", "verbose", "unsupported log level: verbose"},
		{"preferred_dll_source", "mirror", "unsupported DLL source: mirror"},
		{"theme", "neon", "unsupported theme: neon"},
		{"show_hints", "maybe", "invalid boolean for show_hints: maybe"},
		{"manifest_refresh_hours", "daily", "invalid integer for manifest_refresh_hours: daily"},
		{"additional_library_paths", "games", "invalid string list for additional_library_paths"},
	} {
		err := OptionByKey(test.key).Set(configuration, test.value)
		if err == nil || len(err.Error()) < len(test.want) || err.Error()[:len(test.want)] != test.want {
			t.Errorf("Set(%s, %q) error = %v, want prefix %q", test.key, test.value, err, test.want)
		}
	}
	theme := OptionByKey("theme")
	if !reflect.DeepEqual(theme.Choices, []string{"default", "dark", "light"}) {
		t.Fatalf("theme choices = %v", theme.Choices)
	}
	for _, choice := range theme.Choices {
		if err := theme.Set(configuration, choice); err != nil {
			t.Errorf("catalogued theme %q rejected: %v", choice, err)
		}
	}
	if len(Sections(VisibilityTUI)) != len(nav.SettingsSectionLabels) || len(Sections(VisibilityGUI)) != len(nav.SettingsSectionLabels) {
		t.Fatal("visible catalogs do not preserve navigation sections")
	}
}

func TestConfigJSONKeysComeFromPublicTags(t *testing.T) {
	data, err := json.Marshal(Config{})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, option := range Options() {
		if _, ok := got[option.JSONKey]; !ok {
			t.Errorf("JSON key %q missing", option.JSONKey)
		}
	}
}

func TestWailsCatalogOnlyExposesGUIOptions(t *testing.T) {
	seen := map[string]bool{}
	for _, section := range WailsCatalog() {
		for _, option := range section.Options {
			seen[option.Key] = true
		}
	}
	for _, hidden := range []string{"shaderCache", "additionalLibraryPaths", "dllManifestURL"} {
		if seen[hidden] {
			t.Errorf("hidden option %q exposed in GUI catalog", hidden)
		}
	}
	for _, visible := range []string{"theme", "showHints", "manifestRefreshHours", "logLevel"} {
		if !seen[visible] {
			t.Errorf("GUI option %q missing", visible)
		}
	}
}
