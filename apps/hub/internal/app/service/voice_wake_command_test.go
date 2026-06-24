package service

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestDecodeCommandWakeOutputUsesProviderDefaults(t *testing.T) {
	detection, err := decodeCommandWakeOutput(
		[]byte(`{"detected":true,"confidence":0.91}`),
		CommandWakeProvider{ProviderID: "wake-local", ModelID: "hey-jute"},
	)
	if err != nil {
		t.Fatalf("decodeCommandWakeOutput() error = %v", err)
	}
	if !detection.Detected ||
		detection.ProviderID != "wake-local" ||
		detection.ModelID != "hey-jute" ||
		detection.Confidence != 0.91 {
		t.Fatalf("unexpected detection: %+v", detection)
	}
}

func TestDecodeCommandWakeOutputRejectsTrailingJSON(t *testing.T) {
	_, err := decodeCommandWakeOutput(
		[]byte(`{"detected":true}{"detected":false}`),
		CommandWakeProvider{},
	)
	if !errors.Is(err, errCommandWakeProviderUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestCommandWakeProviderRejectsMissingAudioBeforeCommand(t *testing.T) {
	_, err := (CommandWakeProvider{Command: "wake"}).DetectWake(t.Context(), CapturedUtterance{})
	if err == nil || err.Error() != "utterance audio is required" {
		t.Fatalf("expected missing audio error, got %v", err)
	}
}

func TestCommandWakeProviderLogsCommandStderr(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	t.Setenv("JUTE_WAKE_FAIL_HELPER", "1")

	_, err := (CommandWakeProvider{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestCommandWakeProviderFailureHelper"},
		ModelID: "hey_jarvis",
	}).DetectWake(t.Context(), testUtterance())
	if !errors.Is(err, errCommandWakeProviderUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
	if got := logs.String(); !strings.Contains(got, "voice command failed") ||
		!strings.Contains(got, "wake stderr") ||
		!strings.Contains(got, "hey_jarvis") {
		t.Fatalf("expected wake command failure log, got:\n%s", got)
	}
}

func TestCommandWakeProviderFailureHelper(t *testing.T) {
	if os.Getenv("JUTE_WAKE_FAIL_HELPER") == "1" {
		fmt.Fprint(os.Stderr, "wake stderr")
		os.Exit(3)
	}
}
