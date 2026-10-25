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
		{"log_level", "logLevel", nav.SettingsLogging, KindEnum, VisibilityCLI | VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"shader_cache", "shaderCache", nav.SettingsPaths, KindPath, VisibilityCLI | VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"check_updates", "checkUpdates", nav.SettingsStartup, KindBool, VisibilityCLI | VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"rescan_on_startup", "rescanOnStartup", nav.SettingsStartup, KindBool, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"auto_update_dlls", "autoUpdateDLLs", nav.SettingsStartup, KindBool, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"steam_path", "steamPath", nav.SettingsPaths, KindPath, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"additional_library_paths", "additionalLibraryPaths", nav.SettingsPaths, KindStringList, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"dll_cache_path", "dllCachePath", nav.SettingsPaths, KindPath, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"backup_path", "backupPath", nav.SettingsPaths, KindPath, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"dll_manifest_url", "dllManifestURL", nav.SettingsDLLPolicy, KindPath, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"auto_refresh_manifest", "autoRefreshManifest", nav.SettingsDLLPolicy, KindBool, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"manifest_refresh_hours", "manifestRefreshHours", nav.SettingsDLLPolicy, KindInt, VisibilityTUI | VisibilityGUI | VisibilityWails},
		{"theme", "theme", nav.SettingsDisplay, KindEnum, VisibilityGUI | VisibilityWails},
		{"compact_mode", "compactMode", nav.SettingsDisplay, KindBool, VisibilityTUI | VisibilityGUI | VisibilityWails},
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
		LogLevel: LogLevelError, ShaderCache: "", CheckUpdates: false,
		RescanOnStartup: false, AutoUpdateDLLs: true, SteamPath: "", AdditionalLibraryPaths: []string{},
		DLLCachePath: "", BackupPath: "", DLLManifestURL: "", AutoRefreshManifest: false,
		ManifestRefreshHours: 0, Theme: "light", CompactMode: true, ConfirmDestructive: false,
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
		{"dll_manifest_url", "http://example.test/manifest.json", "dll_manifest_url must be an https URL"},
		{"dll_manifest_url", "example.test/manifest.json", "dll_manifest_url must be an https URL"},
		{"theme", "neon", "unsupported theme: neon"},
		{"rescan_on_startup", "maybe", "invalid boolean for rescan_on_startup: maybe"},
		{"manifest_refresh_hours", "daily", "invalid integer for manifest_refresh_hours: daily"},
		{"additional_library_paths", "[games", "invalid string list for additional_library_paths"},
		{"dll_cache_path", "dlls", "dll_cache_path must be an absolute path: dlls"},
		{"backup_path", "~/backups", "backup_path must be an absolute path: ~/backups"},
		{"shader_cache", "cache/nvidia", "shader_cache must be an absolute path: cache/nvidia"},
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
	for _, visible := range []string{"theme", "manifestRefreshHours", "logLevel", "additionalLibraryPaths", "shaderCache", "dllCachePath", "backupPath", "dllManifestURL", "autoRefreshManifest"} {
		if !seen[visible] {
			t.Errorf("GUI option %q missing", visible)
		}
	}
}

func TestListOptionAcceptsTextAndJSONForms(t *testing.T) {
	option := OptionByKey("additional_library_paths")
	for _, test := range []struct {
		name, value string
		want        []string
	}{
		{"text", "/games/a:/games/b", []string{"/games/a", "/games/b"}},
		{"text with blanks", " /games/a ::/games/b: ", []string{"/games/a", "/games/b"}},
		{"json", `["/games/a","/games/b"]`, []string{"/games/a", "/games/b"}},
		{"empty", "", []string{}},
	} {
		configuration := Default()
		if err := option.Set(configuration, test.value); err != nil {
			t.Fatalf("%s: Set error = %v", test.name, err)
		}
		if !reflect.DeepEqual(configuration.AdditionalLibraryPaths, test.want) {
			t.Errorf("%s: paths = %#v, want %#v", test.name, configuration.AdditionalLibraryPaths, test.want)
		}
	}
	configuration := Default()
	configuration.AdditionalLibraryPaths = []string{"/games/a", "/games/b"}
	if got := option.Display(configuration); got != "/games/a:/games/b" {
		t.Errorf("Display = %q, want colon form", got)
	}
	if got := OptionByKey("log_level").Display(configuration); got != string(LogLevelInfo) {
		t.Errorf("Display(log_level) = %q, want canonical value", got)
	}
}
