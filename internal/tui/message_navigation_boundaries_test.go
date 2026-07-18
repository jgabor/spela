package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
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

func TestDLSSPresetModalSupportedKeyAliasesAndBoundaryViews(t *testing.T) {
	modal := NewDLSSPresetModal(NewStyles(DefaultTheme, true))
	if next, command := modal.Update(keyMsg("enter")); next.Visible() || command != nil || next.View() != "" {
		t.Fatal("hidden preset modal handled input")
	}
	modal.SetSize(20, 10)
	modal.Open(profile.DLSSPresetAuto)
	if !strings.Contains(stripANSI(modal.View()), "auto") {
		t.Fatal("auto preset view missing selection")
	}
	for _, key := range []string{"up", "k", "down", "j"} {
		modal, _ = modal.Update(keyMsg(key))
	}
	modal.cursor = 0
	modal, _ = modal.Update(keyMsg("up"))
	modal.cursor = len(dlssPresetOrder) - 1
	modal, _ = modal.Update(keyMsg("down"))
	modal, command := modal.Update(keyMsg("enter"))
	if modal.Visible() || command == nil {
		t.Fatal("preset selection did not close modal")
	}
	if _, ok := command().(dlssPresetSelectedMsg); !ok {
		t.Fatalf("preset selection = %#v", command())
	}
	modal.Open(profile.DLSSPresetDefault)
	modal, command = modal.Update(keyMsg("q"))
	if modal.Visible() || command == nil {
		t.Fatal("preset q did not cancel")
	}
	if _, ok := command().(dlssPresetCancelledMsg); !ok {
		t.Fatalf("preset cancellation = %#v", command())
	}
}

func TestContextNavigationUnsupportedAndGlobalBoundaryContracts(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	context := NewContextNav(styles, testSidebar(), nil)
	context.SetSize(30, 10)
	if _, _, handled := context.Update(keyMsg("1")); handled {
		t.Fatal("nil-state context handled library input")
	}
	context.applySectionCursor()
	context.syncCursorFromState()
	if context.cursor != 0 {
		t.Fatalf("nil-state cursor = %d", context.cursor)
	}
	state := nav.DefaultState()
	context.navState = &state
	for _, key := range []string{"1", "3", "unknown"} {
		if _, _, handled := context.Update(keyMsg(key)); handled {
			t.Errorf("global key %q was unexpectedly handled", key)
		}
	}
	state.Destination = nav.Destination(99)
	if _, _, handled := context.Update(keyMsg("down")); handled {
		t.Fatal("unknown destination handled input")
	}
}
