package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireReleaseAndAlreadyRunningContract(t *testing.T) {
	runtimeDirectory := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDirectory)
	if err := Acquire(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(lockPath())
	if err != nil || string(data) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("lock PID = %q, %v", data, err)
	}
	err = Acquire()
	var running *AlreadyRunningError
	if !errors.Is(err, ErrAlreadyRunning) || !errors.As(err, &running) || running.PID != os.Getpid() {
		t.Fatalf("second Acquire error = %v", err)
	}
	if err := Release(); err != nil {
		t.Fatal(err)
	}
	if err := Release(); !os.IsNotExist(err) {
		t.Fatalf("second Release error = %v", err)
	}
}

func TestAcquireRecoversInvalidAndStaleLocks(t *testing.T) {
	for _, contents := range []string{"not-a-pid", "999999999"} {
		t.Run(contents, func(t *testing.T) {
			runtimeDirectory := t.TempDir()
			t.Setenv("XDG_RUNTIME_DIR", runtimeDirectory)
			path := lockPath()
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Acquire(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != strconv.Itoa(os.Getpid()) {
				t.Fatalf("recovered lock PID = %q, %v", data, err)
			}
			if err := Release(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
