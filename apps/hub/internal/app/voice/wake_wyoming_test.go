package voice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

type recordingWakeEmitter struct {
	events []VoiceEvent
}

func (e *recordingWakeEmitter) EmitVoiceStateChanged(deviceID string, payload VoiceStatePayload) VoiceEvent {
	event := VoiceEvent{
		Type:     EventVoiceStateChanged,
		DeviceID: deviceID,
		Payload:  payload,
	}
	e.events = append(e.events, event)
	return event
}

func (e *recordingWakeEmitter) EmitVoiceWakeDetected(deviceID, conversationID string) VoiceEvent {
	event := VoiceEvent{
		Type:           EventVoiceWakeDetected,
		DeviceID:       deviceID,
		ConversationID: conversationID,
		Payload:        map[string]any{},
	}
	e.events = append(e.events, event)
	return event
}

func TestWyomingWakeProviderEmitsWakeAndCaptureStates(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	provider := WyomingWakeProvider{
		Endpoint: "tcp://127.0.0.1:10400",
		DeviceID: "kitchen-display",
		ModelNames: []string{
			"hey_jute",
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
		ConversationID: func() string {
			return "conversation-1"
		},
	}
	emitter := &recordingWakeEmitter{}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		detectLine, err := readLine(server)
		if err != nil {
			errc <- err
			return
		}
		if !strings.Contains(detectLine, `"type":"detect"`) ||
			!strings.Contains(detectLine, `"names":["hey_jute"]`) {
			errc <- io.ErrUnexpectedEOF
			return
		}
		_, err = server.Write([]byte(`{"type":"detection","data":{"name":"hey_jute","timestamp":120}}` + "\n"))
		errc <- err
	}()

	if err := provider.RunOnce(context.Background(), emitter); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}

	gotTypes := make([]string, 0, len(emitter.events))
	gotStates := make([]string, 0, len(emitter.events))
	for _, event := range emitter.events {
		gotTypes = append(gotTypes, event.Type)
		if payload, ok := event.Payload.(VoiceStatePayload); ok {
			gotStates = append(gotStates, payload.State)
		}
	}
	if want := []string{
		EventVoiceWakeDetected,
		EventVoiceStateChanged,
		EventVoiceStateChanged,
	}; !reflect.DeepEqual(gotTypes, want) {
		t.Fatalf("unexpected event types: got %v want %v", gotTypes, want)
	}
	if want := []string{WakeStateDetected, WakeStateCapturingUtterance}; !reflect.DeepEqual(gotStates, want) {
		t.Fatalf("unexpected wake states: got %v want %v", gotStates, want)
	}
	if emitter.events[0].ConversationID != "conversation-1" {
		t.Fatalf("unexpected conversation id: %s", emitter.events[0].ConversationID)
	}
}

func TestWyomingWakeProviderKeepsNotDetectedSilentByDefault(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	provider := WyomingWakeProvider{
		Endpoint: "tcp://192.168.1.12:10400",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	emitter := &recordingWakeEmitter{}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		if _, err := readLine(server); err != nil {
			errc <- err
			return
		}
		_, err := server.Write([]byte(`{"type":"not-detected"}` + "\n"))
		errc <- err
	}()

	if err := provider.RunOnce(context.Background(), emitter); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
	if len(emitter.events) != 0 {
		t.Fatalf("expected silent not-detected event, got %+v", emitter.events)
	}
}

func TestWyomingWakeProviderKeepsUnexpectedModelDetectionSilentByDefault(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	provider := WyomingWakeProvider{
		Endpoint:   "tcp://127.0.0.1:10400",
		ModelNames: []string{"hey_jute"},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	emitter := &recordingWakeEmitter{}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		if _, err := readLine(server); err != nil {
			errc <- err
			return
		}
		_, err := server.Write([]byte(`{"type":"detection","data":{"name":"okay_nabu"}}` + "\n"))
		errc <- err
	}()

	if err := provider.RunOnce(context.Background(), emitter); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
	if len(emitter.events) != 0 {
		t.Fatalf("expected silent unexpected model detection, got %+v", emitter.events)
	}
}

func TestWyomingWakeProviderDebugReportsUnexpectedModelDetectionAsReady(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	provider := WyomingWakeProvider{
		Endpoint:   "tcp://127.0.0.1:10400",
		ModelNames: []string{"hey_jute"},
		Debug:      true,
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	emitter := &recordingWakeEmitter{}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		if _, err := readLine(server); err != nil {
			errc <- err
			return
		}
		_, err := server.Write([]byte(`{"type":"detection","data":{"name":"okay_nabu"}}` + "\n"))
		errc <- err
	}()

	if err := provider.RunOnce(context.Background(), emitter); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
	if len(emitter.events) != 1 || emitter.events[0].Type != EventVoiceStateChanged {
		t.Fatalf("expected one debug state event, got %+v", emitter.events)
	}
	payload, ok := emitter.events[0].Payload.(VoiceStatePayload)
	if !ok || payload.State != State(true, false) || payload.ServiceStatus != "ready" {
		t.Fatalf("unexpected debug payload: %+v", emitter.events[0].Payload)
	}
}

func TestWyomingWakeProviderRejectsUnsafeEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"tcp://example.com:10400",
		"tcp://token:secret@127.0.0.1:10400",
		"http://127.0.0.1:10400",
	} {
		t.Run(endpoint, func(t *testing.T) {
			provider := WyomingWakeProvider{Endpoint: endpoint}
			if err := provider.RunOnce(context.Background(), nil); err == nil {
				t.Fatalf("expected endpoint rejection")
			}
		})
	}
}

func TestReadWyomingEventMergesDataLengthPayload(t *testing.T) {
	data := []byte(`{"name":"hey_jute","timestamp":120}`)
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]any{
		"type":        "detection",
		"data_length": len(data),
	}); err != nil {
		t.Fatalf("encode fixture header: %v", err)
	}
	buf.Write(data)
	event, err := readWyomingEvent(bufioReader(&buf))
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	if event.Type != WyomingEventDetection ||
		event.Data["name"] != "hey_jute" ||
		event.Data["timestamp"].(float64) != 120 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestReadWyomingEventRejectsUnsafeLengthsBeforeAllocation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		event   string
		wantErr string
	}{
		{
			name:    "negative data length",
			event:   `{"type":"detection","data_length":-1}` + "\n",
			wantErr: "data length is invalid",
		},
		{
			name:    "oversized data length",
			event:   `{"type":"detection","data_length":1048577}` + "\n",
			wantErr: "data length exceeds limit",
		},
		{
			name:    "negative payload length",
			event:   `{"type":"audio-chunk","payload_length":-1}` + "\n",
			wantErr: "payload length is invalid",
		},
		{
			name:    "oversized payload length",
			event:   `{"type":"audio-chunk","payload_length":16777217}` + "\n",
			wantErr: "payload length exceeds limit",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readWyomingEvent(bufioReader(strings.NewReader(tc.event)))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q error, got %v", tc.wantErr, err)
			}
		})
	}
}

func readLine(conn net.Conn) (string, error) {
	var out strings.Builder
	buf := make([]byte, 1)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 1 {
			out.WriteByte(buf[0])
			if buf[0] == '\n' {
				return out.String(), nil
			}
		}
	}
}

func bufioReader(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}
