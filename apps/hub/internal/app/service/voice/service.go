package voice

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	ServiceStateError           = "error"
	WakeStateDetected           = "wake_detected"
	WakeStateCapturingUtterance = "capturing_utterance"
)

type AudioFrame struct {
	PCM         []byte
	SampleRate  int
	SampleWidth int
	Channels    int
	Timestamp   time.Time
	Duration    time.Duration
}

type CapturedUtterance struct {
	Frames     []AudioFrame
	StartedAt  time.Time
	EndedAt    time.Time
	SampleRate int
	Channels   int
}

type AudioCapture interface {
	Capture(ctx context.Context) (<-chan AudioFrame, <-chan error)
}

type VoiceActivityDetector interface {
	Speech(frame AudioFrame) bool
}

type VoiceServiceConfig struct {
	Enabled         bool
	Muted           bool
	DeviceID        string
	PreRoll         time.Duration
	SilenceDuration time.Duration
	MaxUtterance    time.Duration
}

type LocalVoiceService struct {
	capture AudioCapture
	vad     VoiceActivityDetector
	wake    WakeProvider
	emitter WakeEventEmitter
	onTurn  func(CapturedUtterance)

	mu       sync.RWMutex
	cfg      VoiceServiceConfig
	state    string
	lastErr  string
	cancel   context.CancelFunc
	reset    chan chan struct{}
	finished chan struct{}
}

func NewLocalVoiceService(
	cfg VoiceServiceConfig,
	capture AudioCapture,
	vad VoiceActivityDetector,
	wake WakeProvider,
	emitter WakeEventEmitter,
	onTurn func(CapturedUtterance),
) *LocalVoiceService {
	cfg = normalizeVoiceServiceConfig(cfg)
	service := &LocalVoiceService{
		capture: capture,
		vad:     vad,
		wake:    wake,
		emitter: emitter,
		onTurn:  onTurn,
		cfg:     cfg,
		state:   State(cfg.Enabled, cfg.Muted),
	}
	return service
}

func (s *LocalVoiceService) Start(ctx context.Context) error {
	if s.capture == nil {
		s.setError("audio capture is unavailable")
		return errors.New("audio capture is unavailable")
	}
	if s.vad == nil {
		s.setError("voice activity detector is unavailable")
		return errors.New("voice activity detector is unavailable")
	}

	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return errors.New("voice service is already running")
	}
	enabled := s.cfg.Enabled
	muted := s.cfg.Muted
	if !enabled || muted {
		s.mu.Unlock()
		s.emitState(State(enabled, muted))
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	reset := make(chan chan struct{}, 1)
	s.cancel = cancel
	s.reset = reset
	s.finished = make(chan struct{})
	s.mu.Unlock()

	frames, errs := s.capture.Capture(runCtx)
	s.emitState(State(enabled, muted))
	go s.run(runCtx, frames, errs, reset)
	return nil
}

func (s *LocalVoiceService) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	finished := s.finished
	s.reset = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if finished != nil {
		<-finished
	}
}

func (s *LocalVoiceService) Mute() {
	s.mu.Lock()
	s.cfg.Muted = true
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.emitState("muted")
}

func (s *LocalVoiceService) Unmute() {
	s.mu.Lock()
	s.cfg.Muted = false
	enabled := s.cfg.Enabled
	s.mu.Unlock()
	if enabled {
		s.emitState(State(true, false))
		return
	}
	s.emitState("idle")
}

func (s *LocalVoiceService) Cancel() {
	s.mu.Lock()
	reset := s.reset
	enabled := s.cfg.Enabled
	muted := s.cfg.Muted
	s.mu.Unlock()
	acknowledged := false
	if reset != nil {
		done := make(chan struct{})
		select {
		case reset <- done:
			select {
			case <-done:
				acknowledged = true
			case <-time.After(250 * time.Millisecond):
			}
		default:
		}
	}
	if !acknowledged {
		s.emitState(State(enabled, muted))
	}
}

func (s *LocalVoiceService) run(
	ctx context.Context,
	frames <-chan AudioFrame,
	errs <-chan error,
	reset <-chan chan struct{},
) {
	defer func() {
		s.mu.Lock()
		if s.finished != nil {
			close(s.finished)
		}
		s.cancel = nil
		s.reset = nil
		s.finished = nil
		s.mu.Unlock()
	}()

	var pre preRollBuffer
	var active []AudioFrame
	var speechStarted time.Time
	var silence time.Duration
	for {
		select {
		case <-ctx.Done():
			return
		case done := <-reset:
			pre = preRollBuffer{}
			active = nil
			speechStarted = time.Time{}
			silence = 0
			s.emitStateIfDifferent(State(true, false))
			close(done)
		case err, ok := <-errs:
			if ok && err != nil {
				s.setError("audio capture failed")
				return
			}
		case frame, ok := <-frames:
			if !ok {
				select {
				case err, ok := <-errs:
					if ok && err != nil {
						s.setError("audio capture failed")
						return
					}
				default:
				}
				if len(active) > 0 {
					s.finishUtterance(active)
				}
				s.emitStateIfDifferent(State(true, false))
				return
			}
			frame = cloneAudioFrame(frame)
			pre.add(frame, s.preRoll())
			speaking := s.vad.Speech(frame)
			if len(active) == 0 {
				if !speaking {
					continue
				}
				active = pre.frames()
				speechStarted = frame.Timestamp
				silence = 0
				s.emitState(WakeStateCapturingUtterance)
				continue
			}
			active = append(active, frame)
			if speaking {
				silence = 0
			} else {
				silence += frame.Duration
			}
			if s.maxUtterance() > 0 && frame.Timestamp.Sub(speechStarted)+frame.Duration >= s.maxUtterance() {
				s.finishUtterance(active)
				active = nil
				s.emitState(State(true, false))
				continue
			}
			if silence >= s.silenceDuration() {
				s.finishUtterance(active)
				active = nil
				s.emitState(State(true, false))
			}
		}
	}
}

func (s *LocalVoiceService) emitStateIfDifferent(state string) {
	s.mu.RLock()
	current := s.state
	s.mu.RUnlock()
	if current == state {
		return
	}
	s.emitState(state)
}

func (s *LocalVoiceService) finishUtterance(frames []AudioFrame) {
	if len(frames) == 0 || s.onTurn == nil {
		return
	}
	first := frames[0]
	last := frames[len(frames)-1]
	utterance := CapturedUtterance{
		Frames:     cloneAudioFrames(frames),
		StartedAt:  first.Timestamp,
		EndedAt:    last.Timestamp.Add(last.Duration),
		SampleRate: first.SampleRate,
		Channels:   first.Channels,
	}
	if s.wake != nil {
		detection, err := s.wake.DetectWake(context.Background(), utterance)
		if err != nil || !detection.Detected {
			s.emitStateIfDifferent(State(true, false))
			return
		}
		if s.emitter != nil {
			conversationID := newID("voice-conversation")
			s.emitter.EmitVoiceWakeDetected(s.deviceID(), conversationID)
			s.emitter.EmitVoiceStateChanged(s.deviceID(), VoiceStatePayload{
				Enabled:       true,
				Muted:         false,
				State:         WakeStateDetected,
				ServiceStatus: "ready",
			})
		}
	}
	s.emitStateIfDifferent("processing")
	s.onTurn(utterance)
}

func (s *LocalVoiceService) emitState(state string) {
	s.mu.Lock()
	s.state = state
	s.lastErr = ""
	cfg := s.cfg
	deviceID := s.deviceIDLocked()
	status := s.serviceStatusLocked()
	s.mu.Unlock()
	if s.emitter != nil {
		s.emitter.EmitVoiceStateChanged(deviceID, VoiceStatePayload{
			Enabled:       cfg.Enabled,
			Muted:         cfg.Muted,
			State:         state,
			ServiceStatus: status,
		})
	}
}

func (s *LocalVoiceService) setError(message string) {
	s.mu.Lock()
	s.state = ServiceStateError
	s.lastErr = sanitizeText(message)
	cfg := s.cfg
	deviceID := s.deviceIDLocked()
	s.mu.Unlock()
	if s.emitter != nil {
		s.emitter.EmitVoiceStateChanged(deviceID, VoiceStatePayload{
			Enabled:       cfg.Enabled,
			Muted:         cfg.Muted,
			State:         ServiceStateError,
			ServiceStatus: "degraded",
		})
	}
}

func (s *LocalVoiceService) deviceIDLocked() string {
	if s.cfg.DeviceID == "" {
		return DefaultDeviceProfileID
	}
	return s.cfg.DeviceID
}

func (s *LocalVoiceService) deviceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deviceIDLocked()
}

func (s *LocalVoiceService) serviceStatusLocked() string {
	if s.lastErr != "" {
		return "degraded"
	}
	if !s.cfg.Enabled {
		return "disabled"
	}
	if s.cfg.Muted {
		return "muted"
	}
	return "ready"
}

func (s *LocalVoiceService) preRoll() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.PreRoll
}

func (s *LocalVoiceService) silenceDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.SilenceDuration
}

func (s *LocalVoiceService) maxUtterance() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.MaxUtterance
}

func normalizeVoiceServiceConfig(cfg VoiceServiceConfig) VoiceServiceConfig {
	if cfg.DeviceID == "" {
		cfg.DeviceID = DefaultDeviceProfileID
	}
	if cfg.PreRoll == 0 {
		cfg.PreRoll = 500 * time.Millisecond
	}
	if cfg.SilenceDuration == 0 {
		cfg.SilenceDuration = 300 * time.Millisecond
	}
	if cfg.MaxUtterance == 0 {
		cfg.MaxUtterance = 30 * time.Second
	}
	return cfg
}

type preRollBuffer struct {
	items []AudioFrame
}

func (b *preRollBuffer) add(frame AudioFrame, window time.Duration) {
	b.items = append(b.items, frame)
	start := len(b.items) - 1
	total := b.items[start].Duration
	for i := len(b.items) - 2; i >= 0; i-- {
		if total+b.items[i].Duration > window {
			break
		}
		total += b.items[i].Duration
		start = i
	}
	b.items = b.items[start:]
}

func (b *preRollBuffer) frames() []AudioFrame {
	return cloneAudioFrames(b.items)
}

func cloneAudioFrames(frames []AudioFrame) []AudioFrame {
	out := make([]AudioFrame, len(frames))
	for i, frame := range frames {
		out[i] = cloneAudioFrame(frame)
	}
	return out
}

func cloneAudioFrame(frame AudioFrame) AudioFrame {
	if frame.PCM != nil {
		frame.PCM = append([]byte(nil), frame.PCM...)
	}
	return frame
}
