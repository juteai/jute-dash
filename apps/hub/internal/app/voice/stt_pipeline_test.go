package voice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSTTTurnProcessorSubmitsSafeFinalTranscript(t *testing.T) {
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	utterance := CapturedUtterance{
		Frames: []AudioFrame{{
			PCM:         []byte{1, 2, 3, 4},
			SampleRate:  16000,
			SampleWidth: 2,
			Channels:    1,
			Timestamp:   start,
			Duration:    20 * time.Millisecond,
		}},
		StartedAt:  start,
		EndedAt:    start.Add(20 * time.Millisecond),
		SampleRate: 16000,
		Channels:   1,
	}
	provider := &fixtureSTTTurnProvider{
		result: STTResult{
			Text:       "turn on token=secret kitchen lights",
			ProviderID: "local-stt",
			ModelID:    "tiny-en",
			Language:   "en-GB",
			Duration:   20 * time.Millisecond,
		},
	}
	sink := &recordingFinalTranscriptSink{}
	processor := NewSTTTurnProcessor(provider, sink, "default-display", "kitchen-display")

	transcript, err := processor.Process(context.Background(), utterance)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
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
	if len(sink.submitted) != 1 || sink.submitted[0] != transcript {
		t.Fatalf("sink did not receive transcript: %+v", sink.submitted)
	}
	provider.seen.Frames[0].PCM[0] = 99
	if utterance.Frames[0].PCM[0] == 99 {
		t.Fatal("provider received mutable utterance audio")
	}
}

func TestSTTTurnProcessorHandlesProviderFailureSafely(t *testing.T) {
	provider := &fixtureSTTTurnProvider{
		err: errors.New("dial tcp 127.0.0.1:10300: token=secret unavailable"),
	}
	sink := &recordingFinalTranscriptSink{}
	processor := NewSTTTurnProcessor(provider, sink, "default-display", "kitchen-display")

	_, err := processor.Process(context.Background(), fixtureSTTTurnUtterance())

	if err == nil || err.Error() != "STT provider unavailable" {
		t.Fatalf("expected safe provider failure, got %v", err)
	}
	if len(sink.submitted) != 0 {
		t.Fatalf("provider failure submitted transcript: %+v", sink.submitted)
	}
	if strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "token=secret") {
		t.Fatalf("error leaked provider details: %v", err)
	}
}

func TestSTTTurnProcessorRejectsEmptyTranscript(t *testing.T) {
	provider := &fixtureSTTTurnProvider{result: STTResult{Text: "   "}}
	sink := &recordingFinalTranscriptSink{}
	processor := NewSTTTurnProcessor(provider, sink, "default-display", "kitchen-display")

	_, err := processor.Process(context.Background(), fixtureSTTTurnUtterance())

	if err == nil || err.Error() != "STT transcript is empty" {
		t.Fatalf("expected empty transcript error, got %v", err)
	}
	if len(sink.submitted) != 0 {
		t.Fatalf("empty transcript submitted: %+v", sink.submitted)
	}
}

func TestSTTTurnProcessorRequiresProviderAndSink(t *testing.T) {
	if _, err := NewSTTTurnProcessor(nil, &recordingFinalTranscriptSink{}, "", "").Process(
		context.Background(),
		fixtureSTTTurnUtterance(),
	); err == nil || err.Error() != "STT provider is unavailable" {
		t.Fatalf("expected missing provider error, got %v", err)
	}
	if _, err := NewSTTTurnProcessor(&fixtureSTTTurnProvider{}, nil, "", "").Process(
		context.Background(),
		fixtureSTTTurnUtterance(),
	); err == nil || err.Error() != "final transcript sink is unavailable" {
		t.Fatalf("expected missing sink error, got %v", err)
	}
}

type fixtureSTTTurnProvider struct {
	seen   CapturedUtterance
	result STTResult
	err    error
}

func (p *fixtureSTTTurnProvider) Transcribe(_ context.Context, utterance CapturedUtterance) (STTResult, error) {
	p.seen = utterance
	if p.err != nil {
		return STTResult{}, p.err
	}
	return p.result, nil
}

type recordingFinalTranscriptSink struct {
	submitted []FinalTranscript
}

func (s *recordingFinalTranscriptSink) SubmitFinalTranscript(_ context.Context, transcript FinalTranscript) error {
	s.submitted = append(s.submitted, transcript)
	return nil
}

func fixtureSTTTurnUtterance() CapturedUtterance {
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	return CapturedUtterance{
		Frames: []AudioFrame{{
			PCM:         []byte{1, 2},
			SampleRate:  16000,
			SampleWidth: 2,
			Channels:    1,
			Timestamp:   start,
			Duration:    20 * time.Millisecond,
		}},
		StartedAt:  start,
		EndedAt:    start.Add(20 * time.Millisecond),
		SampleRate: 16000,
		Channels:   1,
	}
}
