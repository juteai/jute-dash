package app

import (
	"errors"
	"testing"
	"time"

	"jute-dash/apps/hub/internal/app/voice"
)

func TestVoiceConversationRuntimeExpiresFollowupWindow(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	runtime := newVoiceConversationRuntime()
	runtime.now = func() time.Time { return now }
	settings := voice.Settings{FollowupWindowSeconds: 8}

	session, started, err := runtime.beginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	if !started {
		t.Fatal("expected new session to be started")
	}
	runtime.completeTurn(session.ConversationID, settings)

	now = now.Add(9 * time.Second)
	_, _, err = runtime.beginTurn(session.ConversationID, settings, "default-display", "kitchen-display")
	if !errors.Is(err, errVoiceFollowupExpired) {
		t.Fatalf("expected follow-up expiry, got %v", err)
	}
}

func TestVoiceConversationRuntimeStopsAfterMaxTurns(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	runtime := newVoiceConversationRuntime()
	runtime.now = func() time.Time { return now }
	settings := voice.Settings{FollowupWindowSeconds: 8}

	session, _, err := runtime.beginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	for range maxVoiceSessionTurns {
		runtime.completeTurn(session.ConversationID, settings)
	}

	_, _, err = runtime.beginTurn(session.ConversationID, settings, "default-display", "kitchen-display")
	if !errors.Is(err, errVoiceFollowupExpired) {
		t.Fatalf("expected max-turn expiry, got %v", err)
	}
}

func TestVoiceConversationRuntimeStopsAfterMaximumSessionDuration(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	runtime := newVoiceConversationRuntime()
	runtime.now = func() time.Time { return now }
	settings := voice.Settings{FollowupWindowSeconds: 30}

	session, _, err := runtime.beginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	runtime.completeTurn(session.ConversationID, settings)

	now = now.Add(maxVoiceSessionDuration)
	_, _, err = runtime.beginTurn(session.ConversationID, settings, "default-display", "kitchen-display")
	if !errors.Is(err, errVoiceFollowupExpired) {
		t.Fatalf("expected maximum-duration expiry, got %v", err)
	}
}

func TestVoiceConversationRuntimeRejectsMismatchedFollowupSource(t *testing.T) {
	runtime := newVoiceConversationRuntime()
	settings := voice.Settings{DeviceProfileID: "default-display", FollowupWindowSeconds: 8}

	session, _, err := runtime.beginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	runtime.completeTurn(session.ConversationID, settings)

	_, _, err = runtime.beginTurn(session.ConversationID, settings, "kitchen-voice", "sat-kitchen")
	if !errors.Is(err, errVoiceFollowupSourceMismatch) {
		t.Fatalf("expected source mismatch, got %v", err)
	}
}
