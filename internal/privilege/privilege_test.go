package privilege

import (
	"os"
	"testing"
)

func TestIsPolkitAvailable(t *testing.T) {
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	_ = os.Setenv("PATH", "/nonexistent")
	if IsPolkitAvailable() {
		t.Error("Expected IsPolkitAvailable to return false when pkexec not in PATH")
	}
}

func TestIsRoot(t *testing.T) {
	result := IsRoot()
	if os.Geteuid() == 0 && !result {
		t.Error("Expected IsRoot to return true when running as root")
	}
	if os.Geteuid() != 0 && result {
		t.Error("Expected IsRoot to return false when not running as root")
	}
}

func TestIsAuthDismissed(t *testing.T) {
	tests := []struct {
		name     string
		result   *ExecResult
		expected bool
	}{
		{
			name: "authentication cancelled",
			result: &ExecResult{
				Stderr:   "authentication cancelled",
				ExitCode: 1,
			},
			expected: true,
		},
		{
			name: "exit code 126",
			result: &ExecResult{
				Stderr:   "",
				ExitCode: 126,
			},
			expected: true,
		},
		{
			name: "dismissed message",
			result: &ExecResult{
				Stderr:   "Request dismissed",
				ExitCode: 1,
			},
			expected: true,
		},
		{
			name: "successful execution",
			result: &ExecResult{
				Stderr:   "",
				ExitCode: 0,
			},
			expected: false,
		},
		{
			name: "generic error",
			result: &ExecResult{
				Stderr:   "some other error",
				ExitCode: 1,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAuthDismissed(tt.result); got != tt.expected {
				t.Errorf("isAuthDismissed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthError(t *testing.T) {
	err := &AuthError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("AuthError.Error() = %v, want %v", err.Error(), "test error")
	}

	if !IsAuthError(err) {
		t.Error("IsAuthError should return true for AuthError")
	}

	if IsAuthError(os.ErrNotExist) {
		t.Error("IsAuthError should return false for non-AuthError")
	}
}

func TestRun(t *testing.T) {
	result := run("echo", "hello")
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("Expected stdout 'hello\\n', got %q", result.Stdout)
	}
}

func TestRunWithInput(t *testing.T) {
	result := runWithInput("test input", "cat")
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "test input" {
		t.Errorf("Expected stdout 'test input', got %q", result.Stdout)
	}
}

func TestRunNonexistentCommand(t *testing.T) {
	result := run("nonexistent_command_12345")
	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for nonexistent command")
	}
}

func TestExecAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges")
	}

	result, err := Exec("echo", "test")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Stdout != "test\n" {
		t.Errorf("Expected stdout 'test\\n', got %q", result.Stdout)
	}
}

func TestExecPolkitUnavailable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Test requires non-root privileges")
	}

	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	_ = os.Setenv("PATH", "/nonexistent")
	_, err := Exec("echo", "test")
	if err == nil {
		t.Error("Expected error when pkexec not available")
	}
}
