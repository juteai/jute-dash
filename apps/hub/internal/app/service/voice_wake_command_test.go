package service

import (
	"errors"
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
