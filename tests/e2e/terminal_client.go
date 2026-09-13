//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var sessionSequence atomic.Uint64

// Session represents an active Terminal Control session.
type Session struct {
	name             string
	path             string
	runtimeDirectory string
}

// NewSession starts a uniquely named Terminal Control session.
func NewSession(baseName string, width, height int, command string, args []string, environment []string, runtimeDirectory string) (*Session, error) {
	path, err := exec.LookPath("termctrl")
	if err != nil {
		return nil, fmt.Errorf("termctrl was not found in PATH: %w", err)
	}

	name := fmt.Sprintf("%s-%d-%d", baseName, time.Now().UnixNano(), sessionSequence.Add(1))
	commandArgs := []string{"start", name, "--cols", strconv.Itoa(width), "--rows", strconv.Itoa(height), "--", "env"}
	commandArgs = append(commandArgs, environment...)
	commandArgs = append(commandArgs, command)
	commandArgs = append(commandArgs, args...)
	session := &Session{name: name, path: path, runtimeDirectory: runtimeDirectory}
	if output, err := session.command(commandArgs...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("start Terminal Control session %s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	if _, err := os.Stat(filepath.Join(runtimeDirectory, name+".lock")); err != nil {
		_, _ = session.command("stop", name).CombinedOutput()
		return nil, fmt.Errorf("verify isolated Terminal Control lock for %s: %w", name, err)
	}

	return session, nil
}

func (s *Session) command(arguments ...string) *exec.Cmd {
	command := exec.Command(s.path, arguments...)
	command.Env = append(os.Environ(), "TERMCTRL_RUNTIME_DIR="+s.runtimeDirectory)
	return command
}

// SendKeys sends keyboard inputs to the active session.
func (s *Session) SendKeys(keys ...string) error {
	args := []string{"send", s.name}
	namedKeys := map[string]bool{
		"backspace": true, "delete": true, "down": true, "end": true, "enter": true,
		"escape": true, "home": true, "left": true, "page-down": true, "page-up": true,
		"right": true, "shift-tab": true, "tab": true, "up": true,
	}
	for _, key := range keys {
		if !namedKeys[key] && !strings.HasPrefix(key, "ctrl-") {
			key = "text:" + key
		}
		args = append(args, key)
	}
	if output, err := s.command(args...).CombinedOutput(); err != nil {
		return fmt.Errorf("send input to %s: %w: %s", s.name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// SendText always sends literal text, including words such as "enter" that
// SendKeys treats as named keys.
func (s *Session) SendText(text string) error {
	if output, err := s.command("send", s.name, "text:"+text).CombinedOutput(); err != nil {
		return fmt.Errorf("send text to %s: %w: %s", s.name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Capture reads the rendered visible screen.
func (s *Session) Capture() (string, error) {
	var stdout, stderr bytes.Buffer
	command := s.command("show", s.name)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("show session %s: %w: %s", s.name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// WaitForText waits until text is visible on the rendered screen.
func (s *Session) WaitForText(text string, timeout time.Duration) error {
	output, err := s.command("wait", s.name, text, "--timeout", strconv.FormatInt(timeout.Milliseconds(), 10)).CombinedOutput()
	if err != nil {
		screen, _ := s.Capture()
		return fmt.Errorf("wait for %q in %s: %w: %s\nLast screen:\n%s", text, s.name, err, strings.TrimSpace(string(output)), screen)
	}
	return nil
}

// Resize changes the running PTY and terminal emulator dimensions together.
// Callers must wait for a visible frame before inspecting the new layout.
func (s *Session) Resize(width, height int) error {
	output, err := s.command("resize", s.name, "--cols", strconv.Itoa(width), "--rows", strconv.Itoa(height)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("resize session %s to %dx%d: %w: %s", s.name, width, height, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Close stops the session and removes it from Terminal Control.
func (s *Session) Close() error {
	output, err := s.command("stop", s.name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop session %s: %w: %s", s.name, err, strings.TrimSpace(string(output)))
	}
	return nil
}
