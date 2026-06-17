package app

import (
	"errors"
	"strings"
	"sync"
	"time"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/voice"
)

const (
	defaultVoiceFollowupWindow = 8 * time.Second
	maxVoiceSessionDuration    = 45 * time.Second
	maxVoiceSessionTurns       = 5
)

var (
	errVoiceFollowupExpired        = errors.New("voice follow-up window expired")
	errVoiceFollowupSourceMismatch = errors.New("voice follow-up source mismatch")
)

type voiceConversationSession struct {
	ConversationID  string
	DeviceProfileID string
	DeviceID        string
	StartedAt       time.Time
	ExpiresAt       time.Time
	Turns           int
}

type voiceConversationRuntime struct {
	mu       sync.Mutex
	now      func() time.Time
	sessions map[string]voiceConversationSession
}

func newVoiceConversationRuntime() *voiceConversationRuntime {
	return &voiceConversationRuntime{
		now:      func() time.Time { return time.Now().UTC() },
		sessions: map[string]voiceConversationSession{},
	}
}

func (r *voiceConversationRuntime) beginTurn(
	conversationID string,
	settings voice.Settings,
	deviceProfileID string,
	deviceID string,
) (voiceConversationSession, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	conversationID = strings.TrimSpace(conversationID)
	deviceProfileID = normalizeVoiceSourceID(deviceProfileID, settings.DeviceProfileID)
	deviceID = normalizeVoiceSourceID(deviceID, deviceProfileID)
	if conversationID == "" {
		conversationID = "voice-conversation-" + agents.NewLocalID()
		session := voiceConversationSession{
			ConversationID:  conversationID,
			DeviceProfileID: deviceProfileID,
			DeviceID:        deviceID,
			StartedAt:       now,
			ExpiresAt:       now.Add(followupWindow(settings)),
		}
		r.sessions[conversationID] = session
		return session, true, nil
	}

	session, ok := r.sessions[conversationID]
	if !ok || sessionExpired(session, now) {
		delete(r.sessions, conversationID)
		return voiceConversationSession{}, false, errVoiceFollowupExpired
	}
	if !sameVoiceSource(session, deviceProfileID, deviceID) {
		return voiceConversationSession{}, false, errVoiceFollowupSourceMismatch
	}
	return session, false, nil
}

func (r *voiceConversationRuntime) completeTurn(
	conversationID string,
	settings voice.Settings,
) voiceConversationSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	session, ok := r.sessions[conversationID]
	if !ok {
		session = voiceConversationSession{
			ConversationID: conversationID,
			StartedAt:      now,
		}
	}
	session.Turns++
	session.ExpiresAt = now.Add(followupWindow(settings))
	r.sessions[conversationID] = session
	return session
}

func (r *voiceConversationRuntime) end(conversationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, strings.TrimSpace(conversationID))
}

func (r *voiceConversationRuntime) cancelAll() []voice.CancelledConversation {
	r.mu.Lock()
	defer r.mu.Unlock()
	cancelled := make([]voice.CancelledConversation, 0, len(r.sessions))
	for _, session := range r.sessions {
		cancelled = append(cancelled, voice.CancelledConversation{
			ConversationID: session.ConversationID,
			DeviceID:       session.DeviceID,
		})
	}
	r.sessions = map[string]voiceConversationSession{}
	return cancelled
}

func sessionExpired(session voiceConversationSession, now time.Time) bool {
	return !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) ||
		!session.StartedAt.IsZero() && now.Sub(session.StartedAt) >= maxVoiceSessionDuration ||
		session.Turns >= maxVoiceSessionTurns
}

func followupWindow(settings voice.Settings) time.Duration {
	if settings.FollowupWindowSeconds <= 0 {
		return defaultVoiceFollowupWindow
	}
	window := time.Duration(settings.FollowupWindowSeconds) * time.Second
	if window > maxVoiceSessionDuration {
		return maxVoiceSessionDuration
	}
	return window
}

func normalizeVoiceSourceID(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}

func sameVoiceSource(session voiceConversationSession, deviceProfileID, deviceID string) bool {
	return strings.TrimSpace(session.DeviceProfileID) == strings.TrimSpace(deviceProfileID) &&
		strings.TrimSpace(session.DeviceID) == strings.TrimSpace(deviceID)
}
