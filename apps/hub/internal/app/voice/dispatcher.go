package voice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"jute-dash/apps/hub/internal/pkg/displayactions"
)

const (
	EventVoiceStateChanged           = "voice.state_changed"
	EventVoiceWakeDetected           = "voice.wake_detected"
	EventVoiceTranscriptPartial      = "voice.transcript.partial"
	EventVoiceTranscriptFinal        = "voice.transcript.final"
	EventConversationStarted         = "conversation.started"
	EventConversationTurnStarted     = "conversation.turn_started"
	EventConversationTurnCompleted   = "conversation.turn_completed"
	EventConversationFollowupStarted = "conversation.followup_started"
	EventConversationEnded           = "conversation.ended"
)

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
	subscribers map[chan displayactions.Event]struct{}
	now         func() time.Time
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		subscribers: map[chan displayactions.Event]struct{}{},
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (d *Dispatcher) Subscribe(ctx context.Context) <-chan displayactions.Event {
	ch := make(chan displayactions.Event, 16)
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

func (d *Dispatcher) EmitVoiceStateChanged(deviceID string, payload VoiceStatePayload) VoiceEvent {
	event := VoiceEvent{
		ID:        newID("voice-state"),
		Type:      EventVoiceStateChanged,
		CreatedAt: d.now().UTC().Format(time.RFC3339Nano),
		DeviceID:  deviceID,
		Payload:   payload,
	}
	d.publish(displayactions.Event{Type: EventVoiceStateChanged, Data: event})
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
	d.publish(displayactions.Event{Type: EventVoiceWakeDetected, Data: event})
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
	d.publish(displayactions.Event{Type: eventType, Data: event})
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
	d.publish(displayactions.Event{Type: eventType, Data: event})
	return event
}

func (d *Dispatcher) publish(event displayactions.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for ch := range d.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func newID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "-" + hex.EncodeToString(bytes[:])
}
