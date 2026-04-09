package logging

import "log/slog"

var logger = slog.Default()

func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}
