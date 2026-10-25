package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func captureDefault(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	mu.Lock()
	previous := logger
	logger = slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: &level}))
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		logger = previous
		mu.Unlock()
		_ = SetLevel("info")
	})
	return &output
}

func TestSetLevelFiltersTheDefaultHandler(t *testing.T) {
	output := captureDefault(t)

	if err := SetLevel("warn"); err != nil {
		t.Fatal(err)
	}
	Info("hidden info")
	Warn("shown warning")
	if got := output.String(); strings.Contains(got, "hidden info") || !strings.Contains(got, "shown warning") {
		t.Fatalf("warn level output = %q", got)
	}

	if err := SetLevel("debug"); err != nil {
		t.Fatal(err)
	}
	Debug("shown debug")
	if !strings.Contains(output.String(), "shown debug") {
		t.Fatalf("debug level output = %q", output.String())
	}
}

func TestSetLevelRejectsUnknownNameAndKeepsLevel(t *testing.T) {
	output := captureDefault(t)
	if err := SetLevel("error"); err != nil {
		t.Fatal(err)
	}

	if err := SetLevel("verbose"); err == nil {
		t.Fatal("SetLevel(verbose) error = nil, want an error")
	}
	Warn("hidden warning")
	if strings.Contains(output.String(), "hidden warning") {
		t.Fatalf("level changed after a rejected name: %q", output.String())
	}
}
