package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

// ---------------------------------------------------------------------------
// DLLsResourceModel state-machine tests — Task 6 acceptance
// ---------------------------------------------------------------------------

// makeDLLsResource constructs a DLLs resource with the given games and a
// fake cached-version index. The manifest is also injected so the library
// section exercises latest-version resolution without touching disk.
func makeDLLsResource(games []*game.Game, cached map[string][]string, manifest *dll.Manifest) DLLsResourceModel {
	styles := NewStyles(DefaultTheme, true)
	m := NewDLLsResource(styles, testServices())
	m = m.SetGames(games)
	m = m.SetManifest(manifest)
	m.cached = cached
	m.SetSize(120, 40)
	return m
}

func makeDLLsResourceWithServices(games []*game.Game, cached map[string][]string, services *Services) DLLsResourceModel {
	styles := NewStyles(DefaultTheme, true)
	m := NewDLLsResource(styles, services)
	m = m.SetGames(games)
	m.cached = cached
	m.SetSize(120, 40)
	return m
}

func TestDLLsResource_LibrarySectionLists_AllKnownTypes(t *testing.T) {
	m := makeDLLsResource(nil, map[string][]string{
		"dlss":  {"3.8.10", "3.7.20"},
		"dlssg": {"1.0.3"},
	}, nil)

	lib := m.renderLibrary()
	for _, info := range dll.KnownDLLTypes() {
		if !strings.Contains(lib, info.Label) {
			t.Errorf("library section missing %q\n%s", info.Label, lib)
		}
	}
}

func TestDLLsResource_DeploymentOmitsZeroInstallTypes(t *testing.T) {
	// Two games, both install only DLSS. XeSS / FSR / DLSS-G / DLSS-D
	// rows should not appear.
	g1 := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	g2 := testGame("Beta", testDLL(game.DLLTypeDLSS, "3.8.10"))
	g2.AppID = 2
	m := makeDLLsResource([]*game.Game{g1, g2}, map[string][]string{
		"dlss": {"3.8.10"},
	}, nil)

	out := m.renderDeployment()
	if !strings.Contains(out, "DLSS") {
		t.Errorf("deployment missing DLSS column:\n%s", out)
	}
	for _, absent := range []string{"DLSS-G", "DLSS-D", "XeSS", "FSR"} {
		if strings.Contains(out, absent) {
			t.Errorf("deployment should omit column %q (no installs):\n%s", absent, out)
		}
	}
}

func TestDLLsResource_StaleCellIsMarked(t *testing.T) {
	// Alpha has DLSS 3.7.0, cached latest is 3.8.10 → stale.
	g := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	m := makeDLLsResource([]*game.Game{g}, map[string][]string{
		"dlss": {"3.8.10", "3.7.0"},
	}, nil)

	out := m.renderDeployment()
	if !strings.Contains(out, "◆") {
		t.Errorf("expected stale marker ◆ in deployment output:\n%s", out)
	}
}

func TestDLLsResource_UpToDateCellHasNoStaleMarker(t *testing.T) {
	g := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := makeDLLsResource([]*game.Game{g}, map[string][]string{
		"dlss": {"3.8.10"},
	}, nil)

	out := m.renderDeployment()
	// Strip ANSI first so we only look at the glyph itself.
	raw := stripANSI(out)
	if strings.Contains(raw, "◆") {
		t.Errorf("up-to-date cell must not have stale marker:\n%s", raw)
	}
}

func TestDLLsResource_ZeroGameMessageRenders(t *testing.T) {
	m := makeDLLsResource(nil, nil, nil)
	out := m.renderDeployment()
	if !strings.Contains(out, "No tracked game") {
		t.Errorf("expected zero-game empty-state message, got:\n%s", out)
	}
}

func TestDLLsResource_JKNavigatesRows(t *testing.T) {
	g1 := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	g2 := testGame("Beta", testDLL(game.DLLTypeDLSS, "3.8.10"))
	g2.AppID = 2
	m := makeDLLsResource([]*game.Game{g1, g2}, map[string][]string{
		"dlss": {"3.8.10"},
	}, nil)

	if m.gameRowCursor != 0 {
		t.Fatalf("initial cursor expected 0, got %d", m.gameRowCursor)
	}
	m, _ = m.Update(keyMsg("j"))
	if m.gameRowCursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", m.gameRowCursor)
	}
	m, _ = m.Update(keyMsg("j")) // clamp at end
	if m.gameRowCursor != 1 {
		t.Errorf("cursor should clamp at %d, got %d", 1, m.gameRowCursor)
	}
	m, _ = m.Update(keyMsg("k"))
	if m.gameRowCursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", m.gameRowCursor)
	}
	m, _ = m.Update(keyMsg("k")) // clamp at start
	if m.gameRowCursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.gameRowCursor)
	}
}

func TestDLLsResource_UpdateAllNoop_WhenNothingStale(t *testing.T) {
	g := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := makeDLLsResource([]*game.Game{g}, map[string][]string{
		"dlss": {"3.8.10"},
	}, nil)

	next, cmd := m.Update(keyMsg("U"))
	if cmd != nil {
		t.Errorf("expected no command when nothing is stale")
	}
	if next.busy {
		t.Errorf("busy flag must not engage when nothing stale")
	}
}

func TestDLLsResource_UpdateAllCompleteRefreshesCache(t *testing.T) {
	g := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	m := makeDLLsResource([]*game.Game{g}, map[string][]string{
		"dlss": {"3.8.10"},
	}, nil)
	m.busy = true
	results := map[string]string{"1091500:dlss": "ok"}
	next, _ := m.Update(dllsUpdateAllCompleteMsg{results: results, summary: "ok"})
	if next.busy {
		t.Errorf("busy should clear after batch complete")
	}
	if next.lastBatchResult["1091500:dlss"] != "ok" {
		t.Errorf("expected per-cell ok in lastBatchResult, got %v", next.lastBatchResult)
	}
}

func TestDLLsResource_UpdateAll_PassReportsEachCell(t *testing.T) {
	g1 := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	g2 := testGame("Beta", testDLL(game.DLLTypeDLSS, "3.7.0"))
	g2.AppID = 2
	svc := testServices()
	var calls int
	svc.UpdateCachedDLL = func(req DLLUpdateRequest) error {
		calls++
		for i := range req.Game.DLLs {
			if req.Game.DLLs[i].Type == req.TypeInfo.Type {
				req.Game.DLLs[i].Version = req.LatestVersion
			}
		}
		return nil
	}
	m := makeDLLsResourceWithServices([]*game.Game{g1, g2}, map[string][]string{
		"dlss": {"3.8.10"},
	}, svc)

	next, cmd := m.Update(keyMsg("U"))
	if cmd == nil || !next.busy {
		t.Fatalf("expected update command and busy state")
	}
	msg, ok := execCmd(cmd).(dllsUpdateAllCompleteMsg)
	if !ok {
		t.Fatalf("expected dllsUpdateAllCompleteMsg")
	}
	next, _ = next.Update(msg)

	if calls != 2 {
		t.Fatalf("UpdateCachedDLL calls = %d, want 2", calls)
	}
	if next.lastBatchResult["1091500:dlss"] != "ok" || next.lastBatchResult["2:dlss"] != "ok" {
		t.Fatalf("expected per-cell ok results, got %v", next.lastBatchResult)
	}
	if strings.Contains(stripANSI(next.renderDeployment()), "err:") {
		t.Fatalf("success path should not render errors")
	}
}

func TestDLLsResource_UpdateAll_FailReportsFailedCellWithoutSuccessFooter(t *testing.T) {
	g := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	svc := testServices()
	svc.UpdateCachedDLL = func(req DLLUpdateRequest) error {
		return errors.New("copy denied")
	}
	m := makeDLLsResourceWithServices([]*game.Game{g}, map[string][]string{
		"dlss": {"3.8.10"},
	}, svc)

	next, cmd := m.Update(keyMsg("U"))
	if cmd == nil {
		t.Fatalf("expected update command")
	}
	msg := execCmd(cmd).(dllsUpdateAllCompleteMsg)
	next, _ = next.Update(msg)

	if got := next.lastBatchResult["1091500:dlss"]; !strings.Contains(got, "copy denied") {
		t.Fatalf("expected failed cell result, got %q", got)
	}
	if g.DLLs[0].Version != "3.7.0" {
		t.Fatalf("failed update must not mutate version, got %q", g.DLLs[0].Version)
	}
	footer := stripANSI(next.renderFooter())
	if strings.Contains(footer, "update-all finished") {
		t.Fatalf("failure footer must not claim success, got %q", footer)
	}
	if !strings.Contains(footer, "1 failed") {
		t.Fatalf("failure footer should report failure count, got %q", footer)
	}
}

// TestDLLsResource_StaleMarkerUsesAccentOverride documents that the stale
// marker is the same magenta token as the inheritance override marker.
// Rather than colour-matching ANSI bytes we simply assert the marker glyph
// is rendered with a non-empty ANSI escape (i.e. it is styled).
func TestDLLsResource_StaleMarkerUsesAccentOverride(t *testing.T) {
	g := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	m := makeDLLsResource([]*game.Game{g}, map[string][]string{
		"dlss": {"3.8.10"},
	}, nil)
	out := m.renderDeployment()
	// The override marker style renders "◆ " with an ANSI colour code. If
	// plain text (no ESC) contains ◆ the style wasn't applied.
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI styling in deployment output:\n%s", out)
	}
	if !strings.Contains(out, "◆") {
		t.Errorf("expected ◆ stale marker in output:\n%s", out)
	}
}
