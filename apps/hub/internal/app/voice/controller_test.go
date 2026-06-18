package voice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
		strings.NewReader(`{"text":"The kitchen lights are on."}`),
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

func TestTTSSpeakResolvesActiveProviderAtRequestTime(t *testing.T) {
	active := &fixtureControllerTTSProvider{
		result: TTSAudioResult{
			Audio:        []byte{1, 2},
			ProviderID:   "current-tts",
			VoiceID:      "current-voice",
			ContentType:  "audio/pcm",
			PlaybackKind: "audio",
		},
	}
	stale := &fixtureControllerTTSProvider{
		result: TTSAudioResult{
			Audio:        []byte{9, 9},
			ProviderID:   "stale-tts",
			VoiceID:      "stale-voice",
			ContentType:  "audio/pcm",
			PlaybackKind: "audio",
		},
	}
	store := dynamicControllerTTSStore{
		MemoryRepository: NewMemoryRepositoryFromConfig(Config{
			TTSProviderID: "current-tts",
			TTSVoiceID:    "current-voice",
			TTSEnabled:    true,
		}),
		provider: active,
	}
	controller := NewControllerWithTTSProvider(store, nil, nil, stale)
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
	if body.ProviderID != "current-tts" || body.VoiceID != "current-voice" || body.AudioBytes != 2 {
		t.Fatalf("expected active provider response, got %+v", body)
	}
	if active.request.Text != "hello" {
		t.Fatalf("active provider was not called: %+v", active.request)
	}
	if stale.request.Text != "" {
		t.Fatalf("stale injected provider was called: %+v", stale.request)
	}
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
		strings.NewReader(`{"text":"the door code is 1234","conversationId":"conversation-1"}`),
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
		body.Reason != "sensitive_output_visual_only" {
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
	voiceEvents   []VoiceEvent
	conversations []VoiceEvent
	ttsEvents     []VoiceEvent
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

type fixtureControllerTTSProvider struct {
	request TTSRequest
	result  TTSAudioResult
	err     error
}

type dynamicControllerTTSStore struct {
	*MemoryRepository

	provider TTSProvider
	err      error
}

func (s dynamicControllerTTSStore) ActiveTTSProvider(context.Context, string) (TTSProvider, error) {
	return s.provider, s.err
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
