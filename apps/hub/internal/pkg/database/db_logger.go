package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SlogLogger implements gorm/logger.Interface using slog.
type SlogLogger struct {
	Logger        *slog.Logger
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

// NewSlogLogger creates a new SlogLogger instance wrapping the provided [slog.Logger].
func NewSlogLogger(l *slog.Logger) *SlogLogger {
	return &SlogLogger{
		Logger:        l,
		LogLevel:      logger.Warn, // By default, only log Warn/Error/Slow queries
		SlowThreshold: 200 * time.Millisecond,
	}
}

// LogMode sets the logging level.
func (l *SlogLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info logs messages at Info level.
func (l *SlogLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.Logger.InfoContext(ctx, fmt.Sprintf(msg, data...))
	}
}

// Warn logs messages at Warn level.
func (l *SlogLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.Logger.WarnContext(ctx, fmt.Sprintf(msg, data...))
	}
}

// Error logs messages at Error level.
func (l *SlogLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.Logger.ErrorContext(ctx, fmt.Sprintf(msg, data...))
	}
}

// Trace logs SQL query details including duration and errors.
func (l *SlogLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	attrs := []any{
		slog.String("sql", sql),
		slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000.0),
		slog.Int64("rows", rows),
	}

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		attrs = append(attrs, slog.Any("error", err))
		l.Logger.ErrorContext(ctx, "database query error", attrs...)
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		attrs = append(attrs, slog.Duration("slow_threshold", l.SlowThreshold))
		l.Logger.WarnContext(ctx, "database slow query", attrs...)
	case l.LogLevel >= logger.Info:
		l.Logger.DebugContext(ctx, "database query", attrs...)
	}
}
