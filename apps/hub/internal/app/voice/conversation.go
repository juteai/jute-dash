package voice

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	defaultVoiceFollowupWindow = 8 * time.Second
	maxVoiceSessionDuration    = 45 * time.Second
	MaxConversationTurns       = 5
)

var (
	ErrFollowupExpired        = errors.New("voice follow-up window expired")
	ErrFollowupSourceMismatch = errors.New("voice follow-up source mismatch")
)

type ConversationSession struct {
	ConversationID  string
	DeviceProfileID string
	DeviceID        string
	StartedAt       time.Time
	ExpiresAt       time.Time
	Turns           int
}

type ConversationRuntime struct {
	mu       sync.Mutex
	now      func() time.Time
	sessions map[string]ConversationSession
}

func NewConversationRuntime() *ConversationRuntime {
	return &ConversationRuntime{
		now:      func() time.Time { return time.Now().UTC() },
		sessions: map[string]ConversationSession{},
	}
}

func (r *ConversationRuntime) BeginTurn(
	conversationID string,
	settings Settings,
	deviceProfileID string,
	deviceID string,
) (ConversationSession, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	conversationID = strings.TrimSpace(conversationID)
	deviceProfileID = normalizeVoiceSourceID(deviceProfileID, settings.DeviceProfileID)
	deviceID = normalizeVoiceSourceID(deviceID, deviceProfileID)
	if conversationID == "" {
		conversationID = newID("voice-conversation")
		session := ConversationSession{
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
		return ConversationSession{}, false, ErrFollowupExpired
	}
	if !sameVoiceSource(session, deviceProfileID, deviceID) {
		return ConversationSession{}, false, ErrFollowupSourceMismatch
	}
	return session, false, nil
}

func (r *ConversationRuntime) CompleteTurn(
	conversationID string,
	settings Settings,
) ConversationSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	session, ok := r.sessions[conversationID]
	if !ok {
		session = ConversationSession{
			ConversationID: conversationID,
			StartedAt:      now,
		}
	}
	session.Turns++
	session.ExpiresAt = now.Add(followupWindow(settings))
	r.sessions[conversationID] = session
	return session
}

func (r *ConversationRuntime) End(conversationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, strings.TrimSpace(conversationID))
}

func (r *ConversationRuntime) CancelAll() []CancelledConversation {
	r.mu.Lock()
	defer r.mu.Unlock()
	cancelled := make([]CancelledConversation, 0, len(r.sessions))
	for _, session := range r.sessions {
		cancelled = append(cancelled, CancelledConversation{
			ConversationID: session.ConversationID,
			DeviceID:       session.DeviceID,
		})
	}
	r.sessions = map[string]ConversationSession{}
	return cancelled
}

func ConversationComplete(session ConversationSession) bool {
	return session.Turns >= MaxConversationTurns
}

func sessionExpired(session ConversationSession, now time.Time) bool {
	return !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) ||
		!session.StartedAt.IsZero() && now.Sub(session.StartedAt) >= maxVoiceSessionDuration ||
		session.Turns >= MaxConversationTurns
}

func followupWindow(settings Settings) time.Duration {
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

func sameVoiceSource(session ConversationSession, deviceProfileID, deviceID string) bool {
	return strings.TrimSpace(session.DeviceProfileID) == strings.TrimSpace(deviceProfileID) &&
		strings.TrimSpace(session.DeviceID) == strings.TrimSpace(deviceID)
}
