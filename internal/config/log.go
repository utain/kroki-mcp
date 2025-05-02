package config

import (
	"log/slog"
	"os"
	"strings"
)

func InitLogger(level, format string) *slog.Logger {
	var handler slog.Handler

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: parseLevel(level),
		})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: parseLevel(level),
		})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
