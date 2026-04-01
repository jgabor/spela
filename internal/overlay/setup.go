package overlay

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Setup creates an IPC file for the overlay and starts a collector that
// periodically writes shared state. Returns the IPC path and a cleanup
// function that stops the collector and removes the IPC file.
func Setup(appID uint64, runtimeDir string, interval time.Duration, collect CollectFunc) (string, func(), error) {
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return "", nil, fmt.Errorf("create runtime dir: %w", err)
	}

	ipcPath := filepath.Join(runtimeDir, fmt.Sprintf("overlay-%d.dat", appID))
	ipc, err := CreateIPC(ipcPath)
	if err != nil {
		return "", nil, fmt.Errorf("create ipc: %w", err)
	}

	c := NewCollector(ipc, collect, interval)
	c.Start()

	cleanup := func() {
		c.Stop()
		_ = ipc.Close()
		_ = os.Remove(ipcPath)
	}

	return ipcPath, cleanup, nil
}
