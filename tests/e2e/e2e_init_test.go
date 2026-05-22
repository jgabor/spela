package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var testBinaryPath string

func TestMain(m *testing.M) {
	// Build the spela test binary once before running any E2E tests
	cmd := exec.Command("go", "build", "-o", "./spela-test-binary", "../../cmd/spela")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("ERROR: failed to build spela test binary: %v\n", err)
		os.Exit(1)
	}

	absPath, err := filepath.Abs("./spela-test-binary")
	if err != nil {
		fmt.Printf("ERROR: failed to determine absolute path of test binary: %v\n", err)
		_ = os.Remove("./spela-test-binary")
		os.Exit(1)
	}
	testBinaryPath = absPath

	// Run tests
	code := m.Run()

	// Clean up test binary
	_ = os.Remove(testBinaryPath)
	os.Exit(code)
}
