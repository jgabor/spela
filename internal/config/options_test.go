package config

import (
	"reflect"
	"testing"

	"github.com/jgabor/spela/internal/nav"
)

func TestOptionsClassifyEveryConfigField(t *testing.T) {
	want := []struct {
		key        string
		section    nav.SettingsSection
		kind       Kind
		visibility Visibility
	}{
		{"log_level", nav.SettingsLogging, KindEnum, VisibilityCLI | VisibilityTUI},
		{"shader_cache", nav.SettingsPaths, KindPath, VisibilityCLI | VisibilityTUI},
		{"check_updates", nav.SettingsStartup, KindBool, VisibilityCLI | VisibilityTUI},
		{"rescan_on_startup", nav.SettingsStartup, KindBool, VisibilityTUI},
		{"auto_update_dlls", nav.SettingsStartup, KindBool, VisibilityTUI},
		{"steam_path", nav.SettingsPaths, KindPath, VisibilityTUI},
		{"additional_library_paths", nav.SettingsPaths, KindStringList, VisibilityTUI},
		{"dll_cache_path", nav.SettingsPaths, KindPath, VisibilityTUI},
		{"backup_path", nav.SettingsPaths, KindPath, VisibilityTUI},
		{"dll_manifest_url", nav.SettingsDLLPolicy, KindPath, VisibilityTUI},
		{"auto_refresh_manifest", nav.SettingsDLLPolicy, KindBool, VisibilityTUI},
		{"manifest_refresh_hours", nav.SettingsDLLPolicy, KindInt, VisibilityTUI},
		{"theme", nav.SettingsDisplay, KindEnum, 0},
		{"compact_mode", nav.SettingsDisplay, KindBool, VisibilityTUI},
		{"confirm_destructive", nav.SettingsDisplay, KindBool, VisibilityTUI},
	}
	if reflect.TypeOf(Config{}).NumField() != len(want) || len(Options()) != len(want) {
		t.Fatalf("Config fields = %d, options = %d, want %d", reflect.TypeOf(Config{}).NumField(), len(Options()), len(want))
	}
	for index, expected := range want {
		option := Options()[index]
		if option.Key != expected.key || option.Section != expected.section || option.Kind != expected.kind || option.Visibility != expected.visibility {
			t.Errorf("option[%d] = {%q %d %d %d}, want %+v", index, option.Key, option.Section, option.Kind, option.Visibility, expected)
		}
		if option.Label == "" || option.Description == "" || OptionByKey(option.Key) != &Options()[index] {
			t.Errorf("option[%d] has incomplete operations or presentation metadata", index)
		}
	}
	if OptionByKey("missing") != nil {
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
	if len(Sections(VisibilityTUI)) != len(nav.SettingsSectionLabels) {
		t.Fatal("visible catalogs do not preserve navigation sections")
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
