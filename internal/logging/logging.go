// Package logging is the centralized slog wrapper for spela. It exposes
// level-specific helpers (Warn, Info) and a SetHandler hook so tests can
// redirect output to a buffer without touching slog.SetDefault directly.
package logging

import (
	"log/slog"
	"sync"
)

var (
	mu     sync.RWMutex
	logger = slog.Default()
)

// Warn emits a warning-level record using the current logger.
func Warn(msg string, args ...any) {
	mu.RLock()
	defer mu.RUnlock()
	logger.Warn(msg, args...)
}

// Info emits an info-level record using the current logger. Used for
// non-actionable context (e.g. a preflight probe couldn't run and we're
// continuing anyway).
func Info(msg string, args ...any) {
	mu.RLock()
	defer mu.RUnlock()
	logger.Info(msg, args...)
}

// SetHandler replaces the active slog handler for all logging calls
// made through this package. Returns a restore function suitable for
// t.Cleanup so tests can capture log output without leaking global state.
func SetHandler(h slog.Handler) (restore func()) {
	mu.Lock()
	previous := logger
	logger = slog.New(h)
	mu.Unlock()
	return func() {
		mu.Lock()
		logger = previous
		mu.Unlock()
	}
}
