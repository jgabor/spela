package overlay

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectorWritesState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-collector.dat")

	ipc, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = ipc.Close() }()

	var callCount atomic.Int32
	collect := func() SharedState {
		callCount.Add(1)
		return SharedState{
			Temperature: 72,
			PowerDraw:   285,
			PowerLimit:  350,
			Utilization: 95,
			Visible:     true,
		}
	}

	c := NewCollector(ipc, collect, 50*time.Millisecond)
	c.Start()

	// Wait for at least one tick beyond the initial write
	time.Sleep(120 * time.Millisecond)
	c.Stop()

	if count := callCount.Load(); count < 2 {
		t.Errorf("expected at least 2 calls (initial + tick), got %d", count)
	}

	// Verify state was written to IPC
	state, ok := ipc.ReadState()
	if !ok {
		t.Fatal("ReadState returned inconsistent after collector stopped")
	}
	if state.Temperature != 72 {
		t.Errorf("Temperature: got %d, want 72", state.Temperature)
	}
	if state.Utilization != 95 {
		t.Errorf("Utilization: got %d, want 95", state.Utilization)
	}
	if !state.Visible {
		t.Error("Visible: got false, want true")
	}
}

func TestCollectorImmediateWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-collector.dat")

	ipc, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = ipc.Close() }()

	collect := func() SharedState {
		return SharedState{Temperature: 65, FanSpeed: 40}
	}

	c := NewCollector(ipc, collect, time.Hour) // long interval — won't tick
	c.Start()

	// Give the goroutine a moment to run the initial write
	time.Sleep(10 * time.Millisecond)
	c.Stop()

	// The collector writes immediately on start, before the first tick
	state, ok := ipc.ReadState()
	if !ok {
		t.Fatal("ReadState returned inconsistent")
	}
	if state.Temperature != 65 {
		t.Errorf("Temperature: got %d, want 65", state.Temperature)
	}
	if state.FanSpeed != 40 {
		t.Errorf("FanSpeed: got %d, want 40", state.FanSpeed)
	}
}

func TestCollectorStopIsClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-collector.dat")

	ipc, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = ipc.Close() }()

	collect := func() SharedState {
		return SharedState{Temperature: 50}
	}

	c := NewCollector(ipc, collect, 10*time.Millisecond)
	c.Start()
	time.Sleep(50 * time.Millisecond)

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("Stop() did not return within 1 second")
	}
}

func TestCollectorUpdatesState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-collector.dat")

	ipc, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = ipc.Close() }()

	var temp atomic.Int32
	temp.Store(60)

	collect := func() SharedState {
		return SharedState{Temperature: int(temp.Load())}
	}

	c := NewCollector(ipc, collect, 30*time.Millisecond)
	c.Start()

	time.Sleep(10 * time.Millisecond)

	// Change the temperature — next tick should pick it up
	temp.Store(85)
	time.Sleep(50 * time.Millisecond)
	c.Stop()

	state, ok := ipc.ReadState()
	if !ok {
		t.Fatal("ReadState returned inconsistent")
	}
	if state.Temperature != 85 {
		t.Errorf("Temperature: got %d, want 85 (updated value)", state.Temperature)
	}
}
