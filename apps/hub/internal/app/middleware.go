package app

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriterWrapper struct {
	http.ResponseWriter

	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriterWrapper) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseWriterWrapper) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// RequestLogger returns a middleware that logs structured details of every HTTP request.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapper := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapper, r)

			duration := time.Since(start)

			ip := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				ip = xff
			}

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapper.statusCode),
				slog.Float64("duration_ms", float64(duration.Microseconds())/1000.0),
				slog.String("ip", ip),
				slog.String("user_agent", r.UserAgent()),
			}

			ctx := r.Context()
			switch {
			case wrapper.statusCode >= 500:
				logger.ErrorContext(ctx, "http request error", attrs...)
			case wrapper.statusCode >= 400:
				logger.WarnContext(ctx, "http request warning", attrs...)
			default:
				logger.InfoContext(ctx, "http request", attrs...)
			}
		})
	}
}
