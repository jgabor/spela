//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var testBinaryPath string
var testBinaryDirectory string

func TestMain(m *testing.M) {
	var err error
	testBinaryDirectory, err = os.MkdirTemp("", "spela-tui-e2e-binary-*")
	if err != nil {
		fmt.Printf("ERROR: failed to create binary directory: %v\n", err)
		os.Exit(1)
	}
	testBinaryPath = filepath.Join(testBinaryDirectory, "spela")

	cmd := exec.Command("go", "build", "-tags", "e2e", "-o", testBinaryPath, "../../cmd/spela")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("ERROR: failed to build spela test binary: %v\n", err)
		_ = os.RemoveAll(testBinaryDirectory)
		os.Exit(1)
	}

	code := m.Run()
	if err := os.RemoveAll(testBinaryDirectory); err != nil {
		fmt.Printf("ERROR: failed to remove binary directory: %v\n", err)
		code = 1
	} else if _, err := os.Stat(testBinaryDirectory); err == nil {
		fmt.Printf("ERROR: binary directory remains after cleanup: %s\n", testBinaryDirectory)
		code = 1
	} else if !os.IsNotExist(err) {
		fmt.Printf("ERROR: failed to verify binary directory cleanup: %v\n", err)
		code = 1
	}
	os.Exit(code)
}
