package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/nav"
)

func TestMessageBarSupportedTypesFlashAndStaleTimerContracts(t *testing.T) {
	bar := NewMessageBar(NewStyles(DefaultTheme, true))
	bar.SetWidth(50)
	for _, messageType := range []MessageType{MessageInfo, MessageSuccess, MessageError} {
		command := bar.SetMessage("Saved", messageType)
		if command == nil || !bar.HasMessage() || !strings.Contains(stripANSI(bar.View()), "Saved") {
			t.Fatalf("message type %d did not render", messageType)
		}
		stale := bar.timestamp.Add(-time.Second)
		next, _ := bar.Update(messageClearMsg{timestamp: stale})
		if !next.HasMessage() {
			t.Fatalf("stale timer cleared message type %d", messageType)
		}
		next, command = bar.Update(flashTickMsg{timestamp: bar.timestamp})
		if command == nil || next.flashPhase != 2 || next.View() == "" {
			t.Fatalf("message flash phase = %d, command %v", next.flashPhase, command)
		}
		next, _ = next.Update(flashTickMsg{timestamp: next.timestamp})
		if next.flashPhase != 3 {
			t.Fatalf("final flash phase = %d", next.flashPhase)
		}
		bar, _ = next.Update(messageClearMsg{timestamp: next.timestamp})
		if bar.HasMessage() || bar.View() != "" {
			t.Fatalf("message type %d did not clear", messageType)
		}
	}
	bar.SetMessage("manual", MessageInfo)
	bar.Clear()
	if bar.HasMessage() {
		t.Fatal("manual clear retained message")
	}
}

func TestListPaneUnsupportedAndGlobalBoundaryContracts(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	list := NewListPane(styles, testSidebar(), nil)
	list.SetSize(30, 10)
	if _, _, handled := list.Update(keyMsg("1")); handled {
		t.Fatal("nil-state List handled unsupported input")
	}
	list.applySectionCursor()
	list.syncCursorFromState()
	if list.cursor != 0 {
		t.Fatalf("nil-state cursor = %d", list.cursor)
	}
	state := nav.DefaultState()
	list.navState = &state
	for _, key := range []string{"1", "3", "unknown"} {
		if _, _, handled := list.Update(keyMsg(key)); handled {
			t.Errorf("global key %q was unexpectedly handled", key)
		}
	}
	state.Destination = nav.Destination(99)
	if _, _, handled := list.Update(keyMsg("down")); handled {
		t.Fatal("unknown destination handled input")
	}
}
