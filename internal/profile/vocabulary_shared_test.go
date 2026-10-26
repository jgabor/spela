package profile

import (
	"errors"
	"reflect"
	"testing"
)

func TestClearAllWordsNameTheActionAndTheCount(t *testing.T) {
	if ClearAllAction(false) != ActionFollowAllGamesForAllSettings || ClearAllAction(true) != ActionUseSystemDefaultForAllSettings {
		t.Fatal("whole-page actions do not use the shared words")
	}
	for _, test := range []struct {
		allGames bool
		count    int
		want     string
	}{
		{false, 0, "Nothing to clear: every value already follows All games."},
		{false, 1, "This clears the 1 setting that this game sets itself. It follows All games afterwards. It is saved at once."},
		{false, 5, "This clears the 5 settings that this game sets itself. They follow All games afterwards. It is saved at once."},
		{true, 0, "Nothing to clear: every value already follows the system default."},
		{true, 3, "This clears the 3 settings that All games sets itself. They follow the system default afterwards. It is saved at once."},
	} {
		if got := ClearAllBody(test.allGames, test.count); got != test.want {
			t.Errorf("ClearAllBody(%v, %d) = %q, want %q", test.allGames, test.count, got, test.want)
		}
	}
}

func TestOwnCountCountsWhatThePageSetsItself(t *testing.T) {
	explanations := []FieldExplanation{
		{Origin: OriginThisGame}, {Origin: OriginThisGameSame}, {Origin: OriginAllGames}, {Origin: OriginSetHere}, {Origin: OriginSystemDefault},
	}
	if got := OwnCount(explanations, false); got != 2 {
		t.Errorf("game count = %d, want 2", got)
	}
	if got := OwnCount(explanations, true); got != 1 {
		t.Errorf("All games count = %d, want 1", got)
	}
}

func TestOwnValueWordsCountTheGamesWithTheirOwnValue(t *testing.T) {
	for count, want := range map[int]string{0: "", 1: "1 game has its own value", 4: "4 games have their own value"} {
		if got := OwnValueSuffix(count); got != want {
			t.Errorf("OwnValueSuffix(%d) = %q, want %q", count, got, want)
		}
	}
}

func TestGamesTextCountsGames(t *testing.T) {
	for count, want := range map[int]string{0: "0 games", 1: "1 game", 6: "6 games"} {
		if got := GamesText(count); got != want {
			t.Errorf("GamesText(%d) = %q, want %q", count, got, want)
		}
	}
}

func TestApplyToOwnValueBodyNamesTheGamesAndTheValue(t *testing.T) {
	for _, test := range []struct {
		names []string
		want  string
	}{
		{[]string{"Alpha"}, "SR indicator: 1 game has their own value: Alpha. It will follow All games (On). It is saved at once and cannot be undone."},
		{[]string{"Alpha", "Beta"}, "SR indicator: 2 games have their own value: Alpha, Beta. They will follow All games (On). It is saved at once and cannot be undone."},
		{[]string{"A", "B", "C", "D", "E", "F", "G", "H"}, "SR indicator: 8 games have their own value: A, B, C, D, E, F and 2 more. They will follow All games (On). It is saved at once and cannot be undone."},
	} {
		if got := ApplyToOwnValueBody("SR indicator", "On", test.names); got != test.want {
			t.Errorf("ApplyToOwnValueBody(%v) = %q, want %q", test.names, got, test.want)
		}
	}
}

func TestFollowOutcomeTextSaysWhatChangedAndNamesFailures(t *testing.T) {
	ok := FollowOutcome{Followed: []string{"Alpha", "Beta"}}
	if got, want := FollowOutcomeText("SR indicator", ok), "SR indicator: 2 games follow All games"; got != want {
		t.Errorf("FollowOutcomeText = %q, want %q", got, want)
	}
	one := FollowOutcome{Followed: []string{"Alpha"}}
	if got, want := FollowOutcomeText("SR indicator", one), "SR indicator: 1 game follows All games"; got != want {
		t.Errorf("FollowOutcomeText = %q, want %q", got, want)
	}
	failed := FollowOutcome{Followed: []string{"Alpha"}, Failed: []string{"Beta"}, Errors: map[string]error{"Beta": errors.New("disk full")}}
	if got, want := FollowOutcomeText("SR indicator", failed), "Not saved for Beta (disk full). SR indicator: 1 game follows All games"; got != want {
		t.Errorf("FollowOutcomeText = %q, want %q", got, want)
	}
	if got, want := FollowFailureText(failed), "Beta (disk full)"; got != want {
		t.Errorf("FollowFailureText = %q, want %q", got, want)
	}
	if got := FollowFailureText(ok); got != "" {
		t.Errorf("FollowFailureText with no failure = %q", got)
	}
}

func TestCollectOwnValueGamesSortsByNameAndSkipsMissingProfiles(t *testing.T) {
	first, second := &Profile{}, &Profile{}
	first.MarkOverride(FieldProtonEnableHDR)
	second.MarkOverride(FieldProtonEnableHDR)
	second.MarkOverride(FieldGPUFanSpeed)
	got := CollectOwnValueGames([]OwnValueEntry{{Name: "Zeta", Profile: second}, {Name: "Alpha", Profile: first}, {Name: "None"}})
	want := map[string][]string{FieldProtonEnableHDR: {"Alpha", "Zeta"}, FieldGPUFanSpeed: {"Zeta"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectOwnValueGames = %v, want %v", got, want)
	}
}

func TestNumberHintNamesUnitAndRangeOfFreeNumbersOnly(t *testing.T) {
	for field, want := range map[string]string{
		FieldGPUFanSpeed:        "Whole number in %, range 0 to 100.",
		FieldGPUClockOffset:     "Whole number in MHz.",
		FieldGPUPowerLimit:      "Whole number in W, range at least 0.",
		FieldDLSSMultiFrame:     "",
		FieldProtonEnableHDR:    "",
		FieldGPUShaderCachePath: "",
	} {
		descriptor, _ := Field(field)
		if got := NumberHint(descriptor); got != want {
			t.Errorf("NumberHint(%s) = %q, want %q", field, got, want)
		}
	}
}
