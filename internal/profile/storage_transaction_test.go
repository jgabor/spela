package profile

import (
	"errors"
	"os"
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
