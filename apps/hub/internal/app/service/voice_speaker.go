package service

import (
	"context"
	"strings"
)

type Speaker struct {
	store       VoiceStore
	display     VoiceDisplayEmitter
	tts         *TTSRuntime
	ttsProvider TTSProvider
}

func NewSpeaker(store VoiceStore, display VoiceDisplayEmitter, provider TTSProvider) *Speaker {
	return &Speaker{
		store:       store,
		display:     display,
		tts:         NewTTSRuntime(),
		ttsProvider: provider,
	}
}

func (s *Speaker) Speak(ctx context.Context, deviceID, action string, req TTSRequest) (TTSActionResponse, error) {
	settings, err := s.store.VoiceSettings(ctx, "")
	if err != nil {
		return TTSActionResponse{}, err
	}
	req = effectiveTTSRequest(req, settings)
	allowed, reason := speechPolicyAllows(req, settings)
	synthesisCtx, cancelSynthesis := context.WithCancel(ctx)
	defer cancelSynthesis()
	response := s.tts.Begin(action, req, settings, cancelSynthesis)
	if !allowed {
		response = s.tts.VisualOnly(response.ID, reason)
		if s.display != nil {
			s.display.EmitTTSEvent(EventTTSStopped, deviceID, response)
		}
		return response, nil
	}
	if s.display != nil {
		s.display.EmitTTSEvent(EventTTSStarted, deviceID, response)
	}
	if provider, err := s.activeProvider(synthesisCtx); err != nil {
		response = s.tts.Fail(response.ID, "provider_unavailable")
		if s.display != nil {
			s.display.EmitTTSEvent(EventTTSFailed, deviceID, response)
		}
		return response, nil
	} else if provider != nil {
		audio, err := provider.Synthesize(synthesisCtx, req)
		if err != nil {
			response = s.tts.Fail(response.ID, "provider_unavailable")
			if response.State == TTSStateStopped {
				return response, nil
			}
			if s.display != nil {
				s.display.EmitTTSEvent(EventTTSFailed, deviceID, response)
			}
			return response, nil
		}
		response = s.tts.CompleteWithAudio(response.ID, audio)
	} else {
		response.State = TTSStatePlayback
		response = s.tts.Complete(response.ID)
	}
	if response.State == TTSStateStopped {
		return response, nil
	}
	if s.display != nil {
		s.display.EmitTTSEvent(EventTTSCompleted, deviceID, response)
	}
	return response, nil
}

func (s *Speaker) Stop(deviceID string, req TTSStopRequest) TTSActionResponse {
	response := s.tts.Stop(req)
	if s.display != nil {
		if strings.TrimSpace(deviceID) == "" {
			deviceID = DefaultDeviceProfileID
		}
		s.display.EmitTTSEvent(EventTTSStopped, deviceID, response)
	}
	return response
}

func (s *Speaker) activeProvider(ctx context.Context) (TTSProvider, error) {
	if store, ok := s.store.(interface {
		ActiveTTSProvider(ctx context.Context, deviceProfileID string) (TTSProvider, error)
	}); ok {
		return store.ActiveTTSProvider(ctx, "")
	}
	return s.ttsProvider, nil
}
