package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jgabor/spela/internal/config"
)

var logger *slog.Logger

func Init(level config.LogLevel, verbose bool) error {
	logDir := filepath.Join(os.Getenv("HOME"), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	logFile, err := os.OpenFile(
		filepath.Join(logDir, "spela.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o644,
	)
	if err != nil {
		return err
	}

	var slogLevel slog.Level
	switch level {
	case config.LogLevelDebug:
		slogLevel = slog.LevelDebug
	case config.LogLevelWarn:
		slogLevel = slog.LevelWarn
	case config.LogLevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	if verbose {
		slogLevel = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: slogLevel}

	var writer io.Writer
	if verbose {
		writer = io.MultiWriter(os.Stderr, logFile)
	} else {
		writer = logFile
	}

	logger = slog.New(slog.NewTextHandler(writer, opts))
	return nil
}

func Debug(msg string, args ...any) {
	logger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}

func With(args ...any) *slog.Logger {
	return logger.With(args...)
}
