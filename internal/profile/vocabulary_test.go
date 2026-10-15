package profile_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

func mustDescriptor(t *testing.T, field string) profile.FieldDescriptor {
	t.Helper()
	descriptor, ok := profile.Field(field)
	if !ok {
		t.Fatalf("unknown field %s", field)
	}
	return descriptor
}

// Every (scope, source) pair in the shared vocabulary table renders the exact
// words the review specified.
func TestSourceText_EveryScopeAndSourcePair(t *testing.T) {
	allGamesOff := &profile.Profile{}
	allGamesOff.MarkOverride(profile.FieldProtonEnableHDR) // explicit Off
	allGamesOn := &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}
	allGamesClock := &profile.Profile{GPU: profile.GPUSettings{ClockOffset: 150}}

	gamePins := func(field string, value any) *profile.Profile {
		game := &profile.Profile{}
		if err := game.Set(field, value); err != nil {
			t.Fatalf("Set %s: %v", field, err)
		}
		return game
	}

	cases := []struct {
		name     string
		game     *profile.Profile
		defaults *profile.Profile
		field    string
		origin   profile.ValueOrigin
		want     string
	}{
		{"follows a value All games sets", &profile.Profile{}, allGamesOn, profile.FieldProtonEnableHDR, profile.OriginAllGames, "from All games"},
		{"follows an explicit Off in All games", &profile.Profile{}, allGamesOff, profile.FieldProtonEnableHDR, profile.OriginAllGames, "from All games"},
		{"nothing sets it, no defaults file", &profile.Profile{}, nil, profile.FieldProtonEnableHDR, profile.OriginSystemDefault, "system default"},
		{"nothing sets it, defaults leave it zero", &profile.Profile{}, allGamesClock, profile.FieldProtonEnableHDR, profile.OriginSystemDefault, "system default"},
		{"own value differs from All games", gamePins(profile.FieldProtonEnableHDR, true), allGamesOff, profile.FieldProtonEnableHDR, profile.OriginThisGame, "this game · All games: Off"},
		{"own value differs, All games unset boolean", gamePins(profile.FieldProtonEnableHDR, true), nil, profile.FieldProtonEnableHDR, profile.OriginThisGame, "this game · All games: Off"},
		{"own number differs, All games sets it", gamePins(profile.FieldGPUClockOffset, 100), allGamesClock, profile.FieldGPUClockOffset, profile.OriginThisGame, "this game · All games: 150 MHz"},
		{"own number differs, All games unset", gamePins(profile.FieldGPUClockOffset, 100), nil, profile.FieldGPUClockOffset, profile.OriginThisGame, "this game · All games: none"},
		{"own string differs, All games unset", gamePins(profile.FieldDLSSSRMode, "quality"), nil, profile.FieldDLSSSRMode, profile.OriginThisGame, "this game · All games: none"},
		{"own value equals All games", gamePins(profile.FieldProtonEnableHDR, true), allGamesOn, profile.FieldProtonEnableHDR, profile.OriginThisGameSame, "this game · same as All games"},
		{"own explicit Off equals unset All games", gamePins(profile.FieldProtonEnableHDR, false), nil, profile.FieldProtonEnableHDR, profile.OriginThisGameSame, "this game · same as All games"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			explanation, err := tc.game.ExplainField(tc.field, tc.defaults)
			if err != nil {
				t.Fatal(err)
			}
			if explanation.Origin != tc.origin {
				t.Fatalf("origin = %s, want %s", explanation.Origin, tc.origin)
			}
			if got := profile.SourceText(mustDescriptor(t, tc.field), explanation); got != tc.want {
				t.Fatalf("SourceText = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("All games scope, set here", func(t *testing.T) {
		explanation, err := allGamesOn.ExplainAllGamesField(profile.FieldProtonEnableHDR)
		if err != nil {
			t.Fatal(err)
		}
		if got := profile.SourceText(mustDescriptor(t, profile.FieldProtonEnableHDR), explanation); got != "set here" {
			t.Fatalf("SourceText = %q", got)
		}
	})
	t.Run("All games scope, explicit Off counts as set", func(t *testing.T) {
		explanation, err := allGamesOff.ExplainAllGamesField(profile.FieldProtonEnableHDR)
		if err != nil {
			t.Fatal(err)
		}
		if explanation.Origin != profile.OriginSetHere {
			t.Fatalf("origin = %s", explanation.Origin)
		}
	})
	t.Run("All games scope, nothing set", func(t *testing.T) {
		explanation, err := (&profile.Profile{}).ExplainAllGamesField(profile.FieldProtonEnableHDR)
		if err != nil {
			t.Fatal(err)
		}
		if got := profile.SourceText(mustDescriptor(t, profile.FieldProtonEnableHDR), explanation); got != "system default" {
			t.Fatalf("SourceText = %q", got)
		}
	})
	t.Run("All games scope, unknown field", func(t *testing.T) {
		if _, err := (&profile.Profile{}).ExplainAllGamesField("nope"); err == nil {
			t.Fatal("expected an error")
		}
	})
}

// The existing source strings are a contract with the graphical interface and
// must not change when origins are added.
func TestExplainField_SourceStringsUnchanged(t *testing.T) {
	game := &profile.Profile{}
	if err := game.Set(profile.FieldGPUClockOffset, 100); err != nil {
		t.Fatal(err)
	}
	defaults := &profile.Profile{GPU: profile.GPUSettings{ClockOffset: 150}}
	for _, tc := range []struct {
		name     string
		game     *profile.Profile
		defaults *profile.Profile
		field    string
		want     profile.ExplanationSource
	}{
		{"override", game, defaults, profile.FieldGPUClockOffset, profile.ExplanationSourceOverride},
		{"default", game, defaults, profile.FieldGPUFanSpeed, profile.ExplanationSourceDefault},
		{"unset", game, nil, profile.FieldGPUFanSpeed, profile.ExplanationSourceUnset},
	} {
		explanation, err := tc.game.ExplainField(tc.field, tc.defaults)
		if err != nil {
			t.Fatal(err)
		}
		if explanation.Source != tc.want {
			t.Errorf("%s: source = %s, want %s", tc.name, explanation.Source, tc.want)
		}
	}
}

// A default.yaml saved before the overrides marker existed holds values with no
// marker; a non-zero value there still counts as set by All games.
func TestExplainField_LegacyDefaultsWithoutMarkerCountAsSet(t *testing.T) {
	legacyDefaults := &profile.Profile{GPU: profile.GPUSettings{FanSpeed: 60}}
	explanation, err := (&profile.Profile{}).ExplainField(profile.FieldGPUFanSpeed, legacyDefaults)
	if err != nil {
		t.Fatal(err)
	}
	if explanation.Origin != profile.OriginAllGames {
		t.Fatalf("origin = %s, want all_games", explanation.Origin)
	}
}

func TestDisplayValue(t *testing.T) {
	cases := []struct {
		field string
		value any
		want  string
	}{
		{profile.FieldProtonEnableHDR, true, "On"},
		{profile.FieldProtonEnableHDR, false, "Off"},
		{profile.FieldProtonEnableHDR, nil, "Off"},
		{profile.FieldCPUSMT, true, "On"},
		{profile.FieldCPUSMT, false, "Off"},
		{profile.FieldCPUSMT, nil, "none"},
		{profile.FieldGPUClockOffset, 150, "150 MHz"},
		{profile.FieldGPUClockOffset, 0, "0 MHz"},
		{profile.FieldGPUPowerLimit, 250, "250 W"},
		{profile.FieldGPUFanSpeed, 60, "60%"},
		{profile.FieldDLSSMultiFrame, 2, "2"},
		{profile.FieldDLSSSRMode, profile.DLSSMode(""), "none"},
		{profile.FieldDLSSSRMode, profile.DLSSMode("quality"), "quality"},
		{profile.FieldGPUShaderCachePath, nil, "none"},
	}
	for _, tc := range cases {
		if got := profile.DisplayValue(mustDescriptor(t, tc.field), tc.value); got != tc.want {
			t.Errorf("%s %v = %q, want %q", tc.field, tc.value, got, tc.want)
		}
	}
}

func TestActionWords(t *testing.T) {
	if profile.ResetActionText(false) != "Follow All games" || profile.ResetActionText(true) != "Use system default" {
		t.Fatal("reset action words changed")
	}
}

func TestVocabularyUsesNoRetiredWords(t *testing.T) {
	var texts []string
	texts = append(texts,
		profile.AllGamesName, profile.ActionFollowAllGames, profile.ActionUseSystemDefault,
		profile.ActionFollowAllGamesForAllSettings, profile.ActionUseSystemDefaultForAllSettings, profile.ActionSetForThisGame,
		profile.WordOn, profile.WordOff, profile.WordNone)
	for _, origin := range []profile.ValueOrigin{
		profile.OriginThisGame, profile.OriginThisGameSame, profile.OriginAllGames,
		profile.OriginSystemDefault, profile.OriginSetHere,
	} {
		for _, descriptor := range profile.Fields() {
			texts = append(texts, profile.SourceText(descriptor, profile.FieldExplanation{Origin: origin}))
		}
	}
	for _, descriptor := range profile.Fields() {
		texts = append(texts, descriptor.Label, descriptor.Description)
	}
	for _, text := range texts {
		for _, retired := range profile.RetiredWords() {
			if strings.Contains(text, retired) {
				t.Errorf("%q contains retired word %q", text, retired)
			}
		}
	}
}

// Every descriptor carries a real description, every free numeric field a
// unit, and ranges are limited to those that hold on every device.
func TestDescriptors_DescriptionsUnitsAndRanges(t *testing.T) {
	for _, descriptor := range profile.Fields() {
		if descriptor.Description == "" || strings.HasPrefix(descriptor.Description, "Configure ") {
			t.Errorf("%s: description %q is missing or generated", descriptor.Key, descriptor.Description)
		}
		if descriptor.Kind == profile.PrimitiveInt && len(descriptor.AllowedValues) == 0 && descriptor.Unit == "" {
			t.Errorf("%s: free numeric field has no unit", descriptor.Key)
		}
		if descriptor.Range != nil && descriptor.Key != profile.FieldGPUFanSpeed {
			t.Errorf("%s: range %+v has no source that holds for every device", descriptor.Key, *descriptor.Range)
		}
	}
	fan := mustDescriptor(t, profile.FieldGPUFanSpeed)
	if fan.Range == nil || fan.Range.Min != 0 || fan.Range.Max != 100 || fan.Unit != "%" {
		t.Errorf("fan speed = %+v, want 0 to 100 %%", fan)
	}
}

func TestDescriptor_RangeIsACopy(t *testing.T) {
	first := mustDescriptor(t, profile.FieldGPUFanSpeed)
	first.Range.Max = 5
	if second := mustDescriptor(t, profile.FieldGPUFanSpeed); second.Range.Max != 100 {
		t.Fatal("mutating a returned descriptor changed the table")
	}
}

// Ranges are display only: an out-of-range stored value still loads and saves.
func TestRanges_AreNotEnforced(t *testing.T) {
	withTempXDG(t)
	game := &profile.Profile{}
	if err := game.Set(profile.FieldGPUFanSpeed, 150); err != nil {
		t.Fatalf("Set refused an out-of-range value: %v", err)
	}
	if err := profile.Save(1091500, game); err != nil {
		t.Fatal(err)
	}
	loaded, err := profile.Load(1091500)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GPU.FanSpeed != 150 || !loaded.IsOverridden(profile.FieldGPUFanSpeed) {
		t.Fatalf("out-of-range value lost: %+v", loaded.GPU)
	}
}

func TestValueText_UnsetNumberReadsNone(t *testing.T) {
	clock := mustDescriptor(t, profile.FieldGPUClockOffset)
	unset, err := (&profile.Profile{}).ExplainField(profile.FieldGPUClockOffset, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := profile.ValueText(clock, unset); got != "none" {
		t.Errorf("unset number = %q, want none", got)
	}
	own := &profile.Profile{}
	if err := own.Set(profile.FieldGPUClockOffset, 0); err != nil {
		t.Fatal(err)
	}
	pinned, err := own.ExplainField(profile.FieldGPUClockOffset, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := profile.ValueText(clock, pinned); got != "0 MHz" {
		t.Errorf("explicit zero = %q, want 0 MHz", got)
	}
}

// docs/profile-fields.md lists every descriptor once, in descriptor order, with
// the descriptor label and type, so the document cannot drift from the code.
func TestFieldDocumentMatchesDescriptors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "profile-fields.md"))
	if err != nil {
		t.Fatal(err)
	}
	rowPattern := regexp.MustCompile("^\\| `([a-z_.0-9]+)` \\| ([^|]+) \\| ([^|]+) \\|")
	type row struct{ key, kind, label string }
	var rows []row
	for _, line := range strings.Split(string(data), "\n") {
		if match := rowPattern.FindStringSubmatch(line); match != nil {
			rows = append(rows, row{match[1], strings.TrimSpace(match[2]), strings.TrimSpace(match[3])})
		}
	}
	descriptors := profile.Fields()
	if len(rows) != len(descriptors) {
		t.Fatalf("document has %d field rows, descriptors have %d", len(rows), len(descriptors))
	}
	kinds := map[profile.PrimitiveKind]string{
		profile.PrimitiveBool: "bool", profile.PrimitiveInt: "int",
		profile.PrimitiveString: "string", profile.PrimitiveOptionalBool: "optional bool",
	}
	for index, descriptor := range descriptors {
		got := rows[index]
		if got.key != descriptor.Key {
			t.Errorf("row %d is %s, descriptor order has %s", index, got.key, descriptor.Key)
			continue
		}
		if got.label != descriptor.Label {
			t.Errorf("%s: document label %q, descriptor label %q", descriptor.Key, got.label, descriptor.Label)
		}
		if got.kind != kinds[descriptor.Kind] {
			t.Errorf("%s: document type %q, descriptor type %q", descriptor.Key, got.kind, kinds[descriptor.Kind])
		}
	}
}
