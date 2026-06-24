package logging

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jute-dash/apps/hub/internal/app/config"
)

func TestSetupLoggerWritesToLogFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test-jute.log")

	cfg := config.LogConfig{
		Level:      "info",
		FilePath:   logFile,
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	handler, err := SetupLogger(cfg, tempDir)
	if err != nil {
		t.Fatalf("SetupLogger failed: %v", err)
	}

	logger := slog.New(handler)
	testMessage := "hello logging world"
	logger.Info(testMessage, "key1", "value1")

	// Ensure the file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatalf("Log file %s was not created", logFile)
	}

	file, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	var found bool
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			t.Errorf("Failed to parse log line as JSON: %s (err: %v)", line, err)
			continue
		}

		if logEntry["msg"] == testMessage {
			found = true
			if logEntry["level"] != "INFO" {
				t.Errorf("Expected level INFO, got %v", logEntry["level"])
			}
			if logEntry["key1"] != "value1" {
				t.Errorf("Expected key1=value1, got key1=%v", logEntry["key1"])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	if !found {
		t.Fatalf("Expected log message %q not found in file", testMessage)
	}
}

func TestSetupLoggerValidatesLogLevels(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test-level.log")

	levels := []struct {
		configured     string
		shouldLogDebug bool
		shouldLogInfo  bool
		shouldLogWarn  bool
	}{
		{"debug", true, true, true},
		{"info", false, true, true},
		{"warn", false, false, true},
		{"error", false, false, false},
	}

	for _, tc := range levels {
		t.Run(tc.configured, func(t *testing.T) {
			cfg := config.LogConfig{
				Level:      tc.configured,
				FilePath:   logFile,
				MaxSize:    1,
				MaxBackups: 1,
				MaxAge:     1,
				Compress:   false,
			}
			_ = os.Remove(logFile)

			handler, err := SetupLogger(cfg, tempDir)
			if err != nil {
				t.Fatalf("SetupLogger failed: %v", err)
			}

			logger := slog.New(handler)
			logger.Debug("debug msg")
			logger.Info("info msg")
			logger.Warn("warn msg")
			logger.Error("error msg") // Error is always logged, ensuring the file is always created

			file, err := os.Open(logFile)
			if err != nil {
				t.Fatalf("Failed to open log file: %v", err)
			}
			defer file.Close()

			var hasDebug, hasInfo, hasWarn, hasError bool
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "debug msg") {
					hasDebug = true
				}
				if strings.Contains(line, "info msg") {
					hasInfo = true
				}
				if strings.Contains(line, "warn msg") {
					hasWarn = true
				}
				if strings.Contains(line, "error msg") {
					hasError = true
				}
			}

			if tc.shouldLogDebug != hasDebug {
				t.Errorf("Expected debug logged = %v, got %v", tc.shouldLogDebug, hasDebug)
			}
			if tc.shouldLogInfo != hasInfo {
				t.Errorf("Expected info logged = %v, got %v", tc.shouldLogInfo, hasInfo)
			}
			if tc.shouldLogWarn != hasWarn {
				t.Errorf("Expected warn logged = %v, got %v", tc.shouldLogWarn, hasWarn)
			}
			if !hasError {
				t.Error("Expected error log to always be written")
			}
		})
	}
}
