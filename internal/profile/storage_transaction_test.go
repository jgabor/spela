package profile

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMutateConcurrentDistinctSurfaceFieldsAndRollback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Save(7, &Profile{Name: "original"}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var group sync.WaitGroup
	for surface, mutation := range map[string]struct {
		field string
		value any
	}{"gui": {FieldProtonEnableHDR, true}, "cli": {FieldGPUClockOffset, 125}} {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			if err := Mutate(7, func(current, _ *Profile) error { return current.Set(mutation.field, mutation.value) }); err != nil {
				t.Errorf("%s mutation: %v", surface, err)
			}
		}()
	}
	close(start)
	group.Wait()
	stored, err := Load(7)
	if err != nil || !stored.Proton.EnableHDR || stored.GPU.ClockOffset != 125 {
		t.Fatalf("concurrent profile = %+v, err %v", stored, err)
	}
	before, err := os.ReadFile(profilePath(7))
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("reject mutation")
	if err := Mutate(7, func(current, _ *Profile) error {
		current.Name = "changed"
		return want
	}); !errors.Is(err, want) {
		t.Fatalf("rollback error = %v", err)
	}
	after, err := os.ReadFile(profilePath(7))
	if err != nil || string(after) != string(before) {
		t.Fatalf("failed transaction changed profile: %v", err)
	}
}

func TestCreateAndDeleteShareMutationTransaction(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var group sync.WaitGroup
	results := make(chan bool, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			created, err := Create(9, &Profile{Name: "created"})
			if err != nil {
				t.Errorf("create: %v", err)
			}
			results <- created
		}()
	}
	group.Wait()
	close(results)
	createdCount := 0
	for created := range results {
		if created {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("successful creates = %d, want 1", createdCount)
	}

	loaded := make(chan struct{})
	release := make(chan struct{})
	mutated := make(chan error, 1)
	go func() {
		mutated <- Mutate(9, func(current, _ *Profile) error {
			close(loaded)
			<-release
			current.Name = "mutated"
			return nil
		})
	}()
	<-loaded
	deleted := make(chan error, 1)
	go func() { deleted <- Delete(9) }()
	select {
	case err := <-deleted:
		t.Fatalf("delete bypassed active mutation lock: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-mutated; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	if Exists(9) {
		t.Fatal("serialized delete did not remove profile")
	}
}

func TestFollowAllGamesInClearsOnlyTheNamedFieldInGamesThatSetIt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defaults := &Profile{Proton: ProtonSettings{EnableHDR: true}}
	if err := SaveDefault(defaults); err != nil {
		t.Fatal(err)
	}
	own := &Profile{Proton: ProtonSettings{EnableHDR: false, EnableWayland: true}}
	own.MarkOverride(FieldProtonEnableHDR)
	own.MarkOverride(FieldProtonEnableWayland)
	other := &Profile{GPU: GPUSettings{FanSpeed: 40}}
	other.MarkOverride(FieldGPUFanSpeed)
	inherits := &Profile{}
	for appID, stored := range map[uint64]*Profile{1: own, 2: other, 3: inherits} {
		if err := Save(appID, stored); err != nil {
			t.Fatal(err)
		}
	}
	entries := []OwnValueEntry{{AppID: 1, Name: "Own", Profile: own}, {AppID: 2, Name: "Other", Profile: other}, {AppID: 3, Name: "Inherits", Profile: inherits}, {AppID: 4, Name: "No profile"}}

	outcome := FollowAllGamesIn(entries, []string{FieldProtonEnableHDR})
	if !reflect.DeepEqual(outcome.Followed, []string{"Own"}) || len(outcome.Failed) != 0 {
		t.Fatalf("outcome = %+v, want only Own followed", outcome)
	}
	reloaded, err := Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.IsOverridden(FieldProtonEnableHDR) {
		t.Error("HDR is still the game's own value")
	}
	if !reloaded.IsOverridden(FieldProtonEnableWayland) || !reloaded.Proton.EnableWayland {
		t.Error("another field of the same game changed")
	}
	if untouched, _ := Load(2); !untouched.IsOverridden(FieldGPUFanSpeed) || untouched.GPU.FanSpeed != 40 {
		t.Error("a game that does not set HDR changed")
	}
}

func TestFollowAllGamesInKeepsTheOverridesKeyWhenTheLastValueGoes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	own := &Profile{}
	own.MarkOverride(FieldProtonEnableHDR)
	if err := Save(7, own); err != nil {
		t.Fatal(err)
	}
	FollowAllGamesIn([]OwnValueEntry{{AppID: 7, Name: "Only", Profile: own}}, []string{FieldProtonEnableHDR})
	raw, err := os.ReadFile(profilePath(7))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "overrides") {
		t.Fatalf("overrides key missing, the profile would reload as a legacy file:\n%s", raw)
	}
}

func TestFollowAllGamesInNamesAGameItCannotWriteAndStillChangesTheRest(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	good, broken := &Profile{}, &Profile{}
	good.MarkOverride(FieldProtonEnableHDR)
	broken.MarkOverride(FieldProtonEnableHDR)
	if err := Save(1, good); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath(2), []byte("overrides: [not, a, map"), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome := FollowAllGamesIn([]OwnValueEntry{{AppID: 1, Name: "Good", Profile: good}, {AppID: 2, Name: "Broken", Profile: broken}}, []string{FieldProtonEnableHDR})
	if !reflect.DeepEqual(outcome.Followed, []string{"Good"}) || !reflect.DeepEqual(outcome.Failed, []string{"Broken"}) || outcome.Errors["Broken"] == nil {
		t.Fatalf("outcome = %+v, want Good followed and Broken failed with an error", outcome)
	}
}
