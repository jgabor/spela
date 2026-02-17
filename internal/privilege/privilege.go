package privilege

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func IsRoot() bool {
	return os.Geteuid() == 0
}

func Exec(cmd string, args ...string) (*ExecResult, error) {
	if IsRoot() {
		return run(cmd, args...), nil
	}

	if !IsPolkitAvailable() {
		return nil, fmt.Errorf("pkexec not available: install polkit for privileged operations")
	}

	result := run("pkexec", append([]string{cmd}, args...)...)
	if result.ExitCode != 0 {
		if isAuthDismissed(result) {
			return nil, &AuthError{Message: "authentication dismissed by user"}
		}
		return nil, fmt.Errorf("%s", strings.TrimSpace(result.Stderr))
	}
	return result, nil
}

func ExecWithInput(input string, cmd string, args ...string) (*ExecResult, error) {
	if IsRoot() {
		return runWithInput(input, cmd, args...), nil
	}

	if !IsPolkitAvailable() {
		return nil, fmt.Errorf("pkexec not available: install polkit for privileged operations")
	}

	result := runWithInput(input, "pkexec", append([]string{cmd}, args...)...)
	if result.ExitCode != 0 {
		if isAuthDismissed(result) {
			return nil, &AuthError{Message: "authentication dismissed by user"}
		}
		return nil, fmt.Errorf("%s", strings.TrimSpace(result.Stderr))
	}
	return result, nil
}

func run(cmd string, args ...string) *ExecResult {
	command := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
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

func runWithInput(input string, cmd string, args ...string) *ExecResult {
	command := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.Stdin = strings.NewReader(input)

	err := command.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
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

func isAuthDismissed(result *ExecResult) bool {
	combined := strings.ToLower(result.Stdout + result.Stderr)
	return strings.Contains(combined, "authentication cancelled") ||
		strings.Contains(combined, "canceled") ||
		strings.Contains(combined, "dismissed") ||
		result.ExitCode == 126
}
