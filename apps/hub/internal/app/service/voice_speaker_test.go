package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

type testVoiceStore struct {
	settings Settings
}

func (s testVoiceStore) VoiceSettings(context.Context, string) (Settings, error) {
	return s.settings, nil
}

type testTTSProvider struct {
	called bool
	req    TTSRequest
}

func (p *testTTSProvider) Synthesize(_ context.Context, req TTSRequest) (TTSAudioResult, error) {
	p.called = true
	p.req = req
	return TTSAudioResult{
		Audio:        []byte{1, 2, 3},
		ProviderID:   "local",
		VoiceID:      req.VoiceID,
		Locale:       req.Locale,
		ContentType:  "audio/wav",
		SampleRate:   16000,
		Channels:     1,
		Duration:     250 * time.Millisecond,
		PlaybackKind: "inline",
	}, nil
}

type testVoiceDisplay struct {
	types []string
}

func (d *testVoiceDisplay) EmitVoiceStateChanged(string, VoiceStatePayload) VoiceEvent {
	return VoiceEvent{}
}

func (d *testVoiceDisplay) EmitConversationEvent(string, string, string, any) VoiceEvent {
	return VoiceEvent{}
}

func (d *testVoiceDisplay) EmitTTSEvent(eventType, _ string, _ TTSActionResponse) VoiceEvent {
	d.types = append(d.types, eventType)
	return VoiceEvent{Type: eventType}
}

func TestSpeakerSpeakUsesProviderAndEmitsLifecycle(t *testing.T) {
	provider := &testTTSProvider{}
	display := &testVoiceDisplay{}
	speaker := NewSpeaker(testVoiceStore{settings: Settings{
		TTSEnabled:    true,
		TTSProviderID: "local",
		TTSVoiceID:    "amy",
		TTSLocale:     "en-GB",
	}}, display, provider)

	response, err := speaker.Speak(t.Context(), "display-1", TTSActionSpeak, TTSRequest{Text: "hello"})
	if err != nil {
		t.Fatalf("Speak() error = %v", err)
	}
	if !provider.called || provider.req.VoiceID != "amy" || provider.req.Locale != "en-GB" {
		t.Fatalf("provider was not called with effective settings: called=%v req=%+v", provider.called, provider.req)
	}
	if response.State != TTSStateCompleted || response.AudioBytes != 3 || response.DurationMs != 250 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(display.types) != 2 || display.types[0] != EventTTSStarted || display.types[1] != EventTTSCompleted {
		t.Fatalf("unexpected events: %+v", display.types)
	}
}

func TestSpeakerSpeakSanitizesMarkdownBeforeProvider(t *testing.T) {
	provider := &testTTSProvider{}
	speaker := NewSpeaker(testVoiceStore{settings: Settings{
		TTSEnabled:    true,
		TTSProviderID: "local",
	}}, nil, provider)
	text := strings.Join([]string{
		"## Weather",
		"",
		"- **Cloudy** with `22 C`.",
		"- [More detail](https://example.com)",
		"",
		"```json",
		`{"raw":true}`,
		"```",
	}, "\n")

	_, err := speaker.Speak(t.Context(), "display-1", TTSActionSpeak, TTSRequest{Text: text})
	if err != nil {
		t.Fatalf("Speak() error = %v", err)
	}
	if provider.req.Text != "Weather Cloudy with 22 C. More detail Code omitted." {
		t.Fatalf("unexpected speech text: %q", provider.req.Text)
	}
}

func TestSpeakerSpeakRemovesReasoningBeforeProvider(t *testing.T) {
	provider := &testTTSProvider{}
	speaker := NewSpeaker(testVoiceStore{settings: Settings{
		TTSEnabled:    true,
		TTSProviderID: "local",
	}}, nil, provider)

	_, err := speaker.Speak(
		t.Context(),
		"display-1",
		TTSActionSpeak,
		TTSRequest{
			Text: "Okay, the user asked for weather. I should call a tool.\n\nIt is 22 C and sunny.",
		},
	)
	if err != nil {
		t.Fatalf("Speak() error = %v", err)
	}
	if provider.req.Text != "It is 22 C and sunny." {
		t.Fatalf("unexpected speech text: %q", provider.req.Text)
	}
}

func TestSpeakerSpeakSkipsReasoningOnlyProviderCall(t *testing.T) {
	provider := &testTTSProvider{}
	speaker := NewSpeaker(testVoiceStore{settings: Settings{
		TTSEnabled:    true,
		TTSProviderID: "local",
	}}, nil, provider)

	response, err := speaker.Speak(
		t.Context(),
		"display-1",
		TTSActionSpeak,
		TTSRequest{Text: "<think>I should inspect the weather widget.</think>"},
	)
	if err != nil {
		t.Fatalf("Speak() error = %v", err)
	}
	if provider.called {
		t.Fatal("provider should not be called for reasoning-only speech")
	}
	if response.State != TTSStateVisualOnly || response.Reason != "speech_text_empty" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestSpeakerSensitiveSpeechDoesNotCallProvider(t *testing.T) {
	provider := &testTTSProvider{}
	speaker := NewSpeaker(testVoiceStore{settings: Settings{
		SensitiveOutputPolicy: TTSPolicyVisualOnlySensitive,
	}}, nil, provider)

	response, err := speaker.Speak(t.Context(), "display-1", TTSActionSpeak, TTSRequest{Text: "token=secret"})
	if err != nil {
		t.Fatalf("Speak() error = %v", err)
	}
	if provider.called {
		t.Fatal("provider should not be called for visual-only sensitive output")
	}
	if response.State != TTSStateVisualOnly || response.Reason != "sensitive_output_visual_only" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
