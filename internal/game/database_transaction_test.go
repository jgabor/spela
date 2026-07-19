package game

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDatabaseTransactionHelperProcess(t *testing.T) {
	ready := os.Getenv("SPELA_DATABASE_LOCK_READY")
	if ready == "" {
		return
	}
	_, err := Transaction(func(*Database) (bool, error) {
		if err := os.WriteFile(ready, nil, 0o600); err != nil {
			return false, err
		}
		waitForDatabaseTestFile(t, os.Getenv("SPELA_DATABASE_LOCK_RELEASE"))
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseTransactionSerializesAcrossProcesses(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	ready := filepath.Join(root, "ready")
	release := filepath.Join(root, "release")
	command := exec.Command(os.Args[0], "-test.run=^TestDatabaseTransactionHelperProcess$")
	command.Env = append(os.Environ(), "SPELA_DATABASE_LOCK_READY="+ready, "SPELA_DATABASE_LOCK_RELEASE="+release)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o600)
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	waitForDatabaseTestFile(t, ready)

	acquired := make(chan error, 1)
	go func() {
		_, err := Transaction(func(*Database) (bool, error) { return false, nil })
		acquired <- err
	}()
	select {
	case err := <-acquired:
		t.Fatalf("transaction bypassed cross-process lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transaction did not acquire released lock")
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
}

func waitForDatabaseTestFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
