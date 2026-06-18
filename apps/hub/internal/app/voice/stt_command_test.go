package voice

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestCommandSTTProviderRunsAbsoluteCommandWithInputPathModelAndLanguage(t *testing.T) {
	fixture, err := NewBenchmarkToneFixture("short-command", "fixture", BenchmarkAudioSpec{
		Duration: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	result, err := (CommandSTTProvider{
		ProviderID: "go-whisper-command",
		Command:    os.Args[0],
		Args: []string{
			"-test.run=TestCommandSTTProviderHelper",
			"--",
			"{inputPath}",
			"{modelId}",
			"{language}",
		},
		ModelID:  "tiny-en",
		Language: "en-GB",
	}).Transcribe(t.Context(), fixture.Utterance)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "turn on the lights" ||
		result.ProviderID != "go-whisper-command" ||
		result.ModelID != "tiny-en" ||
		result.Language != "en-GB" ||
		result.Duration != 42*time.Millisecond {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCommandSTTProviderHelper(t *testing.T) {
	inputPath, modelID, language := helperArgs()
	if inputPath == "" {
		return
	}
	if modelID != "tiny-en" {
		t.Fatalf("unexpected model id: %q", modelID)
	}
	if language != "en-GB" {
		t.Fatalf("unexpected language: %q", language)
	}
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	if len(raw) < 4 || string(raw[:4]) != "RIFF" {
		t.Fatalf("expected WAV input, len=%d", len(raw))
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"text":       "turn on the lights",
		"providerId": "go-whisper-command",
		"modelId":    modelID,
		"language":   "en-GB",
		"durationMs": 42,
	})
	os.Exit(0)
}

func helperArgs() (string, string, string) {
	for i, arg := range os.Args {
		if arg == "--" && i+3 < len(os.Args) {
			return os.Args[i+1], os.Args[i+2], os.Args[i+3]
		}
	}
	return "", "", ""
}
