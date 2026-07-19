package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

func TestContentSupportedProfileDLLAndInstallViews(t *testing.T) {
	entry := testGame(
		"Cyberpunk 2077",
		testDLL(game.DLLTypeDLSS, "3.8.10"),
		game.DetectedDLL{Name: "libxess.dll", Type: game.DLLTypeXeSS},
	)
	content := testContent(entry)
	content.SetSize(80, 30)
	if !content.HasGameSelection() || content.HasModalOpen() {
		t.Fatal("selected game content state is inconsistent")
	}

	content.usingDefaultProfile = true
	content.hasUpdates = true
	content.hasBackup = true
	for _, pending := range []PendingAction{PendingNone, PendingDLLUpdate, PendingDLLRestore} {
		content.pendingAction = pending
		view := stripANSI(content.ViewDLLAspect())
		if !strings.Contains(view, "DLL versions") || !strings.Contains(view, "3.8.10") {
			t.Fatalf("DLL view for pending state %d:\n%s", pending, view)
		}
	}
	content.pendingAction = PendingNone
	content.dllOperating = true
	content.dllOperatingLabel = "Updating DLLs"
	if view := stripANSI(content.ViewDLLAspect()); !strings.Contains(view, "Updating DLLs") {
		t.Fatalf("operating DLL view:\n%s", view)
	}

	installStates := []struct {
		state    DLLInstallState
		prepare  func(*ContentModel)
		fragment string
	}{
		{DLLInstallSelectType, func(m *ContentModel) { m.dllTypes = nil }, "Loading"},
		{DLLInstallSelectType, func(m *ContentModel) { m.dllTypes = []string{"dlss", "xess"}; m.dllTypeCursor = 1 }, "XESS"},
		{DLLInstallSelectVersion, func(m *ContentModel) { m.selectedDLLType = "dlss"; m.dllVersions = nil; m.dllVersionsLoaded = false }, "Loading"},
		{DLLInstallSelectVersion, func(m *ContentModel) { m.dllVersions = nil; m.dllVersionsLoaded = true }, "No versions available"},
		{DLLInstallSelectVersion, func(m *ContentModel) {
			m.dllVersions = []dll.DLL{{Version: "3.8.10"}, {Version: "3.7.0"}}
			m.dllVersionCursor = 1
		}, "latest"},
		{DLLInstallDownloading, func(*ContentModel) {}, "Installing DLL"},
	}
	for _, test := range installStates {
		content.dllInstallState = test.state
		test.prepare(&content)
		view := stripANSI(content.ViewDLLAspect())
		if !strings.Contains(view, test.fragment) || !content.HasModalOpen() {
			t.Errorf("install state %d missing %q:\n%s", test.state, test.fragment, view)
		}
	}

	content.dllInstallState = DLLInstallNone
	content.dlssPresetModal.Open(profile.DLSSPresetK)
	if view := stripANSI(content.ViewProfileAspect()); !strings.Contains(view, "Select DLSS preset") {
		t.Fatalf("profile modal view:\n%s", view)
	}
	empty := testContent(nil)
	if !strings.Contains(stripANSI(empty.ViewProfileAspect()), "Select a game") || !strings.Contains(stripANSI(empty.ViewDLLAspect()), "Select a game") {
		t.Fatal("empty content did not retain selection guidance")
	}
}

func TestContentInstallStateMachineSupportedMessagesAndKeys(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_CONFIG_HOME", state+"/config")
	t.Setenv("XDG_CACHE_HOME", state+"/cache")
	t.Setenv("XDG_DATA_HOME", state+"/data")
	content := testContent(testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.7.0")))
	content.dllInstallState = DLLInstallSelectType
	content, _ = content.Update(dllTypesLoadedMsg{types: []string{"dlss", "xess"}})
	if len(content.dllTypes) != 2 {
		t.Fatalf("DLL types = %v", content.dllTypes)
	}
	content, _ = content.Update(keyMsg("down"))
	content, command := content.Update(keyMsg("enter"))
	if content.selectedDLLType != "xess" || content.dllInstallState != DLLInstallSelectVersion || command == nil {
		t.Fatalf("type selection = type %q, state %d, command %v", content.selectedDLLType, content.dllInstallState, command)
	}
	content, _ = content.Update(dllVersionsLoadedMsg{versions: []dll.DLL{{Version: "1.3.1"}, {Version: "1.2.0"}}})
	content, _ = content.Update(keyMsg("down"))
	content, _ = content.Update(keyMsg("up"))
	if content.dllVersionCursor != 0 || !content.dllVersionsLoaded {
		t.Fatalf("version navigation = cursor %d loaded %v", content.dllVersionCursor, content.dllVersionsLoaded)
	}
	content, _ = content.Update(dllInstallMsg{err: errors.New("download failed")})
	if content.dllInstallState != DLLInstallNone || content.dllOperating {
		t.Fatal("failed install did not close and clear operating state")
	}
	content.dllInstallState = DLLInstallDownloading
	content.dllOperating = true
	content, _ = content.Update(dllInstallMsg{result: dll.Result{Outcome: dll.OutcomeChanged, Game: testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.8.10"))}})
	if content.game.DLLs[0].Version != "3.8.10" || content.dllInstallState != DLLInstallNone {
		t.Fatalf("successful install state = %+v", content)
	}
	partialGame := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.8.11"))
	partialResult := dll.Result{Outcome: dll.OutcomeChanged, FilesChanged: true, Game: partialGame}
	partialErr := &dll.PartialFailure{Result: partialResult, Stage: dll.StageSaving, Err: errors.New("disk full")}
	content.dllInstallState = DLLInstallDownloading
	content, _ = content.Update(dllInstallMsg{result: partialResult, err: partialErr})
	if content.game.DLLs[0].Version != "3.8.11" {
		t.Fatalf("partial install did not apply scanned metadata: %+v", content.game.DLLs)
	}
	partialGame = testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.8.12"))
	partialResult.Game = partialGame
	partialErr.Result = partialResult
	content, _ = content.Update(dllRestoreMsg{result: partialResult, err: partialErr})
	if content.game.DLLs[0].Version != "3.8.12" {
		t.Fatalf("partial restore did not apply scanned metadata: %+v", content.game.DLLs)
	}

	content.dllInstallState = DLLInstallSelectType
	content, _ = content.Update(keyMsg("q"))
	if content.HasModalOpen() {
		t.Fatal("install cancel did not close modal")
	}
	if message := execCmd(content.restoreDLLs()); message == nil {
		t.Fatal("restore command did not report its supported missing-backup outcome")
	}
	if message := execCmd(testContent(nil).updateDLLs()); message == nil {
		t.Fatal("update command did not report its supported no-selection outcome")
	}
}

func TestContentDLLCommandsCompleteAgainstIsolatedGameFiles(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", filepath.Join(state, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(state, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(state, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(state, "data"))
	installDirectory := filepath.Join(state, "game")
	if err := os.MkdirAll(installDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
	if err := os.WriteFile(targetPath, []byte("old DLL"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := []byte("new DLL")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	manifest := &dll.Manifest{
		Version: "1", UpdatedAt: time.Now(),
		DLLs: map[string][]dll.DLL{
			"dlss": {{Version: "3.9.0", Filename: "nvngx_dlss.dll", URL: server.URL, SHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}},
		},
	}
	if err := dll.SaveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	entry := &game.Game{
		AppID: 1091500, Name: "Cyberpunk 2077", InstallDir: installDirectory,
		DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: targetPath, Type: game.DLLTypeDLSS, Version: "3.8.10"}},
	}
	content := testContent(entry)
	saveTestDatabase(t, content.database)
	if message, ok := execCmd(content.LoadDLLUpdates()).(dllUpdatesCheckedMsg); !ok || message.err != nil || !message.hasUpdates {
		t.Fatalf("DLL update check = %#v", message)
	}
	if message, ok := execCmd(content.loadDLLTypes()).(dllTypesLoadedMsg); !ok || len(message.types) != 1 || message.types[0] != "dlss" {
		t.Fatalf("DLL type load = %#v", message)
	}
	content.selectedDLLType = "dlss"
	if message, ok := execCmd(content.loadDLLVersions()).(dllVersionsLoadedMsg); !ok || len(message.versions) != 1 || message.versions[0].Version != "3.9.0" {
		t.Fatalf("DLL version load = %#v", message)
	}
	message, ok := execCmd(content.updateDLLs()).(dllUpdateMsg)
	if !ok || message.err != nil || message.batch.Updated != 1 {
		t.Fatalf("DLL update = %#v", message)
	}
	if data, err := os.ReadFile(targetPath); err != nil || string(data) != string(payload) {
		t.Fatalf("updated DLL = %q, %v", data, err)
	}
	if restore, ok := execCmd(content.restoreDLLs()).(dllRestoreMsg); !ok || restore.err != nil || restore.result.Outcome != dll.OutcomeChanged {
		t.Fatalf("DLL restore = %#v", restore)
	}
	if data, err := os.ReadFile(targetPath); err != nil || string(data) != "old DLL" {
		t.Fatalf("restored DLL = %q, %v", data, err)
	}

	content.selectedDLLType = "dlss"
	content.dllVersions = manifest.DLLs["dlss"]
	content.dllVersionCursor = 0
	install, ok := execCmd(content.installSelectedDLL()).(dllInstallMsg)
	if !ok || install.err != nil || install.result.Outcome != dll.OutcomeChanged {
		t.Fatalf("DLL install = %#v", install)
	}
	if data, err := os.ReadFile(targetPath); err != nil || string(data) != string(payload) {
		t.Fatalf("installed DLL = %q, %v", data, err)
	}
}

func TestContentSupportedMessageAndKeyRouting(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", filepath.Join(state, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(state, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(state, "cache"))
	entry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	content := testContent(entry)
	content.SetSize(80, 30)

	content, command := content.Update(openDLSSPresetModalMsg{currentPreset: profile.DLSSPresetK})
	if !content.dlssPresetModal.Visible() || command != nil {
		t.Fatal("open preset message did not open blocking modal")
	}
	content.dlssPresetModal.visible = false
	content, command = content.Update(dlssPresetSelectedMsg{preset: profile.DLSSPresetL})
	if content.detail.RawProfile().DLSS.SRPreset != profile.DLSSPresetL || command == nil {
		t.Fatal("preset selection did not mutate raw profile and schedule save")
	}
	if message, ok := command().(profileSaveMsg); !ok || !message.success {
		t.Fatalf("profile save command = %#v", message)
	}
	if _, command = content.Update(dlssPresetCancelledMsg{}); command != nil {
		t.Fatal("preset cancellation unexpectedly scheduled work")
	}

	content.dllOperating = true
	content, command = content.Update(dllUpdateMsg{err: errors.New("offline")})
	if content.dllOperating || command == nil {
		t.Fatal("DLL update error did not clear operation and recheck update state")
	}
	content.dllOperating = true
	content, _ = content.Update(dllRestoreMsg{result: dll.Result{Outcome: dll.OutcomeChanged}})
	if content.dllOperating || content.hasBackup {
		t.Fatal("DLL restore success did not update operation/backup state")
	}
	content.hasUpdates = false
	content, command = content.Update(keyMsg("u"))
	if command == nil {
		t.Fatal("up-to-date key did not return user notice")
	}
	if notice, ok := command().(contentNoticeMsg); !ok || !strings.Contains(notice.text, "up to date") {
		t.Fatalf("up-to-date notice = %#v", notice)
	}
	content.dllOperating = false
	content, command = content.Update(keyMsg("i"))
	if content.dllInstallState != DLLInstallSelectType || command == nil {
		t.Fatal("install key did not begin install wizard")
	}
	content.dllInstallState = DLLInstallNone
	content.dllOperating = false
	content.hasBackup = true
	content.confirmDestructive = false
	content, command = content.Update(keyMsg("ctrl+shift+r"))
	if !content.dllOperating || command == nil {
		t.Fatal("restore key did not start direct supported restoration")
	}
}

func TestContentFirstGameProfileSaveClearsInheritedBannerAndRetainsFocus(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HOME", filepath.Join(stateRoot, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(stateRoot, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(stateRoot, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(stateRoot, "cache"))

	defaults := &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}
	defaults.MarkOverride(profile.FieldProtonEnableHDR)
	services := testServices()
	services.LoadProfile = profile.Load
	services.LoadDefaultProfile = func() (*profile.Profile, error) { return defaults, nil }
	entry := testGame("Cyberpunk 2077")
	content := NewContent(NewStyles(DefaultTheme, true), true, services).SetGame(entry)
	content.SetSize(100, 30)
	wantField := content.detail.FocusedField()
	if !content.usingDefaultProfile || !strings.Contains(stripANSI(content.ViewProfileAspect()), "Using default profile values") {
		t.Fatal("profile-free game did not begin with inherited-only banner")
	}

	mutated, saveCommand := content.Update(keyMsg("p"))
	if saveCommand == nil {
		t.Fatal("pin returned no save command")
	}
	message, ok := saveCommand().(profileSaveMsg)
	if !ok || !message.success || message.err != nil || message.appID != entry.AppID {
		t.Fatalf("pin save result = %#v", message)
	}
	persisted, err := profile.Load(entry.AppID)
	if err != nil {
		t.Fatalf("load persisted game profile: %v", err)
	}
	if persisted == nil || !persisted.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatalf("persisted game profile = %#v", persisted)
	}

	routed, command := mutated.Update(message)
	if command != nil {
		t.Fatal("profile save result unexpectedly scheduled more work")
	}
	if routed.usingDefaultProfile || strings.Contains(stripANSI(routed.ViewProfileAspect()), "Using default profile values") {
		t.Fatal("successful first game-profile save retained inherited-only banner")
	}
	if got := routed.detail.FocusedField(); got != wantField {
		t.Fatalf("focused field after save = %q, want %q", got, wantField)
	}
}
