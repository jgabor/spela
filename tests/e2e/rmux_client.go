package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Session represents an active RMUX (or TMUX) automation session.
type Session struct {
	name       string
	binaryPath string
	width      int
	height     int
	tempDir    string
}

// NewSession starts a new detached RMUX or TMUX session running the target command.
func NewSession(sessionName string, width, height int, command string, args []string, environment []string) (*Session, error) {
	multiplexer, err := locateMultiplexer()
	if err != nil {
		return nil, fmt.Errorf("failed to locate terminal multiplexer: %w", err)
	}

	// 1. Kill any existing session with the same name to ensure clean state
	_ = killSession(multiplexer, sessionName)

	// 2. Extract tempDir from the environment variable XDG_CONFIG_HOME
	var tempDir string
	for _, envVar := range environment {
		if strings.HasPrefix(envVar, "XDG_CONFIG_HOME=") {
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) == 2 {
				tempDir = filepath.Dir(parts[1])
			}
			break
		}
	}

	// 3. Build the command line argument list for creating the session.
	// We run the command directly inside the shell or PTY.
	runArgs := []string{
		"new-session",
		"-d",
		"-s", sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	}

	envStr := ""
	if len(environment) > 0 {
		envStr = "env " + strings.Join(environment, " ") + " "
	}

	argsStr := ""
	if len(args) > 0 {
		argsStr = " " + strings.Join(args, " ")
	}

	logRedirect := ""
	if tempDir != "" {
		logRedirect = fmt.Sprintf(" 2>%s/stderr.log", tempDir)
	}

	fullCommand := fmt.Sprintf("%s%s%s%s", envStr, command, argsStr, logRedirect)
	runArgs = append(runArgs, fullCommand)

	cmd := exec.Command(multiplexer, runArgs...)
	cmd.Env = append(os.Environ(), environment...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to start session via %s: %w (stderr: %s)", multiplexer, err, stderr.String())
	}

	// 4. Wait a moment for the session to initialize
	time.Sleep(200 * time.Millisecond)

	return &Session{
		name:       sessionName,
		binaryPath: multiplexer,
		width:      width,
		height:     height,
		tempDir:    tempDir,
	}, nil
}

// SendKeys sends keyboard inputs to the active session.
func (s *Session) SendKeys(keys ...string) error {
	for _, key := range keys {
		args := []string{"send-keys", "-t", s.name, key}
		cmd := exec.Command(s.binaryPath, args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to send key %q to session %s: %w", key, s.name, err)
		}
		// Brief pause between keystrokes to mimic human/system processing
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// Capture reads the current contents of the active terminal pane.
func (s *Session) Capture() (string, error) {
	args := []string{"capture-pane", "-t", s.name, "-p"}
	cmd := exec.Command(s.binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var logInfo string
		if s.tempDir != "" {
			stderrBytes, _ := os.ReadFile(filepath.Join(s.tempDir, "stderr.log"))
			logInfo = fmt.Sprintf("\n--- TARGET STDERR ---\n%s\n", string(stderrBytes))
		}
		return "", fmt.Errorf("failed to capture pane in session %s: %w (stderr: %s)%s", s.name, err, strings.TrimSpace(stderr.String()), logInfo)
	}
	return stdout.String(), nil
}

// WaitForText blocks until the specified text is rendered on the screen or the timeout is reached.
func (s *Session) WaitForText(text string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Capture the final screen output to provide a diagnostic error message.
			currentScreen, _ := s.Capture()
			var logInfo string
			if s.tempDir != "" {
				stderrBytes, _ := os.ReadFile(filepath.Join(s.tempDir, "stderr.log"))
				logInfo = fmt.Sprintf("\n--- TARGET STDERR ---\n%s\n", string(stderrBytes))
			}
			return fmt.Errorf("timed out waiting for text %q in session %s.\nLast screen state:\n%s%s", text, s.name, currentScreen, logInfo)
		case <-ticker.C:
			screen, err := s.Capture()
			if err != nil {
				return err
			}
			if strings.Contains(screen, text) {
				return nil
			}
		}
	}
}

// Close terminates the session and frees resources.
func (s *Session) Close() error {
	return killSession(s.binaryPath, s.name)
}

// locateMultiplexer looks for 'rmux' first, and falls back to 'tmux' if not found.
func locateMultiplexer() (string, error) {
	if path, err := exec.LookPath("rmux"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("tmux"); err == nil {
		fmt.Fprintln(os.Stderr, "DIAGNOSTIC: rmux not found in PATH; falling back to tmux")
		return path, nil
	}
	return "", fmt.Errorf("neither rmux nor tmux was found in PATH")
}

func killSession(multiplexer, name string) error {
	cmd := exec.Command(multiplexer, "kill-session", "-t", name)
	return cmd.Run()
}
