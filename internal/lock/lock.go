package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/jgabor/spela/internal/xdg"
)

const lockFileName = "spela.pid"

var ErrAlreadyRunning = errors.New("spela is already running")

type AlreadyRunningError struct {
	PID int
}

func (e *AlreadyRunningError) Error() string {
	return fmt.Sprintf("spela is already running (PID: %d)", e.PID)
}

func (e *AlreadyRunningError) Is(target error) bool {
	return target == ErrAlreadyRunning
}

func lockPath() string {
	return filepath.Join(xdg.RuntimeDir(), lockFileName)
}

func Acquire() error {
	path := lockPath()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		defer func() { _ = file.Close() }()
		_, err = file.WriteString(strconv.Itoa(os.Getpid()))
		return err
	}

	if !os.IsExist(err) {
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	existingPID, readErr := readPID(path)
	if readErr != nil {
		_ = os.Remove(path)
		return Acquire()
	}

	if processExists(existingPID) {
		return &AlreadyRunningError{PID: existingPID}
	}

	_ = os.Remove(path)
	return Acquire()
}

func Release() error {
	return os.Remove(lockPath())
}

func IsHeld() (bool, int) {
	pid, err := readPID(lockPath())
	if err != nil {
		return false, 0
	}

	if !processExists(pid) {
		return false, 0
	}

	return true, pid
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}
