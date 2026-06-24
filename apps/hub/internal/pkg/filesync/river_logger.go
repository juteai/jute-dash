package filesync

import (
	"context"
	"log/slog"
)

// RiverLevelFilterHandler wraps a [slog.Handler] to demote LevelInfo logs to LevelDebug
// to keep the log file clean from frequent background config sync completions.
type RiverLevelFilterHandler struct {
	slog.Handler
}

// Enabled returns true if the handler enables the demoted level.
func (h *RiverLevelFilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level == slog.LevelInfo {
		level = slog.LevelDebug
	}
	return h.Handler.Enabled(ctx, level)
}

// Handle demotes LevelInfo records to LevelDebug before delegating.
func (h *RiverLevelFilterHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level == slog.LevelInfo {
		record.Level = slog.LevelDebug
	}
	return h.Handler.Handle(ctx, record)
}
