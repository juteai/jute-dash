package voice

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDispatcherEmitsVoiceTranscriptEventShapeAndRedactsSecrets(t *testing.T) {
	dispatcher := NewDispatcher()
	dispatcher.now = func() time.Time {
		return time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := dispatcher.Subscribe(ctx)

	event := dispatcher.EmitVoiceTranscript(
		EventVoiceTranscriptFinal,
		" default-display ",
		" conversation-1 ",
		"Bearer token=abc123 please turn on the light",
	)

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	body := string(raw)
	for _, expected := range []string{
		`"type":"voice.transcript.final"`,
		`"createdAt":"2026-06-13T12:00:00Z"`,
		`"deviceId":"default-display"`,
		`"conversationId":"conversation-1"`,
		`"payload":{"text":"Bearer token=[redacted] please turn on the light"}`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected event to contain %s, got %s", expected, body)
		}
	}
	if strings.Contains(body, "abc123") {
		t.Fatalf("event leaked secret material: %s", body)
	}

	published := <-events
	if published.Type != EventVoiceTranscriptFinal {
		t.Fatalf("unexpected published event type: %s", published.Type)
	}
}

func TestDispatcherSanitizesConversationPayload(t *testing.T) {
	dispatcher := NewDispatcher()

	event := dispatcher.EmitConversationEvent(
		EventConversationTurnCompleted,
		"display-1",
		"conversation-1",
		map[string]any{
			"turnId":        "turn-1",
			"status":        "completed",
			"authorization": "Bearer super-secret",
			"headers": map[string]string{
				"provider":      "local-agent",
				"authorization": "Bearer header-secret",
				"summary":       "token=abc123",
			},
			"notes": []string{"ok", "api_key=note-secret"},
			"nested": map[string]any{
				"message": "apiKey=secret-value",
			},
		},
	)

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	body := string(raw)
	for _, leaked := range []string{"super-secret", "header-secret", "abc123", "note-secret", "secret-value"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("event leaked %q: %s", leaked, body)
		}
	}
	for _, expected := range []string{
		`"type":"conversation.turn_completed"`,
		`"deviceId":"display-1"`,
		`"conversationId":"conversation-1"`,
		`"authorization":"[redacted]"`,
		`"provider":"local-agent"`,
		`"summary":"token=[redacted]"`,
		`"notes":["ok","api_key=[redacted]"]`,
		`"message":"apiKey=[redacted]"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected event to contain %s, got %s", expected, body)
		}
	}
}

func TestDispatcherEmitsSafeTTSEvents(t *testing.T) {
	dispatcher := NewDispatcher()

	event := dispatcher.EmitTTSEvent(EventTTSFailed, "display-1", TTSActionResponse{
		Action:         TTSActionSpeak,
		State:          TTSStateVisualOnly,
		ProviderID:     "provider token=secret",
		VoiceID:        "voice-1",
		ConversationID: "conversation-1",
		CacheEligible:  false,
		Reason:         "apiKey=secret-value",
	})

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	body := string(raw)
	for _, leaked := range []string{"secret-value", "token=secret"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("event leaked %q: %s", leaked, body)
		}
	}
	for _, expected := range []string{
		`"type":"tts.failed"`,
		`"deviceId":"display-1"`,
		`"conversationId":"conversation-1"`,
		`"state":"visual_only"`,
		`"providerId":"provider token=[redacted]"`,
		`"reason":"apiKey=[redacted]"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected event to contain %s, got %s", expected, body)
		}
	}
}
