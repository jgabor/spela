//go:build dev || production || bindings

package gui

import (
	"errors"
	"slices"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

func TestGUIProjectionCarriesValueTextAndNumberHints(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	boundary := defaultGUIApplicationBoundary(nil)
	if err := boundary.patchGameProfile(1091500, []ProfilePatch{
		{Field: profile.FieldGPUClockOffset, Operation: "set", Value: float64(100)},
		{Field: profile.FieldGPUFanSpeed, Operation: "set", Value: float64(40)},
		{Field: profile.FieldGPUPowerLimit, Operation: "set", Value: float64(250)},
		{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true},
	}); err != nil {
		t.Fatal(err)
	}
	view := boundary.profileView(appID(1091500))
	for key, want := range map[string]any{"fanSpeed": 40, "powerLimit": 250, "clockOffset": 100, "enableHdr": true, "affinity": ""} {
		if view[key] != want {
			t.Errorf("view[%s] = %#v, want %#v", key, view[key], want)
		}
	}
	semantics := semanticsByField(view["semantics"].([]ProfileFieldSemantics))
	for field, want := range map[string]string{
		profile.FieldGPUClockOffset:  "100 MHz",
		profile.FieldGPUFanSpeed:     "40%",
		profile.FieldGPUPowerLimit:   "250 W",
		profile.FieldProtonEnableHDR: "On",
		profile.FieldProtonVKD3DHeap: "Off",
		profile.FieldDLSSSRMode:      "none",
	} {
		if got := semantics[field].ValueText; got != want {
			t.Errorf("%s value text = %q, want %q", field, got, want)
		}
	}
	for field, want := range map[string]string{
		profile.FieldGPUFanSpeed:     "Whole number in %, range 0 to 100.",
		profile.FieldGPUClockOffset:  "Whole number in MHz.",
		profile.FieldGPUPowerLimit:   "Whole number in W, range at least 0.",
		profile.FieldProtonVKD3DHeap: "",
		profile.FieldDLSSSRMode:      "",
	} {
		if got := semantics[field].Hint; got != want {
			t.Errorf("%s hint = %q, want %q", field, got, want)
		}
	}
}

func TestGUIClearAllCountsTheSettingsAPageHoldsItself(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := profile.SaveDefault(&profile.Profile{}); err != nil {
		t.Fatal(err)
	}
	boundary := defaultGUIApplicationBoundary(nil)
	clearAll := func(view map[string]any) map[string]any { return view["clearAll"].(map[string]any) }

	if got := clearAll(boundary.profileView(appID(1091500))); got["count"] != 0 || got["action"] != profile.ActionFollowAllGamesForAllSettings || got["body"] != profile.ClearAllBody(false, 0) {
		t.Fatalf("a game without a profile = %#v", got)
	}
	if err := boundary.patchGameProfile(1091500, []ProfilePatch{
		{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true},
		{Field: profile.FieldGPUFanSpeed, Operation: "set", Value: float64(40)},
	}); err != nil {
		t.Fatal(err)
	}
	got := clearAll(boundary.profileView(appID(1091500)))
	if got["count"] != 2 || got["body"] != profile.ClearAllBody(false, 2) {
		t.Fatalf("a game with two own values = %#v", got)
	}

	if err := boundary.patchDefaultProfile([]ProfilePatch{{Field: profile.FieldProtonVKD3DHeap, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	got = clearAll(boundary.profileView(nil))
	if got["count"] != 1 || got["action"] != profile.ActionUseSystemDefaultForAllSettings || got["body"] != profile.ClearAllBody(true, 1) {
		t.Fatalf("All games with one value set = %#v", got)
	}

	// Following clears the count again, and nothing else is written.
	if err := boundary.patchGameProfile(1091500, []ProfilePatch{
		{Field: profile.FieldProtonEnableHDR, Operation: "reset"},
		{Field: profile.FieldGPUFanSpeed, Operation: "reset"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := clearAll(boundary.profileView(appID(1091500))); got["count"] != 0 {
		t.Fatalf("after following every value = %#v", got)
	}
}

func TestGUIAllGamesCountsTheGamesWithTheirOwnValue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	boundary := defaultGUIApplicationBoundary(threeGames())
	if err := boundary.patchDefaultProfile([]ProfilePatch{{Field: profile.FieldProtonVKD3DHeap, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	for _, appID := range []uint64{2, 1} {
		if err := boundary.patchGameProfile(appID, []ProfilePatch{{Field: profile.FieldProtonVKD3DHeap, Operation: "set", Value: false}}); err != nil {
			t.Fatal(err)
		}
	}
	semantics := semanticsByField(boundary.profileView(nil)["semantics"].([]ProfileFieldSemantics))
	heap := semantics[profile.FieldProtonVKD3DHeap]
	if heap.OwnValueSuffix != "2 games have their own value" || heap.OwnValueCount != 2 || heap.OwnValueAction != profile.ActionApplyToGamesWithOwnValue {
		t.Fatalf("VKD3D heap suffix %q, count %d, action %q", heap.OwnValueSuffix, heap.OwnValueCount, heap.OwnValueAction)
	}
	if other := semantics[profile.FieldProtonEnableHDR]; other.OwnValueSuffix != "" || other.OwnValueCount != 0 || other.OwnValueAction != "" {
		t.Fatalf("HDR offers the action although no game sets it: %#v", other)
	}
	for _, item := range semanticsByField(boundary.profileView(appID(1))["semantics"].([]ProfileFieldSemantics)) {
		if item.OwnValueSuffix != "" || item.OwnValueCount != 0 || item.OwnValueAction != "" {
			t.Fatalf("a game page carries the All games count: %#v", item)
		}
	}
}

func threeGames() *game.Database {
	database := &game.Database{Games: map[uint64]*game.Game{}}
	for appID, name := range map[uint64]string{1: "Alpha", 2: "Beta", 3: "Gamma"} {
		database.AddGame(&game.Game{AppID: appID, Name: name})
	}
	return database
}

func TestGUIApplyToGamesWithOwnValueAsksThenFollowsOnlyThatField(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	boundary := defaultGUIApplicationBoundary(threeGames())
	if err := boundary.patchDefaultProfile([]ProfilePatch{{Field: profile.FieldProtonVKD3DHeap, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	for appID, patches := range map[uint64][]ProfilePatch{
		1: {{Field: profile.FieldProtonVKD3DHeap, Operation: "set", Value: false}, {Field: profile.FieldGPUFanSpeed, Operation: "set", Value: float64(40)}},
		2: {{Field: profile.FieldProtonVKD3DHeap, Operation: "set", Value: false}},
	} {
		if err := boundary.patchGameProfile(appID, patches); err != nil {
			t.Fatal(err)
		}
	}

	preview, err := boundary.previewApplyToOwnValues([]string{profile.FieldProtonVKD3DHeap})
	if err != nil {
		t.Fatal(err)
	}
	wantBody := "VKD3D heap: 2 games have their own value: Alpha, Beta. They will follow All games (On). It is saved at once and cannot be undone."
	if preview["action"] != profile.ActionApplyToGamesWithOwnValue || preview["count"] != 2 || preview["body"] != wantBody {
		t.Fatalf("preview = %#v", preview)
	}
	appIDs, _ := preview["appIds"].([]uint64)
	slices.Sort(appIDs)
	if !slices.Equal(appIDs, []uint64{1, 2}) {
		t.Fatalf("preview app IDs = %v, want 1 and 2", appIDs)
	}
	if stored, _ := profile.Load(1); !stored.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Fatal("asking changed a profile")
	}

	// A game that gains its own value after the question was asked is not named
	// by it, so the action leaves it alone.
	if err := boundary.patchGameProfile(3, []ProfilePatch{{Field: profile.FieldProtonVKD3DHeap, Operation: "set", Value: false}}); err != nil {
		t.Fatal(err)
	}
	result, err := boundary.applyToOwnValues([]string{profile.FieldProtonVKD3DHeap}, appIDs)
	if err != nil {
		t.Fatal(err)
	}
	if late, _ := profile.Load(3); !late.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Error("a game the question did not name was changed")
	}
	if result["message"] != "VKD3D heap: 2 games follow All games" {
		t.Errorf("message = %q", result["message"])
	}
	if failed, _ := result["failed"].([]string); len(failed) != 0 {
		t.Errorf("failed = %v", failed)
	}
	view, _ := result["profile"].(map[string]any)
	heap := semanticsByField(view["semantics"].([]ProfileFieldSemantics))[profile.FieldProtonVKD3DHeap]
	if heap.OwnValueCount != 1 {
		t.Errorf("the refreshed view counts %d games, want only the game that was not named", heap.OwnValueCount)
	}
	first, _ := profile.Load(1)
	if first.IsOverridden(profile.FieldProtonVKD3DHeap) || !first.IsOverridden(profile.FieldGPUFanSpeed) || first.GPU.FanSpeed != 40 {
		t.Errorf("game 1 after the action: heap own %v, fan own %v at %d", first.IsOverridden(profile.FieldProtonVKD3DHeap), first.IsOverridden(profile.FieldGPUFanSpeed), first.GPU.FanSpeed)
	}
}

func TestGUIApplyToGamesWithOwnValueTakesTwoFieldsAndOmitsTheValue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	boundary := defaultGUIApplicationBoundary(threeGames())
	if err := boundary.patchGameProfile(1, []ProfilePatch{{Field: profile.FieldDLSSFGEnabled, Operation: "set", Value: true}, {Field: profile.FieldDLSSFGOverride, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	if err := boundary.patchGameProfile(2, []ProfilePatch{{Field: profile.FieldDLSSFGOverride, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	preview, err := boundary.previewApplyToOwnValues([]string{profile.FieldDLSSFGEnabled, profile.FieldDLSSFGOverride})
	if err != nil {
		t.Fatal(err)
	}
	if preview["count"] != 2 || preview["body"] != "Frame generation: 2 games have their own value: Alpha, Beta. They will follow All games. It is saved at once and cannot be undone." {
		t.Fatalf("preview = %#v", preview)
	}
}

func TestGUIApplyToGamesWithOwnValueNamesWhatFailsAndRejectsBadFields(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	boundary := defaultGUIApplicationBoundary(threeGames())
	if err := boundary.patchGameProfile(1, []ProfilePatch{{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true}}); err != nil {
		t.Fatal(err)
	}
	boundary.followAllGames = func([]profile.OwnValueEntry, []string) profile.FollowOutcome {
		return profile.FollowOutcome{Failed: []string{"Alpha"}, Errors: map[string]error{"Alpha": errors.New("disk full")}}
	}
	result, err := boundary.applyToOwnValues([]string{profile.FieldProtonEnableHDR}, []uint64{1})
	if err != nil {
		t.Fatal(err)
	}
	if failed, _ := result["failed"].([]string); !slices.Equal(failed, []string{"Alpha"}) || result["message"] != "Not saved for Alpha (disk full). HDR: 0 games follow All games" || result["failure"] != "Alpha (disk full)" {
		t.Fatalf("result = %#v", result)
	}
	for _, fields := range [][]string{nil, {"nope"}} {
		if _, err := boundary.previewApplyToOwnValues(fields); err == nil {
			t.Errorf("preview accepted %v", fields)
		}
		if _, err := boundary.applyToOwnValues(fields, nil); err == nil {
			t.Errorf("apply accepted %v", fields)
		}
	}
}

func TestGUIPatchesSetEveryNewlyVisibleField(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	boundary := defaultGUIApplicationBoundary(nil)
	if err := boundary.patchGameProfile(7, []ProfilePatch{
		{Field: profile.FieldGPUFanSpeed, Operation: "set", Value: float64(55)},
		{Field: profile.FieldGPUPowerLimit, Operation: "set", Value: float64(200)},
		{Field: profile.FieldCPUAffinity, Operation: "set", Value: "0-3"},
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := profile.Load(7)
	if err != nil || saved == nil || saved.GPU.FanSpeed != 55 || saved.GPU.PowerLimit != 200 || saved.CPU.Affinity != "0-3" {
		t.Fatalf("saved = %#v, %v", saved, err)
	}
	for _, field := range []string{profile.FieldGPUFanSpeed, profile.FieldGPUPowerLimit, profile.FieldCPUAffinity} {
		if !saved.IsOverridden(field) {
			t.Errorf("%s is not marked as the game's own", field)
		}
	}
}

func TestGUIProjectionCarriesTheUnitOfNumbers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := profile.SaveDefault(&profile.Profile{}); err != nil {
		t.Fatal(err)
	}
	semantics := semanticsByField(defaultGUIApplicationBoundary(nil).profileView(nil)["semantics"].([]ProfileFieldSemantics))
	for field, want := range map[string]string{
		profile.FieldGPUFanSpeed: "%", profile.FieldGPUClockOffset: "MHz", profile.FieldGPUMemoryOffset: "MHz",
		profile.FieldGPUPowerLimit: "W", profile.FieldProtonEnableHDR: "", profile.FieldDLSSMultiFrame: "",
	} {
		if got := semantics[field].Unit; got != want {
			t.Errorf("%s unit = %q, want %q", field, got, want)
		}
	}
}
