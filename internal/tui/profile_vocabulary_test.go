package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func gameDetailWithDefaults(t *testing.T) DetailModel {
	t.Helper()
	defaults := &profile.Profile{
		GPU:    profile.GPUSettings{ClockOffset: 150, FanSpeed: 60},
		Proton: profile.ProtonSettings{EnableWayland: true},
	}
	game := &profile.Profile{}
	for field, value := range map[string]any{
		profile.FieldProtonEnableHDR:     true,
		profile.FieldProtonEnableWayland: true,
		profile.FieldGPUClockOffset:      100,
	} {
		if err := game.Set(field, value); err != nil {
			t.Fatal(err)
		}
	}
	return NewDetail(NewStyles(DefaultTheme), game, defaults)
}

// Every row names its source with the shared words, and the game's own values
// show the All games value they replaced.
func TestDetailRowsShowValueAndSource(t *testing.T) {
	detail := gameDetailWithDefaults(t)
	detail.SetSize(120, 80)
	view := stripANSI(detail.View())

	for _, want := range []string{
		"HDR", "On", "this game · All games: Off",
		"Wayland", "this game · same as All games",
		"Clock offset", "100 MHz", "this game · All games: 150 MHz",
		"Fan speed", "60%", "from All games",
		"NGX updater", "system default",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
	for _, retired := range profile.RetiredWords() {
		if strings.Contains(view, retired) {
			t.Errorf("view prints the retired word %q", retired)
		}
	}
}

func TestRootDetailUsesSetHereAndSystemDefault(t *testing.T) {
	root := &profile.Profile{GPU: profile.GPUSettings{ClockOffset: 150}}
	detail := NewRootDetail(NewStyles(DefaultTheme), root)
	detail.SetSize(120, 80)
	view := stripANSI(detail.View())
	for _, want := range []string{"150 MHz", "set here", "system default"} {
		if !strings.Contains(view, want) {
			t.Errorf("root view is missing %q", want)
		}
	}
}

// The description wraps instead of being cut, so the restore sentence it ends
// with survives at 80 columns. No Effect line repeats it.
func TestFocusedNotesWrapWithoutLosingTheRestoreSentence(t *testing.T) {
	detail := gameDetailWithDefaults(t)
	detail.SetSize(48, 80)
	focusField(t, &detail, profile.FieldGPUClockOffset)
	view := stripANSI(detail.View())
	joined := strings.Join(strings.Fields(view), " ")
	if strings.Contains(joined, "Effect:") {
		t.Errorf("the Effect line is retired:\n%s", view)
	}
	for _, want := range []string{
		"Limits the GPU clock to at most 2100 MHz plus this offset",
		"Clocks return to the driver default at exit. 0 leaves clocks unchanged.",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("wrapped notes lost %q:\n%s", want, view)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if width := len([]rune(strings.TrimRight(line, " "))); width > 48 {
			t.Errorf("line is %d columns wide, over 48: %q", width, line)
		}
	}
}

// Typing in a numeric editor replaces the prefilled value; Backspace is shown.
func TestNumberEditorReplacesPrefilledValueAndShowsBackspace(t *testing.T) {
	detail := gameDetailWithDefaults(t)
	detail.SetSize(100, 40)
	focusField(t, &detail, profile.FieldGPUFanSpeed)
	if !detail.BeginEdit() {
		t.Fatal("editor did not open")
	}
	view := stripANSI(detail.View())
	for _, want := range []string{"Whole number in %", "shown range 0 to 100", "Typing replaces the value", "Backspace delete"} {
		if !strings.Contains(view, want) {
			t.Errorf("editor is missing %q:\n%s", want, view)
		}
	}
	for _, key := range []string{"7", "5"} {
		detail.editor.UpdateInput(keyMsg(key))
	}
	if got := detail.editor.Value(); got != "75" {
		t.Fatalf("typed 75 over the prefilled value, got %q", got)
	}
	detail.editor.UpdateInput(keyMsg("backspace"))
	if got := detail.editor.Value(); got != "7" {
		t.Fatalf("backspace should delete one character, got %q", got)
	}
}

func TestNumberEditorKeepsValueWhenCursorMovesFirst(t *testing.T) {
	detail := gameDetailWithDefaults(t)
	focusField(t, &detail, profile.FieldGPUClockOffset)
	detail.BeginEdit()
	detail.editor.UpdateInput(keyMsg("left"))
	detail.editor.UpdateInput(keyMsg("9"))
	if got := detail.editor.Value(); got != "1090" {
		t.Fatalf("moving the cursor first must edit in place (want 1090), got %q", got)
	}
}

func TestPathEditorStillAppends(t *testing.T) {
	var host EditorHost
	host.Begin(EditorSpec{Key: "x", Kind: EditorPath}, "/home/user")
	host.Append("/games")
	if host.Value() != "/home/user/games" {
		t.Fatalf("path editing must append, got %q", host.Value())
	}
}

func TestActionsUseSharedActionWords(t *testing.T) {
	game := BindingContext{GameScope: true}
	root := BindingContext{GameScope: false}
	want := map[string]string{
		"game reset":     profile.ActionFollowAllGames,
		"game reset all": profile.ActionFollowAllGamesForAllSettings,
		"root reset":     profile.ActionUseSystemDefault,
		"root reset all": profile.ActionUseSystemDefaultForAllSettings,
	}
	got := map[string]string{}
	for _, entry := range []struct {
		label   string
		context BindingContext
		action  KeyAction
	}{
		{"game reset", game, ActionDetailReset},
		{"game reset all", game, ActionDetailResetAll},
		{"root reset", root, ActionDetailReset},
		{"root reset all", root, ActionDetailResetAll},
	} {
		for _, binding := range CanonicalKeymap.bindings {
			if binding.Action == entry.action {
				got[entry.label] = describeBinding(binding, entry.context).Description
				break
			}
		}
	}
	for label, expected := range want {
		if got[label] != expected {
			t.Errorf("%s = %q, want %q", label, got[label], expected)
		}
	}
}

func TestResetRowsComeFirstInActions(t *testing.T) {
	var menu []KeyAction
	for _, binding := range CanonicalKeymap.bindings {
		if binding.Menu && binding.Mode == ModeBrowse {
			menu = append(menu, binding.Action)
		}
	}
	if len(menu) < 2 || menu[0] != ActionDetailReset || menu[1] != ActionDetailResetAll {
		t.Fatalf("Actions must start with the reset rows, got %v", menu[:min(len(menu), 4)])
	}
}

func TestAllGamesPointsToTheActionForGamesWithTheirOwnValue(t *testing.T) {
	root := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	detail := NewRootDetail(NewStyles(DefaultTheme), root)
	detail.SetSize(120, 80)
	detail.SetOwnValueGames(map[string][]string{
		profile.FieldProtonVKD3DHeap: {"Cyberpunk 2077", "DrainSim"},
	})
	focusField(t, &detail, profile.FieldProtonVKD3DHeap)
	view := stripANSI(detail.View())
	if !strings.Contains(view, "Actions (0): Apply to games with their own value (2 games)") {
		t.Errorf("All games page does not point to the action:\n%s", view)
	}
	if line := detailLineContaining(view, "VKD3D heap"); strings.Contains(line, "games") {
		t.Errorf("the source column carries the count and gets cut: %q", line)
	}
	for _, retired := range []string{"does not reach", "keep their own value", "differ"} {
		if strings.Contains(view, retired) {
			t.Errorf("All games page still says %q:\n%s", retired, view)
		}
	}
	detail.SetOwnValueGames(map[string][]string{profile.FieldProtonVKD3DHeap: {"DrainSim"}})
	if view := stripANSI(detail.View()); !strings.Contains(view, "Apply to games with their own value (1 game)") {
		t.Errorf("singular wording missing:\n%s", view)
	}
	focusField(t, &detail, profile.FieldProtonEnableHDR)
	if view := stripANSI(detail.View()); strings.Contains(view, "Apply to games") {
		t.Error("a field no game sets points to the action")
	}
	game := gameDetailWithDefaults(t)
	game.SetOwnValueGames(map[string][]string{profile.FieldProtonEnableHDR: {"X"}})
	if view := stripANSI(game.View()); strings.Contains(view, "Apply to games") {
		t.Error("a game page points to the All games action")
	}
}

func TestProfileViewNamesTheGameAtConstrainedSize(t *testing.T) {
	layout := sessionLayout(t)
	sized, _ := layout.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	layout = sized.(LayoutModel)
	sessionSelect(t, &layout, 1091500)
	layout.pane.SetState(layout.pane.State().SelectAspect(nav.AspectProfile))
	view := stripANSI(layout.renderMain())
	if !strings.Contains(view, "Cyberpunk 2077 · App ID") {
		t.Errorf("80x24 Profile view does not name the game:\n%s", view)
	}
}

func TestFindRowIsReachedFromTheFirstItemAndLeavesTheScopeAlone(t *testing.T) {
	layout := sessionLayout(t)
	sidebar := layout.listPane.sidebar
	sidebar.cursor = 0
	sidebar, _ = sidebar.UpdateAction(ActionListPrevious)
	if !sidebar.findFocused || sidebar.cursor != 0 {
		t.Fatalf("Up from the first item must reach the Find row: %+v", sidebar.findFocused)
	}
	rendered := stripANSI(sidebar.View())
	if !strings.Contains(rendered, "> Find games") {
		t.Errorf("Find row is not rendered with the cursor:\n%s", rendered)
	}
	if strings.Contains(rendered, "> All games") {
		t.Errorf("the item cursor must leave the list while the Find row has it:\n%s", rendered)
	}
	sidebar, _ = sidebar.UpdateAction(ActionListNext)
	if sidebar.findFocused || sidebar.cursor != 0 {
		t.Fatal("Down from the Find row must return to the first item")
	}
}

func TestFindRowOpensSearchOnEnterAndOnTyping(t *testing.T) {
	for name, press := range map[string][]string{"enter": {"enter", "c", "y"}, "typing": {"c", "y"}} {
		t.Run(name, func(t *testing.T) {
			layout := sessionLayout(t)
			next := layout.listPane.sidebar.cloneSelection()
			next.cursor = 0
			next.findFocused = true
			layout.listPane.sidebar = next
			layout.focus = FocusList
			for _, key := range press {
				sessionKey(&layout, key)
			}
			if layout.inputMode != ModeSearch {
				t.Fatalf("Search did not open, mode = %v", layout.inputMode)
			}
			if got := layout.listPane.sidebar.search.Value(); got != "cy" {
				t.Fatalf("search text = %q, want cy", got)
			}
			if layout.listPane.sidebar.findFocused {
				t.Error("the match list owns the cursor while searching")
			}
		})
	}
}

func TestSettingsListShowsOnlyGroupsWithOptions(t *testing.T) {
	for _, section := range tuiSettingsSections() {
		if len(section.Options) == 0 {
			t.Errorf("empty group %q listed", section.Title)
		}
	}
}
