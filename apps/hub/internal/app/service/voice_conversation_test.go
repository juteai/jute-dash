package service

import (
	"errors"
	"testing"
	"time"
)

func TestVoiceConversationRuntimeExpiresFollowupWindow(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	runtime := NewConversationRuntime()
	runtime.now = func() time.Time { return now }
	settings := Settings{FollowupWindowSeconds: 8}

	session, started, err := runtime.BeginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	if !started {
		t.Fatal("expected new session to be started")
	}
	runtime.CompleteTurn(session.ConversationID, settings)

	now = now.Add(9 * time.Second)
	_, _, err = runtime.BeginTurn(session.ConversationID, settings, "default-display", "kitchen-display")
	if !errors.Is(err, ErrFollowupExpired) {
		t.Fatalf("expected follow-up expiry, got %v", err)
	}
}

func TestVoiceConversationRuntimeStopsAfterMaxTurns(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	runtime := NewConversationRuntime()
	runtime.now = func() time.Time { return now }
	settings := Settings{FollowupWindowSeconds: 8}

	session, _, err := runtime.BeginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	for range MaxConversationTurns {
		runtime.CompleteTurn(session.ConversationID, settings)
	}

	_, _, err = runtime.BeginTurn(session.ConversationID, settings, "default-display", "kitchen-display")
	if !errors.Is(err, ErrFollowupExpired) {
		t.Fatalf("expected max-turn expiry, got %v", err)
	}
}

func TestVoiceConversationRuntimeStopsAfterMaximumSessionDuration(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	runtime := NewConversationRuntime()
	runtime.now = func() time.Time { return now }
	settings := Settings{FollowupWindowSeconds: 30}

	session, _, err := runtime.BeginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	runtime.CompleteTurn(session.ConversationID, settings)

	now = now.Add(maxVoiceSessionDuration)
	_, _, err = runtime.BeginTurn(session.ConversationID, settings, "default-display", "kitchen-display")
	if !errors.Is(err, ErrFollowupExpired) {
		t.Fatalf("expected maximum-duration expiry, got %v", err)
	}
}

func TestVoiceConversationRuntimeRejectsMismatchedFollowupSource(t *testing.T) {
	runtime := NewConversationRuntime()
	settings := Settings{DeviceProfileID: "default-display", FollowupWindowSeconds: 8}

	session, _, err := runtime.BeginTurn("", settings, "default-display", "kitchen-display")
	if err != nil {
		t.Fatalf("beginTurn() error = %v", err)
	}
	runtime.CompleteTurn(session.ConversationID, settings)

	_, _, err = runtime.BeginTurn(session.ConversationID, settings, "kitchen-voice", "sat-kitchen")
	if !errors.Is(err, ErrFollowupSourceMismatch) {
		t.Fatalf("expected source mismatch, got %v", err)
	}
}
