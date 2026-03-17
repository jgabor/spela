package overlay

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Protocol constants for the overlay shared memory file.
const (
	ipcMagic   uint32 = 0x5350454C // "SPEL"
	ipcVersion uint16 = 1

	headerSize = 16
	stateSize  = 64
	totalSize  = headerSize + stateSize
)

// Header layout (16 bytes):
//
//	[0:4]   magic        uint32  = 0x5350454C
//	[4:6]   version      uint16  = 1
//	[6:8]   reserved     uint16
//	[8:12]  stateOffset  uint32  = 16
//	[12:16] stateSize    uint32  = 64
//
// State layout (64 bytes):
//
//	[0:4]   seqlock       uint32  (even=stable, odd=writing)
//	[4:8]   temperature   int32   (°C)
//	[8:12]  powerDraw     uint32  (milliwatts)
//	[12:16] powerLimit    uint32  (milliwatts)
//	[16:20] utilization   uint32  (percentage 0-100)
//	[20:24] vramUsedMB    uint32
//	[24:28] vramTotalMB   uint32
//	[28:32] graphicsMHz   uint32
//	[32:36] memoryMHz     uint32
//	[36:40] fanSpeed      uint32  (percentage 0-100)
//	[40:41] alertActive   uint8   (0 or 1)
//	[41:42] alertSeverity uint8   (0=info, 1=warning, 2=critical)
//	[42:43] visible       uint8   (0 or 1)
//	[43:44] position      uint8   (0=TL, 1=TR, 2=BL, 3=BR)
//	[44:64] reserved      [20]byte

// SharedState is the data written by Go and read by the overlay layer.
type SharedState struct {
	Temperature   int
	PowerDraw     float64 // watts (converted to milliwatts on wire)
	PowerLimit    float64 // watts (converted to milliwatts on wire)
	Utilization   int
	VRAMUsedMB    int
	VRAMTotalMB   int
	GraphicsMHz   int
	MemoryMHz     int
	FanSpeed      int
	AlertActive   bool
	AlertSeverity AlertSeverity
	Visible       bool
	Position      uint8
}

// IPCFile wraps a memory-mapped shared file for overlay communication.
type IPCFile struct {
	file *os.File
	data []byte
	seq  uint32 // writer-side sequence counter
}

// CreateIPC creates a new overlay shared memory file at the given path.
func CreateIPC(path string) (*IPCFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create ipc file: %w", err)
	}

	if err := f.Truncate(totalSize); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("truncate ipc file: %w", err)
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, totalSize,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("mmap ipc file: %w", err)
	}

	ipc := &IPCFile{file: f, data: data}
	ipc.writeHeader()
	return ipc, nil
}

// OpenIPC opens an existing overlay shared memory file and validates the header.
func OpenIPC(path string) (*IPCFile, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open ipc file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat ipc file: %w", err)
	}
	if info.Size() < totalSize {
		_ = f.Close()
		return nil, fmt.Errorf("ipc file too small: %d < %d", info.Size(), totalSize)
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, totalSize,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("mmap ipc file: %w", err)
	}

	ipc := &IPCFile{file: f, data: data}
	if err := ipc.validateHeader(); err != nil {
		_ = ipc.Close()
		return nil, err
	}
	return ipc, nil
}

// WriteState writes the shared state using seqlock synchronization.
// The seqlock is incremented to odd before writing (indicates write in progress),
// then incremented to even after writing (indicates stable data).
func (f *IPCFile) WriteState(state *SharedState) {
	seqPtr := (*uint32)(unsafe.Pointer(&f.data[headerSize]))

	// Increment to odd = write in progress
	f.seq++
	atomic.StoreUint32(seqPtr, f.seq)

	// Write state fields into the mmap region
	s := f.data[headerSize+4 : headerSize+stateSize] // skip seqlock
	binary.LittleEndian.PutUint32(s[0:4], uint32(int32(state.Temperature)))
	binary.LittleEndian.PutUint32(s[4:8], uint32(state.PowerDraw*1000))
	binary.LittleEndian.PutUint32(s[8:12], uint32(state.PowerLimit*1000))
	binary.LittleEndian.PutUint32(s[12:16], uint32(state.Utilization))
	binary.LittleEndian.PutUint32(s[16:20], uint32(state.VRAMUsedMB))
	binary.LittleEndian.PutUint32(s[20:24], uint32(state.VRAMTotalMB))
	binary.LittleEndian.PutUint32(s[24:28], uint32(state.GraphicsMHz))
	binary.LittleEndian.PutUint32(s[28:32], uint32(state.MemoryMHz))
	binary.LittleEndian.PutUint32(s[32:36], uint32(state.FanSpeed))

	if state.AlertActive {
		s[36] = 1
	} else {
		s[36] = 0
	}
	s[37] = uint8(state.AlertSeverity)

	if state.Visible {
		s[38] = 1
	} else {
		s[38] = 0
	}
	s[39] = state.Position

	// Increment to even = write complete, data stable
	f.seq++
	atomic.StoreUint32(seqPtr, f.seq)
}

// ReadState reads the shared state with seqlock verification.
// Returns the state and true if the read was consistent (seqlock was even
// and unchanged during the read). Returns false if the writer was mid-update.
func (f *IPCFile) ReadState() (*SharedState, bool) {
	seqPtr := (*uint32)(unsafe.Pointer(&f.data[headerSize]))

	// Read seqlock before
	seq1 := atomic.LoadUint32(seqPtr)
	if seq1%2 != 0 {
		return nil, false // writer is mid-update
	}

	// Read state
	s := f.data[headerSize+4 : headerSize+stateSize]
	state := &SharedState{
		Temperature:   int(int32(binary.LittleEndian.Uint32(s[0:4]))),
		PowerDraw:     float64(binary.LittleEndian.Uint32(s[4:8])) / 1000,
		PowerLimit:    float64(binary.LittleEndian.Uint32(s[8:12])) / 1000,
		Utilization:   int(binary.LittleEndian.Uint32(s[12:16])),
		VRAMUsedMB:    int(binary.LittleEndian.Uint32(s[16:20])),
		VRAMTotalMB:   int(binary.LittleEndian.Uint32(s[20:24])),
		GraphicsMHz:   int(binary.LittleEndian.Uint32(s[24:28])),
		MemoryMHz:     int(binary.LittleEndian.Uint32(s[28:32])),
		FanSpeed:      int(binary.LittleEndian.Uint32(s[32:36])),
		AlertActive:   s[36] != 0,
		AlertSeverity: AlertSeverity(s[37]),
		Visible:       s[38] != 0,
		Position:      s[39],
	}

	// Read seqlock after — must match
	seq2 := atomic.LoadUint32(seqPtr)
	if seq1 != seq2 {
		return nil, false // writer updated during our read
	}

	return state, true
}

// Close unmaps and closes the shared memory file.
func (f *IPCFile) Close() error {
	if f.data != nil {
		if err := syscall.Munmap(f.data); err != nil {
			return err
		}
		f.data = nil
	}
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

// Path returns the file path of the IPC file.
func (f *IPCFile) Path() string {
	return f.file.Name()
}

func (f *IPCFile) writeHeader() {
	binary.LittleEndian.PutUint32(f.data[0:4], ipcMagic)
	binary.LittleEndian.PutUint16(f.data[4:6], ipcVersion)
	binary.LittleEndian.PutUint16(f.data[6:8], 0) // reserved
	binary.LittleEndian.PutUint32(f.data[8:12], headerSize)
	binary.LittleEndian.PutUint32(f.data[12:16], stateSize)
}

func (f *IPCFile) validateHeader() error {
	magic := binary.LittleEndian.Uint32(f.data[0:4])
	if magic != ipcMagic {
		return fmt.Errorf("invalid ipc magic: 0x%08X (expected 0x%08X)", magic, ipcMagic)
	}
	version := binary.LittleEndian.Uint16(f.data[4:6])
	if version != ipcVersion {
		return fmt.Errorf("unsupported ipc version: %d (expected %d)", version, ipcVersion)
	}
	return nil
}
