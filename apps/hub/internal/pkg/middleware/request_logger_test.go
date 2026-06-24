package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggerRecordsStatusAndForwardedIP(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{}))
	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodGet, "/tea", nil)
	req.Header.Set("X-Forwarded-For", "192.0.2.1")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	logLine := buf.String()
	for _, want := range []string{
		"msg=\"http request warning\"",
		"method=GET",
		"path=/tea",
		"status=418",
		"ip=192.0.2.1",
	} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line missing %q: %s", want, logLine)
		}
	}
}
