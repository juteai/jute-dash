package voice

import (
	"testing"
	"time"
)

func TestFinalTranscriptFromSTTRedactsProviderResult(t *testing.T) {
	transcript, err := FinalTranscriptFromSTT(
		STTResult{
			Text:       "turn on token=secret kitchen lights",
			ProviderID: "local-stt",
			ModelID:    "tiny-en",
			Language:   "en-GB",
			Duration:   20 * time.Millisecond,
		},
		"default-display",
		"kitchen-display",
	)
	if err != nil {
		t.Fatalf("FinalTranscriptFromSTT() error = %v", err)
	}
	if transcript.Text != "turn on token=[redacted] kitchen lights" ||
		transcript.DeviceProfileID != "default-display" ||
		transcript.DeviceID != "kitchen-display" ||
		transcript.ProviderID != "local-stt" ||
		transcript.ModelID != "tiny-en" ||
		transcript.Language != "en-GB" ||
		transcript.Duration != 20*time.Millisecond {
		t.Fatalf("unexpected transcript: %+v", transcript)
	}
}

func TestFinalTranscriptFromSTTRejectsEmptyTranscript(t *testing.T) {
	_, err := FinalTranscriptFromSTT(STTResult{Text: "   "}, "default-display", "kitchen-display")
	if err == nil || err.Error() != "STT transcript is empty" {
		t.Fatalf("expected empty transcript error, got %v", err)
	}
}
