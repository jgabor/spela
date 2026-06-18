package settings

import (
	"testing"

	"github.com/jgabor/spela/internal/nav"
)

func TestCatalog_MatchesNavSectionLabels(t *testing.T) {
	catalog := Catalog()
	if len(catalog) != len(nav.SettingsSectionLabels) {
		t.Fatalf("catalog len %d, want %d", len(catalog), len(nav.SettingsSectionLabels))
	}
	for i, section := range catalog {
		if section.Title != nav.SettingsSectionLabels[i] {
			t.Errorf("section[%d] title %q, want %q", i, section.Title, nav.SettingsSectionLabels[i])
		}
		if int(section.ID) != i {
			t.Errorf("section[%d] id %d, want %d", i, section.ID, i)
		}
	}
}

func TestCatalog_EachOptionMapsOnce(t *testing.T) {
	seen := make(map[string]string)
	for _, section := range Catalog() {
		for _, opt := range section.Options {
			if opt.Key == "" || opt.JSONKey == "" {
				t.Fatalf("option %q missing key", opt.Label)
			}
			if prev, ok := seen[opt.Key]; ok {
				t.Fatalf("duplicate key %q in sections %q and current", opt.Key, prev)
			}
			seen[opt.Key] = section.Title
		}
	}
}
