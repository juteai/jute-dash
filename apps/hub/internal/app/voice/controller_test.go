package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSatelliteEventIngressRequiresAuthorizedPairedSatellite(t *testing.T) {
	store := NewMemoryRepository()
	if _, err := store.SaveSatelliteInstall(context.Background(), SatelliteRecord{
		ID:                  "sat-kitchen",
		DisplayName:         "Kitchen Satellite",
		DeviceProfileID:     "kitchen-voice",
		Status:              SatelliteStatusPaired,
		CredentialSecretRef: "secret-ref-kitchen",
	}); err != nil {
		t.Fatalf("SaveSatelliteInstall() error = %v", err)
	}
	emitter := &recordingControllerEmitter{}
	controller := NewController(store, emitter, nil)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/satellites/sat-kitchen/events",
		strings.NewReader(`{"type":"voice.satellite_state_changed","state":"wake_listening"}`),
	)
	rr := httptest.NewRecorder()

	controller.handleSatelliteRoutes(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
	if len(emitter.satelliteEvents) != 0 {
		t.Fatalf("unauthorized request emitted events: %+v", emitter.satelliteEvents)
	}
}

func TestSatelliteEventIngressRejectsUnpairedSatellite(t *testing.T) {
	store := NewMemoryRepository()
	emitter := &recordingControllerEmitter{}
	controller := NewController(store, emitter, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/satellites/sat-unknown/events",
		strings.NewReader(`{"type":"voice.satellite_state_changed","state":"wake_listening"}`),
	)
	req.Header.Set("X-Jute-Satellite-Auth", "secret-ref-unknown")
	rr := httptest.NewRecorder()

	controller.handleSatelliteRoutes(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
	if len(emitter.satelliteEvents) != 0 {
		t.Fatalf("unpaired request emitted events: %+v", emitter.satelliteEvents)
	}
}

func TestSatelliteEventIngressRejectsRevokedSatellite(t *testing.T) {
	store := NewMemoryRepository()
	if _, err := store.SaveSatelliteInstall(context.Background(), SatelliteRecord{
		ID:                  "sat-bedroom",
		DisplayName:         "Bedroom Satellite",
		Status:              SatelliteStatusRevoked,
		CredentialSecretRef: "secret-ref-bedroom",
	}); err != nil {
		t.Fatalf("SaveSatelliteInstall() error = %v", err)
	}
	controller := NewController(store, &recordingControllerEmitter{}, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/satellites/sat-bedroom/events",
		strings.NewReader(`{"type":"voice.satellite_health_changed","health":"available"}`),
	)
	req.Header.Set("X-Jute-Satellite-Auth", "secret-ref-bedroom")
	rr := httptest.NewRecorder()

	controller.handleSatelliteRoutes(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
}

func TestSatelliteEventIngressRejectsUnsafePayloadWithoutEchoingIt(t *testing.T) {
	store := NewMemoryRepository()
	if _, err := store.SaveSatelliteInstall(context.Background(), SatelliteRecord{
		ID:                  "sat-kitchen",
		DisplayName:         "Kitchen Satellite",
		Status:              SatelliteStatusPaired,
		CredentialSecretRef: "secret-ref-kitchen",
	}); err != nil {
		t.Fatalf("SaveSatelliteInstall() error = %v", err)
	}
	emitter := &recordingControllerEmitter{}
	controller := NewController(store, emitter, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/satellites/sat-kitchen/events",
		strings.NewReader(
			`{"type":"voice.satellite_health_changed","health":"available","rawAudioPcm":"token=secret-audio"}`,
		),
	)
	req.Header.Set("X-Jute-Satellite-Auth", "secret-ref-kitchen")
	rr := httptest.NewRecorder()

	controller.handleSatelliteRoutes(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "secret-audio") || strings.Contains(rr.Body.String(), "rawAudioPcm") {
		t.Fatalf("unsafe payload was echoed in response: %s", rr.Body.String())
	}
	if len(emitter.satelliteEvents) != 0 {
		t.Fatalf("unsafe request emitted events: %+v", emitter.satelliteEvents)
	}
}

func TestSatelliteEventIngressRejectsTrailingUnsafePayloadWithoutEchoingIt(t *testing.T) {
	store := NewMemoryRepository()
	if _, err := store.SaveSatelliteInstall(context.Background(), SatelliteRecord{
		ID:                  "sat-kitchen",
		DisplayName:         "Kitchen Satellite",
		Status:              SatelliteStatusPaired,
		CredentialSecretRef: "secret-ref-kitchen",
	}); err != nil {
		t.Fatalf("SaveSatelliteInstall() error = %v", err)
	}
	emitter := &recordingControllerEmitter{}
	controller := NewController(store, emitter, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/satellites/sat-kitchen/events",
		strings.NewReader(
			`{"type":"voice.satellite_health_changed","health":"available"}{"rawAudioPcm":"token=secret-audio"}`,
		),
	)
	req.Header.Set("X-Jute-Satellite-Auth", "secret-ref-kitchen")
	rr := httptest.NewRecorder()

	controller.handleSatelliteRoutes(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "secret-audio") || strings.Contains(rr.Body.String(), "rawAudioPcm") {
		t.Fatalf("unsafe payload was echoed in response: %s", rr.Body.String())
	}
	if len(emitter.satelliteEvents) != 0 {
		t.Fatalf("unsafe trailing request emitted events: %+v", emitter.satelliteEvents)
	}
}

func TestSatelliteEventIngressRejectsEmptyTypedPayloads(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "state missing state",
			body: `{"type":"voice.satellite_state_changed"}`,
		},
		{
			name: "health missing health",
			body: `{"type":"voice.satellite_health_changed"}`,
		},
		{
			name: "wake missing model",
			body: `{"type":"voice.satellite_wake_detected"}`,
		},
		{
			name: "version missing version",
			body: `{"type":"voice.satellite_version_changed"}`,
		},
		{
			name: "update missing version and channel",
			body: `{"type":"voice.satellite_update_available"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryRepository()
			if _, err := store.SaveSatelliteInstall(context.Background(), SatelliteRecord{
				ID:                  "sat-kitchen",
				DisplayName:         "Kitchen Satellite",
				Status:              SatelliteStatusPaired,
				CredentialSecretRef: "secret-ref-kitchen",
			}); err != nil {
				t.Fatalf("SaveSatelliteInstall() error = %v", err)
			}
			emitter := &recordingControllerEmitter{}
			controller := NewController(store, emitter, nil)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/voice/satellites/sat-kitchen/events",
				strings.NewReader(tt.body),
			)
			req.Header.Set("X-Jute-Satellite-Auth", "secret-ref-kitchen")
			rr := httptest.NewRecorder()

			controller.handleSatelliteRoutes(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
			}
			if len(emitter.satelliteEvents) != 0 {
				t.Fatalf("empty typed payload emitted events: %+v", emitter.satelliteEvents)
			}
		})
	}
}

func TestSatelliteEventIngressEmitsSafePayload(t *testing.T) {
	store := NewMemoryRepository()
	if _, err := store.SaveSatelliteInstall(context.Background(), SatelliteRecord{
		ID:                  "sat-kitchen",
		DisplayName:         "Kitchen Satellite",
		RoomLabel:           "Kitchen",
		DeviceProfileID:     "kitchen-voice",
		Status:              SatelliteStatusPaired,
		CredentialSecretRef: "secret-ref-kitchen",
	}); err != nil {
		t.Fatalf("SaveSatelliteInstall() error = %v", err)
	}
	emitter := &recordingControllerEmitter{}
	controller := NewController(store, emitter, nil)
	body := map[string]any{
		"type":                     EventVoiceSatelliteHealthChanged,
		"state":                    "wake_listening",
		"health":                   "available",
		"version":                  "0.1.0",
		"updateChannel":            "stable",
		"wakeModelId":              "hey-jute",
		"providerIds":              []string{"org.example.openwakeword"},
		"safeErrorCode":            "provider.ok",
		"localProcessingLatencyMs": 42,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/satellites/sat-kitchen/events",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("X-Jute-Satellite-Auth", "secret-ref-kitchen")
	rr := httptest.NewRecorder()

	controller.handleSatelliteRoutes(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	if len(emitter.satelliteEvents) != 1 {
		t.Fatalf("expected one satellite event, got %+v", emitter.satelliteEvents)
	}
	event := emitter.satelliteEvents[0]
	if event.Type != EventVoiceSatelliteHealthChanged || event.DeviceID != "sat-kitchen" {
		t.Fatalf("unexpected event: %+v", event)
	}
	payload, ok := event.Payload.(SatelliteEventPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", event.Payload)
	}
	if payload.SatelliteID != "sat-kitchen" ||
		payload.DeviceProfileID != "kitchen-voice" ||
		payload.RoomLabel != "Kitchen" ||
		payload.State != "wake_listening" ||
		payload.Health != "available" ||
		payload.Version != "0.1.0" ||
		payload.SafeErrorCode != "provider.ok" ||
		payload.LocalProcessingLatencyMS != 42 {
		t.Fatalf("unexpected satellite payload: %+v", payload)
	}
	assertJSONOmits(t, event, "secret-ref-kitchen", "credential", "rawAudio", "token=")
}

func TestTTSSpeakUsesProviderAndReturnsSafePlaybackMetadata(t *testing.T) {
	store := NewMemoryRepositoryFromConfig(Config{
		TTSProviderID: "local-tts",
		TTSVoiceID:    "amy",
		TTSLocale:     "en-GB",
		TTSEnabled:    true,
	})
	provider := &fixtureControllerTTSProvider{
		result: TTSAudioResult{
			Audio:        []byte{1, 2, 3, 4},
			ProviderID:   "local-tts",
			VoiceID:      "amy",
			Locale:       "en-GB",
			ContentType:  "audio/pcm",
			SampleRate:   16000,
			SampleWidth:  2,
			Channels:     1,
			Duration:     125 * time.Microsecond,
			PlaybackKind: "audio",
		},
	}
	emitter := &recordingControllerEmitter{}
	controller := NewControllerWithTTSProvider(store, emitter, nil, provider)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		strings.NewReader(`{"text":"The kitchen lights are on.","cache":true}`),
	)
	rr := httptest.NewRecorder()

	controller.handleTTSSpeak(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body TTSActionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != TTSStateCompleted ||
		body.ProviderID != "local-tts" ||
		body.VoiceID != "amy" ||
		body.PlaybackKind != "audio" ||
		body.ContentType != "audio/pcm" ||
		body.SampleRate != 16000 ||
		body.SampleWidth != 2 ||
		body.Channels != 1 ||
		body.AudioBytes != 4 ||
		body.DurationMs != 0 {
		t.Fatalf("unexpected provider-backed TTS response: %+v", body)
	}
	if provider.request.ProviderID != "local-tts" ||
		provider.request.VoiceID != "amy" ||
		provider.request.Locale != "en-GB" {
		t.Fatalf("provider saw non-effective request: %+v", provider.request)
	}
	assertJSONOmits(t, body, "AQIDBA", string([]byte{1, 2, 3, 4}), "rawAudio")
	if len(emitter.ttsEvents) != 2 ||
		emitter.ttsEvents[0].Type != EventTTSStarted ||
		emitter.ttsEvents[1].Type != EventTTSCompleted {
		t.Fatalf("unexpected TTS events: %+v", emitter.ttsEvents)
	}
	completed, ok := emitter.ttsEvents[1].Payload.(TTSActionResponse)
	if !ok || completed.AudioBytes != 4 || completed.ContentType != "audio/pcm" {
		t.Fatalf("completed event omitted playback metadata: %+v", emitter.ttsEvents[1])
	}
	assertJSONOmits(t, emitter.ttsEvents, "AQIDBA", string([]byte{1, 2, 3, 4}), "rawAudio")
}

func TestTTSSpeakUsesWyomingProviderAndReturnsSafePlaybackMetadata(t *testing.T) {
	store := NewMemoryRepositoryFromConfig(Config{
		TTSProviderID: "local-tts",
		TTSVoiceID:    "amy",
		TTSLocale:     "en-GB",
		TTSEnabled:    true,
	})
	client, server := net.Pipe()
	defer client.Close()
	provider := WyomingTTSProvider{
		ProviderID: "local-tts",
		Endpoint:   "tcp://127.0.0.1:10200",
		VoiceID:    "amy",
		Locale:     "en-GB",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		line, err := readLine(server)
		if err != nil {
			errc <- err
			return
		}
		if !strings.Contains(line, `"type":"synthesize"`) ||
			!strings.Contains(line, `"text":"The kitchen lights are on."`) ||
			!strings.Contains(line, `"name":"amy"`) ||
			!strings.Contains(line, `"language":"en-GB"`) ||
			strings.Contains(line, "payload_length") {
			errc <- errors.New("unexpected synthesize request: " + line)
			return
		}
		if _, err := server.Write(
			[]byte(`{"type":"audio-start","data":{"rate":16000,"width":2,"channels":1}}` + "\n"),
		); err != nil {
			errc <- err
			return
		}
		if err := writeWyomingEvent(server, wyomingEvent{
			Type: WyomingEventAudioChunk,
			Data: map[string]any{
				"rate":     16000,
				"width":    2,
				"channels": 1,
			},
			Payload: []byte{1, 2, 3, 4},
		}); err != nil {
			errc <- err
			return
		}
		_, err = server.Write([]byte(`{"type":"audio-stop"}` + "\n"))
		errc <- err
	}()

	emitter := &recordingControllerEmitter{}
	controller := NewControllerWithTTSProvider(store, emitter, nil, provider)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		strings.NewReader(`{"text":"The kitchen lights are on.","cache":true}`),
	)
	rr := httptest.NewRecorder()

	controller.handleTTSSpeak(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
	var body TTSActionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != TTSStateCompleted ||
		body.ProviderID != "local-tts" ||
		body.VoiceID != "amy" ||
		body.PlaybackKind != "audio" ||
		body.ContentType != "audio/pcm" ||
		body.SampleRate != 16000 ||
		body.SampleWidth != 2 ||
		body.Channels != 1 ||
		body.AudioBytes != 4 ||
		body.DurationMs != 0 {
		t.Fatalf("unexpected provider-backed TTS response: %+v", body)
	}
	if len(emitter.ttsEvents) != 2 ||
		emitter.ttsEvents[0].Type != EventTTSStarted ||
		emitter.ttsEvents[1].Type != EventTTSCompleted {
		t.Fatalf("unexpected TTS events: %+v", emitter.ttsEvents)
	}
	completed, ok := emitter.ttsEvents[1].Payload.(TTSActionResponse)
	if !ok || completed.AudioBytes != 4 || completed.ContentType != "audio/pcm" {
		t.Fatalf("completed event omitted playback metadata: %+v", emitter.ttsEvents[1])
	}
	assertJSONOmits(t, body, "AQIDBA", string([]byte{1, 2, 3, 4}), "rawAudio", "127.0.0.1:10200")
	assertJSONOmits(t, emitter.ttsEvents, "AQIDBA", string([]byte{1, 2, 3, 4}), "rawAudio", "127.0.0.1:10200")
}

func TestTTSSpeakProviderFailureReturnsSafeFailedState(t *testing.T) {
	store := NewMemoryRepositoryFromConfig(Config{
		TTSProviderID: "local-tts",
		TTSVoiceID:    "amy",
		TTSEnabled:    true,
	})
	provider := &fixtureControllerTTSProvider{
		err: errors.New("dial tcp 127.0.0.1:10200: token=secret unavailable"),
	}
	emitter := &recordingControllerEmitter{}
	controller := NewControllerWithTTSProvider(store, emitter, nil, provider)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		strings.NewReader(`{"text":"hello"}`),
	)
	rr := httptest.NewRecorder()

	controller.handleTTSSpeak(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body TTSActionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != TTSStateFailed || body.Reason != "provider_unavailable" {
		t.Fatalf("unexpected failed TTS response: %+v", body)
	}
	assertJSONOmits(t, body, "127.0.0.1:10200", "token=secret", "dial tcp")
	if len(emitter.ttsEvents) != 2 ||
		emitter.ttsEvents[0].Type != EventTTSStarted ||
		emitter.ttsEvents[1].Type != EventTTSFailed {
		t.Fatalf("unexpected TTS events: %+v", emitter.ttsEvents)
	}
	assertJSONOmits(t, emitter.ttsEvents, "127.0.0.1:10200", "token=secret", "dial tcp")
}

func TestTTSSpeakSensitiveOutputStopsAsVisualOnlyWithoutProviderSynthesis(t *testing.T) {
	store := NewMemoryRepositoryFromConfig(Config{
		TTSProviderID:           "local-tts",
		TTSVoiceID:              "amy",
		TTSEnabled:              true,
		SensitiveOutputPolicy:   TTSPolicyVisualOnlySensitive,
		CommandProvidersEnabled: false,
	})
	provider := &fixtureControllerTTSProvider{
		result: TTSAudioResult{Audio: []byte{1, 2, 3, 4}},
	}
	emitter := &recordingControllerEmitter{}
	controller := NewControllerWithTTSProvider(store, emitter, nil, provider)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		strings.NewReader(`{"text":"the door code is 1234","cache":true,"conversationId":"conversation-1"}`),
	)
	rr := httptest.NewRecorder()

	controller.handleTTSSpeak(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body TTSActionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != TTSStateVisualOnly ||
		!body.VisualOnly ||
		body.Reason != "sensitive_output_visual_only" ||
		body.CacheEligible ||
		body.CacheKey != "" {
		t.Fatalf("unexpected sensitive TTS response: %+v", body)
	}
	if provider.request.Text != "" {
		t.Fatalf("provider should not synthesize sensitive output, saw %+v", provider.request)
	}
	if len(emitter.ttsEvents) != 1 ||
		emitter.ttsEvents[0].Type != EventTTSStopped {
		t.Fatalf("unexpected TTS events: %+v", emitter.ttsEvents)
	}
	stopped, ok := emitter.ttsEvents[0].Payload.(TTSActionResponse)
	if !ok ||
		stopped.State != TTSStateVisualOnly ||
		!stopped.VisualOnly ||
		stopped.Reason != "sensitive_output_visual_only" ||
		stopped.ConversationID != "conversation-1" {
		t.Fatalf("unexpected stopped event payload: %+v", emitter.ttsEvents[0])
	}
	assertJSONOmits(t, body, "door code", "1234", "AQIDBA", "rawAudio")
	assertJSONOmits(t, emitter.ttsEvents, "door code", "1234", "AQIDBA", "rawAudio")
}

func TestTTSSpeakWyomingProviderOfflineReturnsSafeFailedState(t *testing.T) {
	store := NewMemoryRepositoryFromConfig(Config{
		TTSProviderID: "local-tts",
		TTSVoiceID:    "amy",
		TTSEnabled:    true,
	})
	provider := WyomingTTSProvider{
		ProviderID: "local-tts",
		Endpoint:   "tcp://127.0.0.1:10200",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("dial tcp 127.0.0.1:10200: token=secret unavailable")
		},
	}
	emitter := &recordingControllerEmitter{}
	controller := NewControllerWithTTSProvider(store, emitter, nil, provider)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		strings.NewReader(`{"text":"hello"}`),
	)
	rr := httptest.NewRecorder()

	controller.handleTTSSpeak(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body TTSActionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != TTSStateFailed || body.Reason != "provider_unavailable" {
		t.Fatalf("unexpected failed TTS response: %+v", body)
	}
	if len(emitter.ttsEvents) != 2 ||
		emitter.ttsEvents[0].Type != EventTTSStarted ||
		emitter.ttsEvents[1].Type != EventTTSFailed {
		t.Fatalf("unexpected TTS events: %+v", emitter.ttsEvents)
	}
	assertJSONOmits(t, body, "127.0.0.1:10200", "token=secret", "dial tcp")
	assertJSONOmits(t, emitter.ttsEvents, "127.0.0.1:10200", "token=secret", "dial tcp")
}

func TestTTSStopCancelsInFlightProviderSynthesis(t *testing.T) {
	store := NewMemoryRepositoryFromConfig(Config{
		TTSProviderID: "local-tts",
		TTSVoiceID:    "amy",
		TTSEnabled:    true,
	})
	provider := &blockingControllerTTSProvider{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
	}
	emitter := &recordingControllerEmitter{}
	controller := NewControllerWithTTSProvider(store, emitter, nil, provider)
	speakReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		strings.NewReader(`{"text":"hello kitchen","conversationId":"conversation-1","turnId":"turn-1"}`),
	)
	speakRR := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		controller.handleTTSSpeak(speakRR, speakReq)
	}()

	select {
	case <-provider.started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider synthesis did not start")
	}
	stopReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/stop",
		strings.NewReader(`{"reason":"barge_in"}`),
	)
	stopRR := httptest.NewRecorder()

	controller.handleTTSStop(stopRR, stopReq)

	if stopRR.Code != http.StatusOK {
		t.Fatalf("stop status = %d body=%s", stopRR.Code, stopRR.Body.String())
	}
	select {
	case <-provider.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("provider synthesis context was not cancelled")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("speak request did not return after stop")
	}
	var speakBody TTSActionResponse
	if err := json.Unmarshal(speakRR.Body.Bytes(), &speakBody); err != nil {
		t.Fatalf("decode speak response: %v body=%s", err, speakRR.Body.String())
	}
	if speakBody.State != TTSStateStopped || speakBody.Reason != "barge_in" {
		t.Fatalf("expected stopped speak response, got %+v", speakBody)
	}
	if len(emitter.ttsEvents) != 2 ||
		emitter.ttsEvents[0].Type != EventTTSStarted ||
		emitter.ttsEvents[1].Type != EventTTSStopped {
		t.Fatalf("unexpected TTS events: %+v", emitter.ttsEvents)
	}
}

func TestTTSStopPreservesStoppedStateWhenProviderReturnsAfterCancel(t *testing.T) {
	store := NewMemoryRepositoryFromConfig(Config{
		TTSProviderID: "local-tts",
		TTSVoiceID:    "amy",
		TTSEnabled:    true,
	})
	provider := &blockingControllerTTSProvider{
		started:           make(chan struct{}),
		cancelled:         make(chan struct{}),
		returnAudioOnStop: true,
	}
	emitter := &recordingControllerEmitter{}
	controller := NewControllerWithTTSProvider(store, emitter, nil, provider)
	speakReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		strings.NewReader(`{"text":"hello kitchen"}`),
	)
	speakRR := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		controller.handleTTSSpeak(speakRR, speakReq)
	}()

	select {
	case <-provider.started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider synthesis did not start")
	}
	stopReq := httptest.NewRequest(http.MethodPost, "/api/v1/tts/stop", strings.NewReader(`{"reason":"cancel"}`))
	stopRR := httptest.NewRecorder()
	controller.handleTTSStop(stopRR, stopReq)

	select {
	case <-provider.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("provider synthesis context was not cancelled")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("speak request did not return after stop")
	}
	var speakBody TTSActionResponse
	if err := json.Unmarshal(speakRR.Body.Bytes(), &speakBody); err != nil {
		t.Fatalf("decode speak response: %v body=%s", err, speakRR.Body.String())
	}
	if speakBody.State != TTSStateStopped || speakBody.Reason != "cancel" || speakBody.AudioBytes != 0 {
		t.Fatalf("expected stopped response without late audio metadata, got %+v", speakBody)
	}
	if len(emitter.ttsEvents) != 2 ||
		emitter.ttsEvents[0].Type != EventTTSStarted ||
		emitter.ttsEvents[1].Type != EventTTSStopped {
		t.Fatalf("unexpected TTS events: %+v", emitter.ttsEvents)
	}
}

type recordingControllerEmitter struct {
	voiceEvents     []VoiceEvent
	conversations   []VoiceEvent
	ttsEvents       []VoiceEvent
	satelliteEvents []VoiceEvent
}

func (e *recordingControllerEmitter) EmitVoiceStateChanged(
	deviceProfileID string,
	payload VoiceStatePayload,
) VoiceEvent {
	event := VoiceEvent{Type: EventVoiceStateChanged, DeviceID: deviceProfileID, Payload: payload}
	e.voiceEvents = append(e.voiceEvents, event)
	return event
}

func (e *recordingControllerEmitter) EmitConversationEvent(
	eventType, deviceID, conversationID string,
	payload any,
) VoiceEvent {
	event := VoiceEvent{Type: eventType, DeviceID: deviceID, ConversationID: conversationID, Payload: payload}
	e.conversations = append(e.conversations, event)
	return event
}

func (e *recordingControllerEmitter) EmitTTSEvent(eventType, deviceID string, response TTSActionResponse) VoiceEvent {
	event := VoiceEvent{Type: eventType, DeviceID: deviceID, Payload: response}
	e.ttsEvents = append(e.ttsEvents, event)
	return event
}

func (e *recordingControllerEmitter) EmitSatelliteEvent(
	eventType string,
	satellite SatelliteProjection,
	payload SatelliteEventPayload,
) VoiceEvent {
	payload.SatelliteID = satellite.ID
	payload.DeviceProfileID = satellite.DeviceProfileID
	payload.RoomLabel = satellite.RoomLabel
	event := VoiceEvent{Type: eventType, DeviceID: satellite.ID, Payload: payload}
	e.satelliteEvents = append(e.satelliteEvents, event)
	return event
}

type fixtureControllerTTSProvider struct {
	request TTSRequest
	result  TTSAudioResult
	err     error
}

func (p *fixtureControllerTTSProvider) Synthesize(_ context.Context, req TTSRequest) (TTSAudioResult, error) {
	p.request = req
	if p.err != nil {
		return TTSAudioResult{}, p.err
	}
	return p.result, nil
}

type blockingControllerTTSProvider struct {
	started           chan struct{}
	cancelled         chan struct{}
	returnAudioOnStop bool
}

func (p *blockingControllerTTSProvider) Synthesize(ctx context.Context, _ TTSRequest) (TTSAudioResult, error) {
	close(p.started)
	<-ctx.Done()
	close(p.cancelled)
	if p.returnAudioOnStop {
		return TTSAudioResult{
			Audio:        []byte{1, 2, 3, 4},
			ProviderID:   "local-tts",
			VoiceID:      "amy",
			ContentType:  "audio/pcm",
			PlaybackKind: "audio",
		}, nil
	}
	return TTSAudioResult{}, ctx.Err()
}
