package tui

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/profile"
)

func holdProfileMutationLock(t *testing.T, configHome string) func() {
	t.Helper()
	directory := filepath.Join(configHome, "spela", "profiles")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(directory, ".mutation.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("hold mutation lock: %v", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		t.Fatalf("hold mutation lock: %v", err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}
}

func requireDeferredSave(t *testing.T, start func() tea.Cmd, unlock func()) profileSaveMsg {
	t.Helper()
	commandReady := make(chan tea.Cmd, 1)
	go func() { commandReady <- start() }()
	var command tea.Cmd
	select {
	case command = <-commandReady:
	case <-time.After(200 * time.Millisecond):
		unlock()
		t.Fatal("starting save blocked on the profile filesystem lock")
	}
	if command == nil {
		unlock()
		t.Fatal("save did not return a command")
	}
	completed := make(chan tea.Msg, 1)
	go func() { completed <- command() }()
	select {
	case <-completed:
		unlock()
		t.Fatal("save command completed while mutation lock was held")
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	select {
	case message := <-completed:
		return message.(profileSaveMsg)
	case <-time.After(time.Second):
		t.Fatal("save did not complete after mutation lock release")
	}
	return profileSaveMsg{}
}

func TestProfilePersistenceStartsAsynchronously(t *testing.T) {
	t.Run("game", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		content := testContent(testGame("Cyberpunk 2077"))
		if err := content.detail.RawProfile().Set(profile.FieldProtonEnableHDR, true); err != nil {
			t.Fatal(err)
		}
		message := requireDeferredSave(t, content.saveResolvedProfile, holdProfileMutationLock(t, configHome))
		if message.err != nil {
			t.Fatalf("save failed: %v", message.err)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		pane := newResourcePane(NewStyles(DefaultTheme, true), ContentModel{})
		pane.persistedDefaults = (&profile.Profile{}).Clone()
		pane.defaultsDetail = NewRootDetail(pane.styles, &profile.Profile{})
		if err := pane.defaultsDetail.RawProfile().Set(profile.FieldProtonEnableHDR, true); err != nil {
			t.Fatal(err)
		}
		message := requireDeferredSave(t, pane.saveDefaultProfile, holdProfileMutationLock(t, configHome))
		if message.err != nil {
			t.Fatalf("save failed: %v", message.err)
		}
	})
}

func TestContentProfileSaveCoalescesLatestIntent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	entry := testGame("Cyberpunk 2077")
	content := testContent(entry)
	raw := content.detail.RawProfile()
	if err := raw.Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	first := content.saveResolvedProfile()
	if profile.Exists(entry.AppID) {
		t.Fatal("profile was written before the save command ran")
	}
	if err := raw.Reset(profile.FieldProtonEnableHDR); err != nil {
		t.Fatal(err)
	}
	if command := content.saveResolvedProfile(); command != nil {
		t.Fatal("second save ran concurrently instead of being coalesced")
	}
	firstMessage := first().(profileSaveMsg)
	content, second, _ := content.updateContentMessage(firstMessage)
	stored, _ := profile.Load(entry.AppID)
	if !stored.IsOverridden(profile.FieldProtonEnableHDR) || second == nil {
		t.Fatal("first intent was not persisted before the pending save started")
	}
	content, _, _ = content.updateContentMessage(second())
	stored, err := profile.Load(entry.AppID)
	if err != nil || stored.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatalf("stored latest game intent = %+v, %v", stored, err)
	}
}

func TestDefaultProfileSaveCoalescesLatestIntent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	pane := newResourcePane(NewStyles(DefaultTheme, true), ContentModel{})
	pane.persistedDefaults = (&profile.Profile{}).Clone()
	pane.defaultsDetail = NewRootDetail(pane.styles, &profile.Profile{})
	raw := pane.defaultsDetail.RawProfile()
	if err := raw.Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	first := pane.saveDefaultProfile()
	if err := raw.Reset(profile.FieldProtonEnableHDR); err != nil {
		t.Fatal(err)
	}
	if command := pane.saveDefaultProfile(); command != nil {
		t.Fatal("second default save ran concurrently instead of being coalesced")
	}
	second := pane.completeDefaultSave(first().(profileSaveMsg))
	if second == nil {
		t.Fatal("pending default save was not started")
	}
	pane.completeDefaultSave(second().(profileSaveMsg))
	stored, err := profile.LoadDefault()
	if err != nil || stored.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatalf("stored latest default intent = %+v, %v", stored, err)
	}
}

func TestPendingProfileSaveContinuesFromBaselineAfterFailure(t *testing.T) {
	badConfigHome := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(badConfigHome, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", badConfigHome)
	entry := testGame("Cyberpunk 2077")
	content := testContent(entry)
	raw := content.detail.RawProfile()
	if err := raw.Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	first := content.saveResolvedProfile()
	if err := raw.Set(profile.FieldProtonEnableWayland, true); err != nil {
		t.Fatal(err)
	}
	content.saveResolvedProfile()
	failed := first().(profileSaveMsg)
	if failed.err == nil {
		t.Fatal("first save unexpectedly succeeded")
	}
	validConfigHome := t.TempDir()
	if err := os.Setenv("XDG_CONFIG_HOME", validConfigHome); err != nil {
		t.Fatal(err)
	}
	content, retry, _ := content.updateContentMessage(failed)
	if retry == nil {
		t.Fatal("failed save discarded pending intent")
	}
	content, _, _ = content.updateContentMessage(retry())
	stored, err := profile.Load(entry.AppID)
	if err != nil || !stored.IsOverridden(profile.FieldProtonEnableHDR) || !stored.IsOverridden(profile.FieldProtonEnableWayland) {
		t.Fatalf("stored intent after recovery = %+v, %v", stored, err)
	}
}

func TestGameLoadAndPinUsePendingDefaultIntent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gameB := testGame("Hades")
	gameB.AppID = 1145360
	pane := newResourcePane(NewStyles(DefaultTheme, true), testContent(testGame("Cyberpunk 2077")))
	pane.persistedDefaults = (&profile.Profile{}).Clone()
	pane.defaultsDetail = NewRootDetail(pane.styles, &profile.Profile{})
	if err := pane.defaultsDetail.RawProfile().Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	defaultSave := pane.saveDefaultProfile()
	pane.content = pane.content.SetGame(gameB)
	if !pane.content.detail.resolved.Proton.EnableHDR || pane.content.detail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatal("game did not display pending inherited HDR value")
	}
	if changed, err := pane.content.detail.PinFocused(); err != nil || !changed {
		t.Fatalf("pin pending inherited HDR = changed %v, err %v", changed, err)
	}
	if command := pane.content.saveResolvedProfile(); command != nil {
		t.Fatal("game pin ran concurrently with pending defaults")
	}
	gameSave := pane.completeDefaultSave(defaultSave().(profileSaveMsg))
	pane.content, _, _ = pane.content.updateContentMessage(gameSave())
	stored, err := profile.Load(gameB.AppID)
	if err != nil || !stored.IsOverridden(profile.FieldProtonEnableHDR) || !stored.Proton.EnableHDR {
		t.Fatalf("pinned pending inherited HDR = %+v, %v", stored, err)
	}
}

func TestProfileSaveQueuePreservesInterleavedTargets(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gameA := testGame("Cyberpunk 2077")
	gameB := testGame("Hades")
	gameB.AppID = 1145360
	pane := newResourcePane(NewStyles(DefaultTheme, true), testContent(gameA))
	pane.persistedDefaults = (&profile.Profile{}).Clone()
	pane.defaultsDetail = NewRootDetail(pane.styles, &profile.Profile{})
	if err := pane.content.detail.RawProfile().Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	gameASave := pane.content.saveResolvedProfile()
	if err := pane.defaultsDetail.RawProfile().Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	if command := pane.saveDefaultProfile(); command != nil {
		t.Fatal("default save ran concurrently with game save")
	}
	pane.content = pane.content.SetGame(gameB)
	if err := pane.content.detail.RawProfile().Set(profile.FieldProtonEnableWayland, true); err != nil {
		t.Fatal(err)
	}
	if command := pane.content.saveResolvedProfile(); command != nil {
		t.Fatal("game B save ran concurrently with game A save")
	}
	updatedContent, defaultSave, _ := pane.content.updateContentMessage(gameASave())
	pane.content = updatedContent
	if defaultSave == nil {
		t.Fatal("pending default save did not follow game save")
	}
	gameBSave := pane.completeDefaultSave(defaultSave().(profileSaveMsg))
	if gameBSave == nil {
		t.Fatal("game B save was dropped behind defaults")
	}
	pane.content, _, _ = pane.content.updateContentMessage(gameBSave())
	storedA, gameAErr := profile.Load(gameA.AppID)
	storedB, gameBErr := profile.Load(gameB.AppID)
	storedDefaults, defaultErr := profile.LoadDefault()
	if gameAErr != nil || gameBErr != nil || defaultErr != nil || !storedA.IsOverridden(profile.FieldProtonEnableHDR) ||
		!storedDefaults.IsOverridden(profile.FieldProtonEnableHDR) || !storedB.IsOverridden(profile.FieldProtonEnableWayland) {
		t.Fatalf("serialized stores = A %+v (%v), defaults %+v (%v), B %+v (%v)", storedA, gameAErr, storedDefaults, defaultErr, storedB, gameBErr)
	}
}

func TestGameReloadUsesQueuedIntentAsNextSaveBaseline(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gameA := testGame("Cyberpunk 2077")
	gameB := testGame("Hades")
	gameB.AppID = 1145360
	content := testContent(gameA)
	if err := content.detail.RawProfile().Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	first := content.saveResolvedProfile()
	content = content.SetGame(gameB).SetGame(gameA)
	if !content.detail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatal("game reload discarded active save intent")
	}
	if err := content.detail.RawProfile().Set(profile.FieldProtonEnableWayland, true); err != nil {
		t.Fatal(err)
	}
	content.saveResolvedProfile()
	content, second, _ := content.updateContentMessage(first())
	content, _, _ = content.updateContentMessage(second())
	stored, err := profile.Load(gameA.AppID)
	if err != nil || !stored.IsOverridden(profile.FieldProtonEnableHDR) || !stored.IsOverridden(profile.FieldProtonEnableWayland) {
		t.Fatalf("stored game after reload and later edit = %+v, %v", stored, err)
	}
}

func TestDefaultsReloadUsesQueuedIntentAsNextSaveBaseline(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	pane := newResourcePane(NewStyles(DefaultTheme, true), ContentModel{})
	pane.persistedDefaults = (&profile.Profile{}).Clone()
	pane.defaultsDetail = NewRootDetail(pane.styles, &profile.Profile{})
	if err := pane.defaultsDetail.RawProfile().Set(profile.FieldProtonEnableHDR, true); err != nil {
		t.Fatal(err)
	}
	first := pane.saveDefaultProfile()
	pane.refreshDefaultsDetail()
	if !pane.defaultsDetail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatal("defaults reload discarded active save intent")
	}
	if err := pane.defaultsDetail.RawProfile().Set(profile.FieldProtonEnableWayland, true); err != nil {
		t.Fatal(err)
	}
	pane.saveDefaultProfile()
	second := pane.completeDefaultSave(first().(profileSaveMsg))
	pane.completeDefaultSave(second().(profileSaveMsg))
	stored, err := profile.LoadDefault()
	if err != nil || !stored.IsOverridden(profile.FieldProtonEnableHDR) || !stored.IsOverridden(profile.FieldProtonEnableWayland) {
		t.Fatalf("stored defaults after reload and later edit = %+v, %v", stored, err)
	}
}
