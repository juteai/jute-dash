package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fixtureCapture struct {
	frames []AudioFrame
	err    error
}

func (f fixtureCapture) Capture(ctx context.Context) (<-chan AudioFrame, <-chan error) {
	frames := make(chan AudioFrame)
	errs := make(chan error, 1)
	go func() {
		defer close(frames)
		defer close(errs)
		for _, frame := range f.frames {
			select {
			case <-ctx.Done():
				return
			case frames <- frame:
			}
		}
		if f.err != nil {
			errs <- f.err
		}
	}()
	return frames, errs
}

type thresholdVAD struct {
	threshold byte
}

type recordingWakeEmitter struct {
	events []VoiceEvent
}

func (e *recordingWakeEmitter) EmitVoiceStateChanged(deviceID string, payload VoiceStatePayload) VoiceEvent {
	event := VoiceEvent{Type: EventVoiceStateChanged, DeviceID: deviceID, Payload: payload}
	e.events = append(e.events, event)
	return event
}

func (e *recordingWakeEmitter) EmitVoiceWakeDetected(deviceID, conversationID string) VoiceEvent {
	event := VoiceEvent{
		Type:           EventVoiceWakeDetected,
		DeviceID:       deviceID,
		ConversationID: conversationID,
	}
	e.events = append(e.events, event)
	return event
}

func (v thresholdVAD) Speech(frame AudioFrame) bool {
	for _, sample := range frame.PCM {
		if sample >= v.threshold {
			return true
		}
	}
	return false
}

func TestLocalVoiceServiceStateTransitions(t *testing.T) {
	emitter := &recordingWakeEmitter{}
	service := NewLocalVoiceService(
		VoiceServiceConfig{Enabled: false, Muted: false},
		fixtureCapture{},
		thresholdVAD{threshold: 10},
		nil,
		emitter,
		nil,
	)
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start disabled service: %v", err)
	}
	service.Mute()
	service.Unmute()

	states := voiceStates(emitter.events)
	want := []string{"idle", "muted", "idle"}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("unexpected states: got %v want %v", states, want)
	}
}

func TestLocalVoiceServiceCapturesUtteranceWithPreRoll(t *testing.T) {
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	frames := []AudioFrame{
		fixtureFrame(start, 0, 0),
		fixtureFrame(start, 100*time.Millisecond, 1),
		fixtureFrame(start, 200*time.Millisecond, 2),
		fixtureFrame(start, 300*time.Millisecond, 42),
		fixtureFrame(start, 400*time.Millisecond, 50),
		fixtureFrame(start, 500*time.Millisecond, 0),
		fixtureFrame(start, 600*time.Millisecond, 0),
	}
	emitter := &recordingWakeEmitter{}
	var got []CapturedUtterance
	service := NewLocalVoiceService(
		VoiceServiceConfig{
			Enabled:         true,
			Muted:           false,
			PreRoll:         250 * time.Millisecond,
			SilenceDuration: 200 * time.Millisecond,
		},
		fixtureCapture{frames: frames},
		thresholdVAD{threshold: 10},
		nil,
		emitter,
		func(utterance CapturedUtterance) {
			got = append(got, utterance)
		},
	)

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}
	waitServiceDone(t, service)

	if len(got) != 1 {
		t.Fatalf("expected one utterance, got %d", len(got))
	}
	if len(got[0].Frames) != 5 {
		t.Fatalf("expected pre-roll plus speech/silence frames, got %d frames", len(got[0].Frames))
	}
	if got[0].Frames[0].PCM[0] != 2 || got[0].Frames[1].PCM[0] != 42 {
		t.Fatalf("expected pre-roll frame immediately before speech, got %+v", got[0].Frames[:2])
	}
	got[0].Frames[0].PCM[0] = 99
	if frames[2].PCM[0] == 99 {
		t.Fatalf("utterance leaked mutable fixture audio")
	}
	states := voiceStates(emitter.events)
	wantStates := []string{"wake_listening", "capturing_utterance", "processing", "wake_listening"}
	if !reflect.DeepEqual(states, wantStates) {
		t.Fatalf("unexpected states: got %v want %v", states, wantStates)
	}
}

func TestLocalVoiceServiceMaxUtteranceFlushesWithoutWaitingForSilence(t *testing.T) {
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	frames := []AudioFrame{
		fixtureFrame(start, 0, 42),
		fixtureFrame(start, 100*time.Millisecond, 50),
		fixtureFrame(start, 200*time.Millisecond, 60),
		fixtureFrame(start, 300*time.Millisecond, 0),
	}
	emitter := &recordingWakeEmitter{}
	var got []CapturedUtterance
	service := NewLocalVoiceService(
		VoiceServiceConfig{
			Enabled:      true,
			Muted:        false,
			MaxUtterance: 300 * time.Millisecond,
		},
		fixtureCapture{frames: frames},
		thresholdVAD{threshold: 10},
		nil,
		emitter,
		func(utterance CapturedUtterance) {
			got = append(got, utterance)
		},
	)

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}
	waitServiceDone(t, service)

	if len(got) != 1 {
		t.Fatalf("expected max utterance flush, got %d utterances", len(got))
	}
	if len(got[0].Frames) != 3 || got[0].EndedAt.Sub(got[0].StartedAt) != 300*time.Millisecond {
		t.Fatalf("unexpected max utterance capture: %+v", got[0])
	}
}

func TestLocalVoiceServiceCancelReturnsToWakeListeningWithoutUtterance(t *testing.T) {
	frames := make(chan AudioFrame)
	errs := make(chan error)
	capture := blockingCapture{frames: frames, errs: errs}
	emitter := &recordingWakeEmitter{}
	var got []CapturedUtterance
	service := NewLocalVoiceService(
		VoiceServiceConfig{Enabled: true},
		capture,
		thresholdVAD{threshold: 10},
		nil,
		emitter,
		func(utterance CapturedUtterance) {
			got = append(got, utterance)
		},
	)
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}
	service.Cancel()
	if len(got) != 0 {
		t.Fatalf("expected no utterance after cancel, got %d", len(got))
	}
	states := voiceStates(emitter.events)
	want := []string{"wake_listening"}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("unexpected states: got %v want %v", states, want)
	}
	service.Stop()
}

func TestLocalVoiceServiceCancelDiscardsActiveUtteranceAndContinuesListening(t *testing.T) {
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	frames := make(chan AudioFrame)
	errs := make(chan error)
	capture := blockingCapture{frames: frames, errs: errs}
	emitter := &recordingWakeEmitter{}
	utterances := make(chan CapturedUtterance, 1)
	service := NewLocalVoiceService(
		VoiceServiceConfig{
			Enabled:         true,
			Muted:           false,
			PreRoll:         time.Nanosecond,
			SilenceDuration: 100 * time.Millisecond,
		},
		capture,
		thresholdVAD{threshold: 10},
		nil,
		emitter,
		func(utterance CapturedUtterance) {
			utterances <- utterance
		},
	)
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}
	frames <- fixtureFrame(start, 0, 42)
	waitForState(t, emitter, WakeStateCapturingUtterance)

	service.Cancel()
	time.Sleep(10 * time.Millisecond)

	frames <- fixtureFrame(start, 300*time.Millisecond, 50)
	frames <- fixtureFrame(start, 400*time.Millisecond, 0)
	close(frames)
	close(errs)
	waitServiceDone(t, service)

	select {
	case utterance := <-utterances:
		if len(utterance.Frames) == 0 {
			t.Fatalf("expected post-cancel utterance frames")
		}
		for _, frame := range utterance.Frames {
			if frame.PCM[0] == 42 || frame.Timestamp.Equal(start) {
				t.Fatalf("cancelled utterance frame leaked into next capture: %+v", utterance.Frames)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("service did not capture a new utterance after cancel")
	}

	states := voiceStates(emitter.events)
	want := []string{
		"wake_listening",
		"capturing_utterance",
		"wake_listening",
		"capturing_utterance",
		"processing",
		"wake_listening",
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("unexpected states: got %v want %v", states, want)
	}
}

func TestLocalVoiceServiceMuteStopsActiveCaptureWithoutUtterance(t *testing.T) {
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	frames := make(chan AudioFrame)
	errs := make(chan error)
	capture := blockingCapture{frames: frames, errs: errs}
	emitter := &recordingWakeEmitter{}
	var got []CapturedUtterance
	service := NewLocalVoiceService(
		VoiceServiceConfig{Enabled: true, Muted: false, PreRoll: 100 * time.Millisecond},
		capture,
		thresholdVAD{threshold: 10},
		nil,
		emitter,
		func(utterance CapturedUtterance) {
			got = append(got, utterance)
		},
	)
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}
	frames <- fixtureFrame(start, 0, 42)
	waitForState(t, emitter, WakeStateCapturingUtterance)

	service.Mute()
	close(frames)
	close(errs)
	waitServiceDone(t, service)

	if len(got) != 0 {
		t.Fatalf("expected mute to discard active utterance, got %d", len(got))
	}
	states := voiceStates(emitter.events)
	want := []string{"wake_listening", "capturing_utterance", "muted"}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("unexpected states: got %v want %v", states, want)
	}
}

func TestLocalVoiceServiceCaptureErrorUsesSafeStateWithoutAudio(t *testing.T) {
	emitter := &recordingWakeEmitter{}
	service := NewLocalVoiceService(
		VoiceServiceConfig{Enabled: true},
		fixtureCapture{err: errors.New("pcm bytes: secret raw audio")},
		thresholdVAD{threshold: 10},
		nil,
		emitter,
		nil,
	)
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}
	waitServiceDone(t, service)

	payload := lastVoiceState(emitter.events)
	if payload.State != ServiceStateError || payload.ServiceStatus != "degraded" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
	states := voiceStates(emitter.events)
	want := []string{"wake_listening", "error"}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("unexpected states: got %v want %v", states, want)
	}
}

type blockingCapture struct {
	frames <-chan AudioFrame
	errs   <-chan error
}

func (b blockingCapture) Capture(context.Context) (<-chan AudioFrame, <-chan error) {
	return b.frames, b.errs
}

func fixtureFrame(start time.Time, offset time.Duration, value byte) AudioFrame {
	return AudioFrame{
		PCM:        []byte{value, value},
		SampleRate: 16000,
		Channels:   1,
		Timestamp:  start.Add(offset),
		Duration:   100 * time.Millisecond,
	}
}

func voiceStates(events []VoiceEvent) []string {
	states := make([]string, 0, len(events))
	for _, event := range events {
		if payload, ok := event.Payload.(VoiceStatePayload); ok {
			states = append(states, payload.State)
		}
	}
	return states
}

func lastVoiceState(events []VoiceEvent) VoiceStatePayload {
	for i := len(events) - 1; i >= 0; i-- {
		if payload, ok := events[i].Payload.(VoiceStatePayload); ok {
			return payload
		}
	}
	return VoiceStatePayload{}
}

func waitForState(t *testing.T, emitter *recordingWakeEmitter, state string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		for _, got := range voiceStates(emitter.events) {
			if got == state {
				return
			}
		}
		select {
		case <-deadline:
			t.Fatalf("voice service did not enter state %q; states=%v", state, voiceStates(emitter.events))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func waitServiceDone(t *testing.T, service *LocalVoiceService) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		service.mu.RLock()
		done := service.finished == nil
		service.mu.RUnlock()
		if done {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("voice service did not finish")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
