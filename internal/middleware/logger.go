package middleware

import (
	"log/slog"
	"os"
)

type LoggerConfig struct {
	Level  slog.Level
	Format string // "json" or "text"
}

func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:  slog.LevelInfo,
		Format: "json",
	}
}

func SetupLogger(cfg LoggerConfig) {
	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
