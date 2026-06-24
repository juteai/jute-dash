package logging

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"jute-dash/apps/hub/internal/app/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

// MultiHandler multiplexes log records to multiple slog.Handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

// Enabled returns true if any handler enables the given level.
func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle sends the log record to all handlers.
func (m *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

// WithAttrs returns a MultiHandler with the attributes added to each handler.
func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nextHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		nextHandlers[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: nextHandlers}
}

// WithGroup returns a MultiHandler with the group added to each handler.
func (m *MultiHandler) WithGroup(name string) slog.Handler {
	nextHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		nextHandlers[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: nextHandlers}
}

// SetupLogger initializes structured logging with slog and a rolling Lumberjack log file.
func SetupLogger(cfg config.LogConfig, dataDir string) (slog.Handler, error) {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 1. Setup Stderr Console Handler (clean Text format)
	consoleHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	// 2. Setup File Handler (structured JSON format)
	logFilePath := cfg.FilePath
	if strings.TrimSpace(logFilePath) == "" {
		logFilePath = filepath.Join(dataDir, "jute.log")
	}

	// Ensure parent directory exists for logs if it's explicitly set
	if dir := filepath.Dir(logFilePath); dir != "" {
		_ = os.MkdirAll(dir, 0o700)
	}

	fileWriter := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		LocalTime:  true,
		Compress:   cfg.Compress,
	}

	fileHandler := slog.NewJSONHandler(fileWriter, &slog.HandlerOptions{
		Level: level,
	})

	// 3. Set multi-handler as default
	multiHandler := &MultiHandler{
		handlers: []slog.Handler{consoleHandler, fileHandler},
	}

	return multiHandler, nil
}
