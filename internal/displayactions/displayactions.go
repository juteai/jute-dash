package displayactions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	EventNotification = "display.notification"
	EventFocusWidget  = "display.focus_widget"

	EventVoiceStateChanged           = "voice.state_changed"
	EventVoiceWakeDetected           = "voice.wake_detected"
	EventVoiceTranscriptPartial      = "voice.transcript.partial"
	EventVoiceTranscriptFinal        = "voice.transcript.final"
	EventConversationStarted         = "conversation.started"
	EventConversationTurnStarted     = "conversation.turn_started"
	EventConversationTurnCompleted   = "conversation.turn_completed"
	EventConversationFollowupStarted = "conversation.followup_started"
	EventConversationEnded           = "conversation.ended"

	defaultNotificationTTL = 6 * time.Second
	maxMessageRunes        = 180
	maxReasonRunes         = 120
)

var (
	ErrEmptyMessage = errors.New("notification message is required")
	ErrEmptyWidget  = errors.New("widget instance id is required")

	secretPattern = regexp.MustCompile(`(?i)\b(bearer|token|secret|password|api[_-]?key)\s*[:=]\s*[^,\s]+`)
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Notification struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
}

type FocusWidget struct {
	ID               string `json:"id"`
	WidgetInstanceID string `json:"widgetInstanceId"`
	Reason           string `json:"reason,omitempty"`
	CreatedAt        string `json:"createdAt"`
}

type VoiceStatePayload struct {
	Enabled       bool   `json:"enabled"`
	Muted         bool   `json:"muted"`
	State         string `json:"state"`
	ServiceStatus string `json:"serviceStatus"`
}

type VoiceEvent struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	CreatedAt      string `json:"createdAt"`
	DeviceID       string `json:"deviceId"`
	ConversationID string `json:"conversationId,omitempty"`
	Payload        any    `json:"payload"`
}

type Dispatcher struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
	now         func() time.Time
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		subscribers: map[chan Event]struct{}{},
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (d *Dispatcher) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event, 16)
	d.mu.Lock()
	d.subscribers[ch] = struct{}{}
	d.mu.Unlock()

	go func() {
		<-ctx.Done()
		d.mu.Lock()
		delete(d.subscribers, ch)
		close(ch)
		d.mu.Unlock()
	}()

	return ch
}

func (d *Dispatcher) Notify(message, severity string) (Notification, error) {
	message = sanitizeText(message, maxMessageRunes)
	if message == "" {
		return Notification{}, ErrEmptyMessage
	}
	severity = normalizeSeverity(severity)
	now := d.now().UTC()
	notification := Notification{
		ID:        newID("notification"),
		Message:   message,
		Severity:  severity,
		CreatedAt: now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(defaultNotificationTTL).Format(time.RFC3339Nano),
	}
	d.publish(Event{Type: EventNotification, Data: notification})
	return notification, nil
}

func (d *Dispatcher) FocusWidget(widgetInstanceID, reason string) (FocusWidget, error) {
	widgetInstanceID = strings.TrimSpace(widgetInstanceID)
	if widgetInstanceID == "" {
		return FocusWidget{}, ErrEmptyWidget
	}
	now := d.now().UTC()
	focus := FocusWidget{
		ID:               newID("focus"),
		WidgetInstanceID: widgetInstanceID,
		Reason:           sanitizeText(reason, maxReasonRunes),
		CreatedAt:        now.Format(time.RFC3339Nano),
	}
	d.publish(Event{Type: EventFocusWidget, Data: focus})
	return focus, nil
}

func (d *Dispatcher) EmitVoiceStateChanged(deviceID string, payload VoiceStatePayload) VoiceEvent {
	event := VoiceEvent{
		ID:        newID("voice-state"),
		Type:      EventVoiceStateChanged,
		CreatedAt: d.now().UTC().Format(time.RFC3339Nano),
		DeviceID:  deviceID,
		Payload:   payload,
	}
	d.publish(Event{Type: EventVoiceStateChanged, Data: event})
	return event
}

func (d *Dispatcher) EmitVoiceWakeDetected(deviceID, conversationID string) VoiceEvent {
	event := VoiceEvent{
		ID:             newID("voice-wake"),
		Type:           EventVoiceWakeDetected,
		CreatedAt:      d.now().UTC().Format(time.RFC3339Nano),
		DeviceID:       deviceID,
		ConversationID: conversationID,
		Payload:        map[string]any{},
	}
	d.publish(Event{Type: EventVoiceWakeDetected, Data: event})
	return event
}

func (d *Dispatcher) EmitVoiceTranscript(eventType, deviceID, conversationID, text string) VoiceEvent {
	event := VoiceEvent{
		ID:             newID("voice-transcript"),
		Type:           eventType,
		CreatedAt:      d.now().UTC().Format(time.RFC3339Nano),
		DeviceID:       deviceID,
		ConversationID: conversationID,
		Payload: map[string]any{
			"text": text,
		},
	}
	d.publish(Event{Type: eventType, Data: event})
	return event
}

func (d *Dispatcher) EmitConversationEvent(eventType, deviceID, conversationID string, payload any) VoiceEvent {
	event := VoiceEvent{
		ID:             newID("conversation-event"),
		Type:           eventType,
		CreatedAt:      d.now().UTC().Format(time.RFC3339Nano),
		DeviceID:       deviceID,
		ConversationID: conversationID,
		Payload:        payload,
	}
	d.publish(Event{Type: eventType, Data: event})
	return event
}

func (d *Dispatcher) publish(event Event) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for ch := range d.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func normalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "success", "warning", "error":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "info"
	}
}

func sanitizeText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(secretPattern.ReplaceAllString(strings.TrimSpace(value), "$1=[redacted]")), " ")
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}

func newID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "-" + hex.EncodeToString(bytes[:])
}
