package voice

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	WakeStateDetected           = "wake_detected"
	WakeStateCapturingUtterance = "capturing_utterance"
	WyomingEventDetect          = "detect"
	WyomingEventDetection       = "detection"
	WyomingEventNotDetected     = "not-detected"
	maxWyomingEventDataLength   = 1 << 20
	maxWyomingEventPayloadSize  = 16 << 20
)

type WakeEventEmitter interface {
	EmitVoiceStateChanged(deviceID string, payload VoiceStatePayload) VoiceEvent
	EmitVoiceWakeDetected(deviceID, conversationID string) VoiceEvent
}

type WakeProvider interface {
	RunOnce(ctx context.Context, emitter WakeEventEmitter) error
}

type WyomingWakeProvider struct {
	ProviderID     string
	Endpoint       string
	DeviceID       string
	ModelNames     []string
	Debug          bool
	DialContext    func(ctx context.Context, network, address string) (net.Conn, error)
	ConversationID func() string
}

type wyomingEvent struct {
	Type          string         `json:"type"`
	Data          map[string]any `json:"data,omitempty"`
	DataLength    int            `json:"data_length,omitempty"`
	PayloadLength int            `json:"payload_length,omitempty"`
	Payload       []byte         `json:"-"`
}

func (p WyomingWakeProvider) RunOnce(ctx context.Context, emitter WakeEventEmitter) error {
	if strings.TrimSpace(p.Endpoint) == "" {
		return errors.New("wyoming wake endpoint is required")
	}
	network, address, err := wyomingTCPAddress(p.Endpoint)
	if err != nil {
		return err
	}
	dial := p.DialContext
	if dial == nil {
		var d net.Dialer
		dial = d.DialContext
	}
	conn, err := dial(ctx, network, address)
	if err != nil {
		return fmt.Errorf("connect wyoming wake provider: %w", err)
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	if err := writeWyomingEvent(conn, wyomingEvent{
		Type: WyomingEventDetect,
		Data: map[string]any{"names": sanitizeModelNames(p.ModelNames)},
	}); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	for {
		event, err := readWyomingEvent(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return err
		}
		switch event.Type {
		case WyomingEventDetection:
			if !p.acceptsDetection(event) {
				if p.Debug && emitter != nil {
					emitter.EmitVoiceStateChanged(p.deviceID(), VoiceStatePayload{
						Enabled:       true,
						Muted:         false,
						State:         State(true, false),
						ServiceStatus: "ready",
					})
				}
				return nil
			}
			p.emitDetection(emitter)
			return nil
		case WyomingEventNotDetected:
			if p.Debug && emitter != nil {
				emitter.EmitVoiceStateChanged(p.deviceID(), VoiceStatePayload{
					Enabled:       true,
					Muted:         false,
					State:         State(true, false),
					ServiceStatus: "ready",
				})
			}
			return nil
		}
	}
}

func (p WyomingWakeProvider) acceptsDetection(event wyomingEvent) bool {
	names := sanitizeModelNames(p.ModelNames)
	if len(names) == 0 {
		return true
	}
	name, _ := event.Data["name"].(string)
	name = safeIdentifier(name)
	for _, allowed := range names {
		if name == allowed {
			return true
		}
	}
	return false
}

func (p WyomingWakeProvider) emitDetection(emitter WakeEventEmitter) {
	if emitter == nil {
		return
	}
	deviceID := p.deviceID()
	conversationID := ""
	if p.ConversationID != nil {
		conversationID = p.ConversationID()
	}
	if conversationID == "" {
		conversationID = newID("voice-conversation")
	}
	emitter.EmitVoiceWakeDetected(deviceID, conversationID)
	emitter.EmitVoiceStateChanged(deviceID, VoiceStatePayload{
		Enabled:       true,
		Muted:         false,
		State:         WakeStateDetected,
		ServiceStatus: "ready",
	})
	emitter.EmitVoiceStateChanged(deviceID, VoiceStatePayload{
		Enabled:       true,
		Muted:         false,
		State:         WakeStateCapturingUtterance,
		ServiceStatus: "ready",
	})
}

func (p WyomingWakeProvider) deviceID() string {
	if strings.TrimSpace(p.DeviceID) == "" {
		return "default-display"
	}
	return p.DeviceID
}

func wyomingTCPAddress(endpoint string) (string, string, error) {
	if !safeLocalEndpoint(endpoint, true) {
		return "", "", errors.New("wyoming wake endpoint must be loopback or LAN-scoped")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("parse wyoming wake endpoint: %w", err)
	}
	if u.Scheme != "tcp" {
		return "", "", errors.New("wyoming wake endpoint must use tcp")
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		return "", "", errors.New("wyoming wake endpoint port is required")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", errors.New("wyoming wake endpoint port is invalid")
	}
	return "tcp", net.JoinHostPort(host, port), nil
}

func writeWyomingEvent(w io.Writer, event wyomingEvent) error {
	if len(event.Payload) > 0 {
		event.PayloadLength = len(event.Payload)
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode wyoming event: %w", err)
	}
	if _, err := w.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("write wyoming event: %w", err)
	}
	if len(event.Payload) > 0 {
		if _, err := w.Write(event.Payload); err != nil {
			return fmt.Errorf("write wyoming event payload: %w", err)
		}
	}
	return nil
}

func readWyomingEvent(reader *bufio.Reader) (wyomingEvent, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return wyomingEvent{}, err
	}
	var event wyomingEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return wyomingEvent{}, fmt.Errorf("decode wyoming event: %w", err)
	}
	if event.DataLength < 0 {
		return wyomingEvent{}, errors.New("wyoming event data length is invalid")
	}
	if event.DataLength > maxWyomingEventDataLength {
		return wyomingEvent{}, errors.New("wyoming event data length exceeds limit")
	}
	if event.PayloadLength < 0 {
		return wyomingEvent{}, errors.New("wyoming event payload length is invalid")
	}
	if event.PayloadLength > maxWyomingEventPayloadSize {
		return wyomingEvent{}, errors.New("wyoming event payload length exceeds limit")
	}
	if event.DataLength > 0 {
		dataBytes := make([]byte, event.DataLength)
		if _, err := io.ReadFull(reader, dataBytes); err != nil {
			return wyomingEvent{}, fmt.Errorf("read wyoming event data: %w", err)
		}
		var data map[string]any
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			return wyomingEvent{}, fmt.Errorf("decode wyoming event data: %w", err)
		}
		if event.Data == nil {
			event.Data = map[string]any{}
		}
		for key, value := range data {
			event.Data[key] = value
		}
	}
	if event.PayloadLength > 0 {
		event.Payload = make([]byte, event.PayloadLength)
		if _, err := io.ReadFull(reader, event.Payload); err != nil {
			return wyomingEvent{}, fmt.Errorf("read wyoming event payload: %w", err)
		}
	}
	return event, nil
}

func sanitizeModelNames(names []string) []string {
	clean := make([]string, 0, len(names))
	for _, name := range names {
		name = safeIdentifier(name)
		if name != "" {
			clean = append(clean, name)
		}
	}
	return clean
}

func DefaultWyomingWakeProvider(endpoint, providerID, deviceID, modelID string) WyomingWakeProvider {
	return WyomingWakeProvider{
		ProviderID: providerID,
		Endpoint:   endpoint,
		DeviceID:   deviceID,
		ModelNames: []string{modelID},
		DialContext: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).DialContext,
	}
}
