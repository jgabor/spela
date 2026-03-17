package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIPCRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-test.dat")

	writer, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = writer.Close() }()

	input := &SharedState{
		Temperature:   72,
		PowerDraw:     285.5,
		PowerLimit:    350,
		Utilization:   95,
		VRAMUsedMB:    18432,
		VRAMTotalMB:   24576,
		GraphicsMHz:   2520,
		MemoryMHz:     10501,
		FanSpeed:      65,
		AlertActive:   true,
		AlertSeverity: AlertWarning,
		Visible:       true,
		Position:      1, // top-right
	}
	writer.WriteState(input)

	reader, err := OpenIPC(path)
	if err != nil {
		t.Fatalf("OpenIPC: %v", err)
	}
	defer func() { _ = reader.Close() }()

	state, ok := reader.ReadState()
	if !ok {
		t.Fatal("ReadState returned inconsistent")
	}

	if state.Temperature != 72 {
		t.Errorf("Temperature: got %d, want 72", state.Temperature)
	}
	if state.PowerDraw != 285.5 {
		t.Errorf("PowerDraw: got %f, want 285.5", state.PowerDraw)
	}
	if state.PowerLimit != 350 {
		t.Errorf("PowerLimit: got %f, want 350", state.PowerLimit)
	}
	if state.Utilization != 95 {
		t.Errorf("Utilization: got %d, want 95", state.Utilization)
	}
	if state.VRAMUsedMB != 18432 {
		t.Errorf("VRAMUsedMB: got %d, want 18432", state.VRAMUsedMB)
	}
	if state.VRAMTotalMB != 24576 {
		t.Errorf("VRAMTotalMB: got %d, want 24576", state.VRAMTotalMB)
	}
	if state.GraphicsMHz != 2520 {
		t.Errorf("GraphicsMHz: got %d, want 2520", state.GraphicsMHz)
	}
	if state.MemoryMHz != 10501 {
		t.Errorf("MemoryMHz: got %d, want 10501", state.MemoryMHz)
	}
	if state.FanSpeed != 65 {
		t.Errorf("FanSpeed: got %d, want 65", state.FanSpeed)
	}
	if !state.AlertActive {
		t.Error("AlertActive: got false, want true")
	}
	if state.AlertSeverity != AlertWarning {
		t.Errorf("AlertSeverity: got %d, want %d", state.AlertSeverity, AlertWarning)
	}
	if !state.Visible {
		t.Error("Visible: got false, want true")
	}
	if state.Position != 1 {
		t.Errorf("Position: got %d, want 1", state.Position)
	}
}

func TestIPCMultipleWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-test.dat")

	ipc, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = ipc.Close() }()

	// Write twice — second write should overwrite first
	ipc.WriteState(&SharedState{Temperature: 60, PowerDraw: 200, PowerLimit: 350})
	ipc.WriteState(&SharedState{Temperature: 85, PowerDraw: 340, PowerLimit: 350, AlertActive: true})

	state, ok := ipc.ReadState()
	if !ok {
		t.Fatal("ReadState returned inconsistent")
	}
	if state.Temperature != 85 {
		t.Errorf("Temperature: got %d, want 85 (from second write)", state.Temperature)
	}
	if !state.AlertActive {
		t.Error("AlertActive should be true from second write")
	}
}

func TestIPCInvalidMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-bad.dat")

	// Create a file with wrong magic
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, totalSize)
	data[0] = 0xFF // wrong magic
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	_, err = OpenIPC(path)
	if err == nil {
		t.Error("OpenIPC should fail with invalid magic")
	}
}

func TestIPCFileTooSmall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-small.dat")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("too small")); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	_, err = OpenIPC(path)
	if err == nil {
		t.Error("OpenIPC should fail with too-small file")
	}
}

func TestIPCSeqlockConsistency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-test.dat")

	ipc, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = ipc.Close() }()

	// After a complete write, seqlock should be even, read should succeed
	ipc.WriteState(&SharedState{Temperature: 70})
	_, ok := ipc.ReadState()
	if !ok {
		t.Error("ReadState should succeed after a complete write")
	}
}

func TestIPCNegativeTemperature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spela-test.dat")

	ipc, err := CreateIPC(path)
	if err != nil {
		t.Fatalf("CreateIPC: %v", err)
	}
	defer func() { _ = ipc.Close() }()

	// Temperature stored as int32, should handle negative
	ipc.WriteState(&SharedState{Temperature: -5})
	state, ok := ipc.ReadState()
	if !ok {
		t.Fatal("ReadState returned inconsistent")
	}
	if state.Temperature != -5 {
		t.Errorf("Temperature: got %d, want -5", state.Temperature)
	}
}
