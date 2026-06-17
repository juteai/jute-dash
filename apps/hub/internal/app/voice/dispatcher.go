package voice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
	"time"

	"jute-dash/apps/hub/internal/pkg/displayactions"
)

const (
	EventVoiceStateChanged             = "voice.state_changed"
	EventVoiceWakeDetected             = "voice.wake_detected"
	EventVoiceTranscriptPartial        = "voice.transcript.partial"
	EventVoiceTranscriptFinal          = "voice.transcript.final"
	EventConversationStarted           = "conversation.started"
	EventConversationTurnStarted       = "conversation.turn_started"
	EventConversationTurnCompleted     = "conversation.turn_completed"
	EventConversationFollowupStarted   = "conversation.followup_started"
	EventConversationEnded             = "conversation.ended"
	EventTTSStarted                    = "tts.started"
	EventTTSCompleted                  = "tts.completed"
	EventTTSFailed                     = "tts.failed"
	EventTTSStopped                    = "tts.stopped"
	EventVoiceSatelliteStateChanged    = "voice.satellite_state_changed"
	EventVoiceSatelliteHealthChanged   = "voice.satellite_health_changed"
	EventVoiceSatelliteWakeDetected    = "voice.satellite_wake_detected"
	EventVoiceSatelliteVersionChanged  = "voice.satellite_version_changed"
	EventVoiceSatelliteUpdateAvailable = "voice.satellite_update_available"
)

var secretPattern = regexp.MustCompile(`(?i)\b(bearer|token|secret|password|api[_-]?key)\s*[:=]\s*[^,\s]+`)

type VoiceStatePayload struct {
	Enabled       bool   `json:"enabled"`
	Muted         bool   `json:"muted"`
	State         string `json:"state"`
	ServiceStatus string `json:"serviceStatus"`
}

type VoiceTranscriptPayload struct {
	Text string `json:"text"`
}

type SatelliteEventPayload struct {
	SatelliteID              string   `json:"satelliteId"`
	DeviceProfileID          string   `json:"deviceProfileId"`
	RoomLabel                string   `json:"roomLabel,omitempty"`
	State                    string   `json:"state,omitempty"`
	Health                   string   `json:"health,omitempty"`
	Version                  string   `json:"version,omitempty"`
	UpdateChannel            string   `json:"updateChannel,omitempty"`
	WakeModelID              string   `json:"wakeModelId,omitempty"`
	ProviderIDs              []string `json:"providerIds,omitempty"`
	SafeErrorCode            string   `json:"safeErrorCode,omitempty"`
	LocalProcessingLatencyMS int      `json:"localProcessingLatencyMs,omitempty"`
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
		DeviceID:  safeIdentifier(deviceID),
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
		DeviceID:       safeIdentifier(deviceID),
		ConversationID: safeIdentifier(conversationID),
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
		DeviceID:       safeIdentifier(deviceID),
		ConversationID: safeIdentifier(conversationID),
		Payload: VoiceTranscriptPayload{
			Text: sanitizeText(text),
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
		DeviceID:       safeIdentifier(deviceID),
		ConversationID: safeIdentifier(conversationID),
		Payload:        sanitizePayload(payload),
	}
	d.publish(displayactions.Event{Type: eventType, Data: event})
	return event
}

func (d *Dispatcher) EmitTTSEvent(eventType, deviceID string, response TTSActionResponse) VoiceEvent {
	event := VoiceEvent{
		ID:             newID("tts-event"),
		Type:           eventType,
		CreatedAt:      d.now().UTC().Format(time.RFC3339Nano),
		DeviceID:       safeIdentifier(deviceID),
		ConversationID: safeIdentifier(response.ConversationID),
		Payload: TTSEventPayload{
			Action:        response.Action,
			State:         response.State,
			ProviderID:    safeIdentifier(response.ProviderID),
			VoiceID:       safeIdentifier(response.VoiceID),
			CacheEligible: response.CacheEligible,
			CacheKey:      safeIdentifier(response.CacheKey),
			Reason:        sanitizeText(response.Reason),
			PlaybackKind:  safeIdentifier(response.PlaybackKind),
			ContentType:   safeIdentifier(response.ContentType),
			SampleRate:    response.SampleRate,
			SampleWidth:   response.SampleWidth,
			Channels:      response.Channels,
			AudioBytes:    response.AudioBytes,
			DurationMs:    response.DurationMs,
		},
	}
	d.publish(displayactions.Event{Type: eventType, Data: event})
	return event
}

func (d *Dispatcher) EmitSatelliteEvent(
	eventType string,
	satellite SatelliteProjection,
	payload SatelliteEventPayload,
) VoiceEvent {
	payload.SatelliteID = safeIdentifier(satellite.ID)
	payload.DeviceProfileID = safeIdentifier(satellite.DeviceProfileID)
	payload.RoomLabel = sanitizeText(satellite.RoomLabel)
	payload.State = safeIdentifier(payload.State)
	payload.Health = safeIdentifier(payload.Health)
	payload.Version = safeIdentifier(payload.Version)
	payload.UpdateChannel = safeIdentifier(payload.UpdateChannel)
	payload.WakeModelID = safeIdentifier(payload.WakeModelID)
	payload.SafeErrorCode = safeIdentifier(payload.SafeErrorCode)
	payload.ProviderIDs = safeIdentifiers(payload.ProviderIDs)
	event := VoiceEvent{
		ID:        newID("voice-satellite"),
		Type:      eventType,
		CreatedAt: d.now().UTC().Format(time.RFC3339Nano),
		DeviceID:  safeIdentifier(satellite.ID),
		Payload:   payload,
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

func safeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	value = secretPattern.ReplaceAllString(value, "$1=[redacted]")
	return value
}

func safeIdentifiers(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	safe := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := safeIdentifier(value); trimmed != "" {
			safe = append(safe, trimmed)
		}
	}
	return safe
}

func sanitizeText(value string) string {
	value = strings.TrimSpace(value)
	return secretPattern.ReplaceAllString(value, "$1=[redacted]")
}

func sanitizePayload(payload any) any {
	switch value := payload.(type) {
	case nil:
		return map[string]any{}
	case string:
		return sanitizeText(value)
	case map[string]any:
		sanitized := make(map[string]any, len(value))
		for key, item := range value {
			if unsafePayloadKey(key) {
				sanitized[key] = "[redacted]"
				continue
			}
			sanitized[key] = sanitizePayload(item)
		}
		return sanitized
	case map[string]string:
		sanitized := make(map[string]string, len(value))
		for key, item := range value {
			if unsafePayloadKey(key) {
				sanitized[key] = "[redacted]"
				continue
			}
			sanitized[key] = sanitizeText(item)
		}
		return sanitized
	case []any:
		sanitized := make([]any, 0, len(value))
		for _, item := range value {
			sanitized = append(sanitized, sanitizePayload(item))
		}
		return sanitized
	case []string:
		sanitized := make([]string, 0, len(value))
		for _, item := range value {
			sanitized = append(sanitized, sanitizeText(item))
		}
		return sanitized
	default:
		return value
	}
}

func unsafePayloadKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password") ||
		strings.Contains(key, "credential") ||
		strings.Contains(key, "authorization") ||
		strings.Contains(key, "apikey") ||
		strings.Contains(key, "api_key")
}
