package privilege

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

func IsPolkitAvailable() bool {
	_, err := exec.LookPath("pkexec")
	return err == nil
}

var ErrPolkitNotAvailable = errors.New("pkexec not available: install polkit for privileged operations")

func IsRoot() bool {
	return os.Geteuid() == 0
}

func Exec(cmd string, args ...string) (*ExecResult, error) {
	if IsRoot() {
		return run(cmd, args...), nil
	}

	if !IsPolkitAvailable() {
		return nil, ErrPolkitNotAvailable
	}

	result := run("pkexec", append([]string{cmd}, args...)...)
	if result.ExitCode != 0 {
		if isAuthDismissed(result) {
			return nil, &AuthError{Message: "authentication dismissed by user"}
		}
		return nil, errors.New(strings.TrimSpace(result.Stderr))
	}
	return result, nil
}

func ExecWithInput(input string, cmd string, args ...string) (*ExecResult, error) {
	if IsRoot() {
		return runWithInput(input, cmd, args...), nil
	}

	if !IsPolkitAvailable() {
		return nil, ErrPolkitNotAvailable
	}

	result := runWithInput(input, "pkexec", append([]string{cmd}, args...)...)
	if result.ExitCode != 0 {
		if isAuthDismissed(result) {
			return nil, &AuthError{Message: "authentication dismissed by user"}
		}
		return nil, errors.New(strings.TrimSpace(result.Stderr))
	}
	return result, nil
}

func run(cmd string, args ...string) *ExecResult {
	return execCommand(nil, cmd, args...)
}

func runWithInput(input string, cmd string, args ...string) *ExecResult {
	return execCommand(strings.NewReader(input), cmd, args...)
}

func execCommand(stdin io.Reader, cmd string, args ...string) *ExecResult {
	command := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if stdin != nil {
		command.Stdin = stdin
	}

	err := command.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// ExecSelf re-executes the current binary with the given arguments under
// privilege escalation. This enables batching multiple privileged operations
// into a single pkexec round-trip.
func ExecSelf(args ...string) (*ExecResult, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to determine executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve executable path: %w", err)
	}
	return Exec(exe, args...)
}

func isAuthDismissed(result *ExecResult) bool {
	combined := strings.ToLower(result.Stdout + result.Stderr)
	return strings.Contains(combined, "authentication cancelled") ||
		strings.Contains(combined, "canceled") ||
		strings.Contains(combined, "dismissed") ||
		result.ExitCode == 126
}
