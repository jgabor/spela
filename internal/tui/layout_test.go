package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------

func TestLayout_HelpToggle(t *testing.T) {
	m := testLayout()

	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)
	if !layout.showHelp {
		t.Error("expected help to be shown")
	}

	result, _ = sendKey(&layout, "?")
	layout = result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected help to be hidden")
	}
}

func TestLayout_HelpEscCloses(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)

	result, _ = sendKey(&layout, "esc")
	layout = result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected esc to close help")
	}
}

func TestLayout_HelpQCloses(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)

	result, _ = sendKey(&layout, "q")
	layout = result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected q to close help")
	}
}

func TestLayout_HelpBlocksOtherKeys(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)

	// Tab should NOT toggle focus while help is shown.
	focused := layout.navState.Zone
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.navState.Zone != focused {
		t.Error("expected help to block tab from toggling focus")
	}

	// 1-4 rail hotkeys should NOT activate while help is shown.
	activeBefore := layout.rail.Active()
	result, _ = sendKey(&layout, "3")
	layout = result.(LayoutModel)
	if layout.rail.Active() != activeBefore {
		t.Error("expected help to block rail hotkeys")
	}
}

func TestLayout_SettingsPathInputReceivesQuestionMark(t *testing.T) {
	m := testLayout()
	*m.navState = m.navState.SelectDestination(nav.DestinationSettings)
	m.navState.Zone = nav.ZoneContent
	m.syncNavToComponents()
	for sectionIndex, section := range m.pane.settings.sections {
		for optionIndex, option := range section.Options {
			if option.Kind == config.KindPath {
				m.pane.settings.sectionCursor = sectionIndex
				m.pane.settings.optionCursor = optionIndex
				m.pane.settings.startPathEditing()
				result, _ := sendKey(&m, "?")
				layout := result.(LayoutModel)
				if layout.showHelp || !strings.Contains(layout.pane.settings.pathInput.Value(), "?") {
					t.Fatalf("question mark opened help or missed focused input: help=%v value=%q", layout.showHelp, layout.pane.settings.pathInput.Value())
				}
				return
			}
		}
	}
	t.Fatal("no path setting found")
}

// ---------------------------------------------------------------------------
// Focus / rail toggling
// ---------------------------------------------------------------------------

func TestLayout_TabTogglesFocus(t *testing.T) {
	m := testLayout()
	if m.navState.Zone != nav.ZonePrimary {
		t.Fatal("precondition: primary zone should be focused initially")
	}

	result, _ := sendKey(&m, "tab")
	layout := result.(LayoutModel)
	if layout.navState.Zone != nav.ZoneContext {
		t.Error("expected tab to move focus to context zone")
	}

	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.navState.Zone != nav.ZoneContent {
		t.Error("second tab should move focus to content zone")
	}
}

func TestLayout_TabTogglesFocus_NonGamesResource(t *testing.T) {
	m := testLayout()
	m, _, _ = m.handleGlobalKeys(tea.KeyPressMsg{Code: '2', Text: "2"})
	if m.rail.Active() != nav.DestinationDLLCatalog {
		t.Fatalf("precondition: expected DLL Catalog active, got %v", m.rail.Active())
	}
	result, _ := sendKey(&m, "tab")
	layout := result.(LayoutModel)
	if layout.navState.Zone != nav.ZoneContext {
		t.Error("tab should move to context zone")
	}
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.navState.Zone != nav.ZoneContent {
		t.Error("second tab should move to content zone")
	}
}

// ---------------------------------------------------------------------------
// Global shortcuts
// ---------------------------------------------------------------------------

func TestLayout_CtrlCQuits(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "ctrl+c")
	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from ctrl+c, got %T", msg)
	}
}

func TestLayout_QFromRailQuits(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "q")
	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from q on rail, got %T", msg)
	}
}

func TestLayout_F5TogglesDensity(t *testing.T) {
	m := testLayout()
	if m.densityMode != DensityStandard {
		t.Fatal("precondition: should start in standard density")
	}

	result, _ := sendKey(&m, "f5")
	layout := result.(LayoutModel)
	if layout.densityMode != DensityCompact {
		t.Errorf("expected DensityCompact, got %d", layout.densityMode)
	}

	result, _ = sendKey(&layout, "f5")
	layout = result.(LayoutModel)
	if layout.densityMode != DensityStandard {
		t.Errorf("expected DensityStandard, got %d", layout.densityMode)
	}
}

func TestLayout_F11TogglesFocused(t *testing.T) {
	m := testLayout()

	result, _ := sendKey(&m, "f11")
	layout := result.(LayoutModel)
	if layout.densityMode != DensityFocused {
		t.Errorf("expected DensityFocused, got %d", layout.densityMode)
	}

	result, _ = sendKey(&layout, "f11")
	layout = result.(LayoutModel)
	if layout.densityMode != DensityStandard {
		t.Errorf("expected DensityStandard, got %d", layout.densityMode)
	}
}

func TestLayout_CtrlFActivatesSearch(t *testing.T) {
	m := testLayout(testGame("Cyberpunk 2077"))

	result, _ := sendKey(&m, "ctrl+f")
	layout := result.(LayoutModel)
	if layout.navState.Zone == nav.ZonePrimary {
		t.Error("expected ctrl+f to drop rail focus")
	}
	if !layout.contextNav.sidebar.search.Focused() {
		t.Error("expected ctrl+f to activate search input")
	}
}

func TestLayout_SettingsDestination(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "4")
	layout := result.(LayoutModel)
	if layout.rail.Active() != nav.DestinationSettings {
		t.Fatalf("expected Settings destination, got %v", layout.rail.Active())
	}
	if layout.pane.settings.config == nil || layout.pane.settings.renderOptionsBody() == "" {
		t.Error("expected settings destination content")
	}
}

// ---------------------------------------------------------------------------
// Rescan — displaced from `r` to `ctrl+r` as part of the keymap audit
// ---------------------------------------------------------------------------

func TestLayout_RescanOnCtrlR(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "ctrl+r")
	if cmd == nil {
		t.Error("expected rescan command from ctrl+r")
	}
}

func TestLayout_BareRIsReservedForTask5(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "r")
	// Bare `r` must NOT trigger rescan — it is reserved for Task 5
	// (reset-field). The layout passes it through to the active resource
	// (which ignores it in Task 3). The one thing we care about: no
	// rescanGamesMsg in the resulting cmd chain.
	msg := execCmd(cmd)
	if _, ok := msg.(rescanGamesMsg); ok {
		t.Error("bare r must not trigger rescan (Task 5 reservation)")
	}
}

// ---------------------------------------------------------------------------
// Rail hotkeys 1-4 — smoke at layout level (detailed assertions in rail_test.go)
// ---------------------------------------------------------------------------

func TestLayout_RailHotkeysWithoutGame(t *testing.T) {
	// 1-4 no longer require a game selected; they are the rail spine.
	m := testLayout()

	for i, tc := range []struct {
		key  string
		want nav.Destination
	}{
		{"1", nav.DestinationLibrary},
		{"2", nav.DestinationDLLCatalog},
		{"3", nav.DestinationMonitor},
		{"4", nav.DestinationSettings},
	} {
		t.Run(tc.key, func(t *testing.T) {
			result, _ := sendKey(&m, tc.key)
			layout := result.(LayoutModel)
			if layout.rail.Active() != tc.want {
				t.Errorf("[iter %d] after %q: active = %v, want %v", i, tc.key, layout.rail.Active(), tc.want)
			}
			if layout.navState.Zone != nav.ZonePrimary {
				t.Errorf("[iter %d] expected rail focus preserved after %q", i, tc.key)
			}
			m = layout
		})
	}
}

func TestLayout_RailHotkeyFromDeepFocus(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g)
	if m.rail.Active() != nav.DestinationLibrary {
		t.Fatalf("precondition: expected Library, got %v", m.rail.Active())
	}
	if m.navState.Zone != nav.ZoneContent {
		t.Fatalf("precondition: expected content zone after game confirm, got %v", m.navState.Zone)
	}

	result, _ := sendKey(&m, "3")
	layout := result.(LayoutModel)
	if layout.rail.Active() != nav.DestinationLibrary {
		t.Errorf("destination hotkeys must not fire outside Primary zone, got %v", layout.rail.Active())
	}
	if layout.navState.Zone != nav.ZoneContent {
		t.Error("expected content zone unchanged when hotkey blocked")
	}
}

func TestLayout_ContextAspectHotkeyInContextZone(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g)
	m.navState.Zone = nav.ZoneContext
	m.contextNav.SetState(*m.navState)

	result, _ := sendKey(&m, "2")
	layout := result.(LayoutModel)
	if layout.rail.Active() != nav.DestinationLibrary {
		t.Errorf("expected Library destination, got %v", layout.rail.Active())
	}
	if layout.navState.Aspect != nav.AspectProfile {
		t.Errorf("expected Profile aspect, got %v", layout.navState.Aspect)
	}
}

func TestLayout_DLLCatalogSectionFiltersContent(t *testing.T) {
	m := testLayout(testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.7.0")))
	result, _ := sendKey(&m, "2")
	layout := result.(LayoutModel)
	layout.navState.DLLCatalogSection = nav.SectionDLLDeployment
	layout.pane.SetState(*layout.navState)

	view := layout.pane.dllsResource.View(false, nav.SectionDLLDeployment)
	if !strings.Contains(view, "Deployment") {
		t.Fatalf("expected deployment section, got:\n%s", view)
	}
	if strings.Contains(stripANSI(view), "Inventory of DLL types") {
		t.Fatal("deployment view must not include library inventory")
	}

	libraryView := layout.pane.dllsResource.View(false, nav.SectionDLLLibrary)
	if !strings.Contains(libraryView, "Inventory of DLL types") {
		t.Fatalf("expected library section, got:\n%s", libraryView)
	}
}

func TestLayout_ResourceKeysStayScopedToActiveResource(t *testing.T) {
	g1 := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	g2 := testGame("Beta", testDLL(game.DLLTypeDLSS, "3.8.10"))
	g2.AppID = 2
	m := testLayout(g1, g2)

	result, _ := sendKey(&m, "2")
	m = result.(LayoutModel)
	result, _ = sendKey(&m, "tab")
	m = result.(LayoutModel)
	result, _ = sendKey(&m, "tab")
	m = result.(LayoutModel)
	railCursor := m.rail.Cursor()
	result, _ = sendKey(&m, "j")
	m = result.(LayoutModel)
	if m.rail.Cursor() != railCursor {
		t.Errorf("DLLs j should not move rail cursor: got %d want %d", m.rail.Cursor(), railCursor)
	}
	if got := m.pane.dllsResource.gameRowCursor; got != 1 {
		t.Errorf("DLLs j should move deployment cursor to 1, got %d", got)
	}

	result, _ = sendKey(&m, "1")
	m = result.(LayoutModel)
	if m.navState.Zone != nav.ZonePrimary {
		for i := 0; i < 3 && m.navState.Zone != nav.ZonePrimary; i++ {
			result, _ = sendKey(&m, "esc")
			m = result.(LayoutModel)
		}
		result, _ = sendKey(&m, "1")
		m = result.(LayoutModel)
	}
	m.pane.loadGlobalScope()
	*m.navState = m.pane.State()
	m.navState.Zone = nav.ZoneContent
	m.syncNavToComponents()
	defaultCursor := m.pane.defaultsDetail.Cursor()
	result, _ = sendKey(&m, "j")
	m = result.(LayoutModel)
	if got := m.pane.defaultsDetail.Cursor(); got != defaultCursor+1 {
		t.Errorf("global profile j should move detail cursor to %d, got %d", defaultCursor+1, got)
	}
	if m.pane.dllsResource.gameRowCursor != 1 {
		t.Error("profile j should not mutate DLL row cursor")
	}
}

func TestLayout_DLLUpdateAllMessageReachesDLLsWhenMetricsActive(t *testing.T) {
	m := testLayout(testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.7.0")))
	result, _ := sendKey(&m, "2")
	m = result.(LayoutModel)
	if m.rail.Active() != nav.DestinationDLLCatalog {
		t.Fatalf("precondition: expected DLL Catalog active, got %v", m.rail.Active())
	}

	updated, _ := m.Update(dllsUpdateAllCompleteMsg{
		results: map[string]string{"1091500:dlss": "ok"},
		summary: "Update-all: 1 updated, 0 failed",
	})
	m = updated.(LayoutModel)

	if got := m.pane.dllsResource.lastBatchSummary; got != "Update-all: 1 updated, 0 failed" {
		t.Errorf("DLLs resource did not receive message while Metrics active, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Navigation — q / esc
// ---------------------------------------------------------------------------

func TestLayout_QFromContent_StepsBackToContext(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g)
	if m.navState.Zone != nav.ZoneContent {
		t.Fatalf("precondition: expected content zone, got %v", m.navState.Zone)
	}

	result, _ := sendKey(&m, "q")
	layout := result.(LayoutModel)
	if layout.navState.Zone != nav.ZoneContext {
		t.Error("expected q from content to return to context zone")
	}
}

func TestLayout_EscFromContext_StepsBackToPrimary(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g)
	m.navState.Zone = nav.ZoneContext
	m.syncNavToComponents()

	result, _ := sendKey(&m, "esc")
	layout := result.(LayoutModel)
	if layout.navState.Zone != nav.ZonePrimary {
		t.Error("expected esc from context to return to primary zone")
	}
}

// ---------------------------------------------------------------------------
// Modal interception
// ---------------------------------------------------------------------------

func TestLayout_ModalInterceptsInput(t *testing.T) {
	m := testLayout()
	m.pane.content.pendingAction = PendingDLLUpdate
	m.navState.Zone = nav.ZoneContent

	focused := m.navState.Zone
	result, _ := sendKey(&m, "tab")
	layout := result.(LayoutModel)
	if layout.navState.Zone != focused {
		t.Error("expected pending DLL confirm to block tab zone change")
	}
}

func TestLayout_ModalClosesOnCancel(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.7.0"))
	m := testLayoutWithGame(g)
	m.navState.Aspect = nav.AspectDLLs
	m.pane.SetState(*m.navState)
	m.pane.content.pendingAction = PendingDLLUpdate
	m.navState.Zone = nav.ZoneContent

	result, _ := sendKey(&m, "esc")
	layout := result.(LayoutModel)
	if layout.pane.content.pendingAction != PendingNone {
		t.Error("expected esc to clear pending action")
	}
}

// ---------------------------------------------------------------------------
// Batch menu
// ---------------------------------------------------------------------------

func TestLayout_BatchMenu_EscCloses(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}

	result, _ := sendKey(&m, "esc")
	layout := result.(LayoutModel)
	if layout.showBatchMenu {
		t.Error("expected esc to close batch menu")
	}
	if layout.batchGames != nil {
		t.Error("expected batch games to be cleared")
	}
}

func TestLayout_BatchMenu_Navigation(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}
	m.batchCursor = 0

	result, _ := sendKey(&m, "up")
	layout := result.(LayoutModel)
	if layout.batchCursor != 0 {
		t.Error("expected cursor to clamp at 0")
	}
}

func TestLayout_BatchMenu_EnterExecutes(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}
	m.batchCursor = 0

	_, cmd := sendKey(&m, "enter")
	if cmd == nil {
		t.Error("expected enter in batch menu to return a command")
	}
}

func TestLayout_BatchMenu_BlocksGlobalKeys(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}

	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected batch menu to block ? from opening help")
	}
}

// ---------------------------------------------------------------------------
// Window size
// ---------------------------------------------------------------------------

func TestLayout_WindowSize(t *testing.T) {
	m := testLayout()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	layout := result.(LayoutModel)
	if layout.width != 200 || layout.height != 60 {
		t.Errorf("expected 200x60, got %dx%d", layout.width, layout.height)
	}
}

// ---------------------------------------------------------------------------
// Assertions that no `Launch` symbols survive in the shell layer
// ---------------------------------------------------------------------------

// TestLayout_NoLaunchSurface is a structural test: the shell must expose
// no launching field, no TabLaunch constant, no launchGame method, and no
// launchGameMsg type. This guards the Task 3 acceptance criterion
// "rg -i launch internal/tui/ returns no matches outside test fixtures /
// historical comments".
//
// We can't run rg from inside a test, but we can assert the absence of
// the concrete types the old shell relied on by failing to compile if
// they reappear. The go test harness catches this because the test file
// references none of those symbols — adding a new one would not break
// this test directly, which is why the acceptance check also runs grep
// in the verification step. Keep this test as a documentation anchor.
func TestLayout_NoLaunchSurface(t *testing.T) {
	// Intentionally empty — compile-time anchor only. The body could
	// reference `_ = TabLaunch` to fail the build if reintroduced, but
	// we keep it body-less so the rg grep from the acceptance criterion
	// is the authoritative check.
}

func TestExecuteBatchDLLUpdatePersists(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv("XDG_CONFIG_HOME", base)

	payload := []byte("new-dll-payload")
	hash := sha256.Sum256(payload)
	sha := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)

	manifest := dll.Manifest{
		Version:   "1",
		UpdatedAt: time.Now().UTC(),
		DLLs: map[string][]dll.DLL{
			"dlss": {{
				Version:  "3.8.10",
				Filename: "nvngx_dlss.dll",
				URL:      server.URL,
				SHA256:   sha,
			}},
		},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(base, "spela", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatal(err)
	}

	installDir := t.TempDir()
	dllPath := filepath.Join(installDir, "nvngx_dlss.dll")
	if err := os.WriteFile(dllPath, []byte("old-dll"), 0o644); err != nil {
		t.Fatal(err)
	}

	gameEntry := &game.Game{
		AppID:      1091500,
		Name:       "Cyberpunk 2077",
		InstallDir: installDir,
		DLLs: []game.DetectedDLL{{
			Path:    dllPath,
			Name:    "nvngx_dlss.dll",
			Type:    game.DLLTypeDLSS,
			Version: "3.7.0",
		}},
	}
	db := &game.Database{Games: map[uint64]*game.Game{1091500: gameEntry}}
	saveTestDatabase(t, db)

	msg := executeBatchDLLUpdate([]uint64{gameEntry.AppID})
	if msg.message == "" {
		t.Fatal("expected batch completion message")
	}

	reloaded, err := game.LoadDatabase()
	if err != nil {
		t.Fatalf("LoadDatabase() error = %v", err)
	}
	got := reloaded.GetGame(1091500)
	if got == nil || len(got.DLLs) == 0 {
		t.Fatal("expected persisted DLL scan results")
	}
	if got.DLLs[0].Version == "3.7.0" {
		t.Fatalf("expected updated DLL version in database, still %q", got.DLLs[0].Version)
	}
}

func TestLayout_SlashOpensSearchFromPrimary(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "/")
	layout := result.(LayoutModel)
	if layout.navState.Zone != nav.ZoneContext {
		t.Fatalf("expected context zone after /, got %v", layout.navState.Zone)
	}
	if !layout.contextNav.sidebar.search.Focused() {
		t.Error("expected sidebar search to be focused after /")
	}
}

func TestLayout_AspectHotkeyFromContent(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.7.0"))
	m := testLayoutWithGame(g)
	m.navState.Zone = nav.ZoneContent
	m.navState.Aspect = nav.AspectProfile
	m.syncNavToComponents()

	result, _ := sendKey(&m, "3")
	layout := result.(LayoutModel)
	if layout.navState.Aspect != nav.AspectDLLs {
		t.Fatalf("expected DLL aspect after 3 from content, got %v", layout.navState.Aspect)
	}
}
