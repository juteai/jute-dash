package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"jute-dash/apps/hub/internal/app/service"
	"jute-dash/apps/hub/internal/pkg/httphelper"
)

type VoiceFinalTranscriptRequest struct {
	Text            string `json:"text"`
	DeviceProfileID string `json:"deviceProfileId,omitempty"`
	DeviceID        string `json:"deviceId,omitempty"`
	ConversationID  string `json:"conversationId,omitempty"`
	AgentID         string `json:"agentId,omitempty"`
	wakeDetected    bool
}

type VoiceFinalTranscriptResponse struct {
	Conversation service.ConversationDetail `json:"conversation"`
	Followup     VoiceFollowupResponse      `json:"followup"`
}

type VoiceFollowupResponse struct {
	Active    bool   `json:"active"`
	ExpiresAt string `json:"expiresAt,omitempty"`
	Turns     int    `json:"turns"`
	MaxTurns  int    `json:"maxTurns"`
}

const maxVoiceAudioBytes = 2 * 1024 * 1024
const minTTSChunkBytes = 220

type voiceTranscriptError struct {
	status  int
	message string
}

func (e voiceTranscriptError) Error() string {
	return e.message
}

func (s *Server) handleVoiceFinalTranscript(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req VoiceFinalTranscriptRequest
	if err := decodeStrictJSON(r.Body, &req); err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid final transcript request")
		return
	}
	s.handleFinalTranscriptRequest(w, r, req)
}

func (s *Server) handleVoiceAudio(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	pcm, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxVoiceAudioBytes))
	if err != nil {
		httphelper.WriteError(w, http.StatusRequestEntityTooLarge, "voice audio is too large")
		return
	}
	if len(pcm) == 0 || len(pcm)%service.DefaultSampleWidth != 0 {
		httphelper.WriteError(w, http.StatusBadRequest, "voice audio PCM is required")
		return
	}
	sampleRate := headerInt(r, "X-Jute-Sample-Rate", service.DefaultSampleRate)
	channels := headerInt(r, "X-Jute-Channels", service.DefaultChannels)
	if sampleRate < 8000 || sampleRate > 48000 || channels != service.DefaultChannels {
		httphelper.WriteError(w, http.StatusBadRequest, "unsupported voice audio format")
		return
	}
	requireWake := r.URL.Query().Get("wake") == "true"
	slog.Default().DebugContext(r.Context(), "voice audio received",
		"bytes", len(pcm),
		"sample_rate", sampleRate,
		"channels", channels,
		"wake_required", requireWake,
		"device_profile_id", strings.TrimSpace(r.Header.Get("X-Jute-Device-Profile-Id")),
		"device_id", strings.TrimSpace(r.Header.Get("X-Jute-Device-Id")),
		"conversation_id", strings.TrimSpace(r.Header.Get("X-Jute-Conversation-Id")),
	)
	utterance := service.UtteranceFromPCM(pcm, sampleRate, channels, time.Now().UTC(), 20*time.Millisecond)
	if !utteranceHasSpeech(utterance, service.EnergyVAD{Threshold: s.cfg.Voice.VADThreshold}) {
		slog.Default().DebugContext(r.Context(), "voice audio rejected without speech",
			"bytes", len(pcm),
			"sample_rate", sampleRate,
			"vad_threshold", s.cfg.Voice.VADThreshold,
		)
		httphelper.WriteError(w, http.StatusBadRequest, "speech is required")
		return
	}
	req := VoiceFinalTranscriptRequest{
		DeviceProfileID: strings.TrimSpace(r.Header.Get("X-Jute-Device-Profile-Id")),
		DeviceID:        strings.TrimSpace(r.Header.Get("X-Jute-Device-Id")),
		ConversationID:  strings.TrimSpace(r.Header.Get("X-Jute-Conversation-Id")),
		AgentID:         strings.TrimSpace(r.Header.Get("X-Jute-Agent-Id")),
	}
	if requireWake {
		detected, err := s.detectWake(r.Context(), req, utterance)
		if err != nil {
			slog.Default().WarnContext(r.Context(), "voice audio wake detection unavailable",
				"device_profile_id", req.DeviceProfileID,
				"device_id", req.DeviceID,
				"error", err,
			)
			httphelper.WriteError(w, http.StatusInternalServerError, "wake detection is unavailable")
			return
		}
		if !detected {
			slog.Default().DebugContext(r.Context(), "voice audio wake not detected",
				"device_profile_id", req.DeviceProfileID,
				"device_id", req.DeviceID,
			)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		req.wakeDetected = true
		response, err := s.submitVoiceUtterance(r.Context(), req, utterance)
		if err != nil {
			var transcriptErr voiceTranscriptError
			if errors.As(err, &transcriptErr) {
				httphelper.WriteError(w, transcriptErr.status, transcriptErr.message)
				return
			}
			httphelper.WriteError(w, http.StatusInternalServerError, "voice conversation is unavailable")
			return
		}
		httphelper.WriteJSON(w, http.StatusOK, response)
		return
	}
	response, err := s.submitVoiceUtterance(r.Context(), req, utterance)
	if err != nil {
		var transcriptErr voiceTranscriptError
		if errors.As(err, &transcriptErr) {
			httphelper.WriteError(w, transcriptErr.status, transcriptErr.message)
			return
		}
		httphelper.WriteError(w, http.StatusInternalServerError, "voice audio could not be processed")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, response)
}

func (s *Server) beginWakeConversation(
	ctx context.Context,
	req VoiceFinalTranscriptRequest,
) (VoiceFinalTranscriptResponse, error) {
	settings, err := s.voiceStore.VoiceSettings(ctx, req.DeviceProfileID)
	if err != nil {
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusInternalServerError,
			message: "voice settings are unavailable",
		}
	}
	if !settings.Enabled || settings.Muted {
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusConflict,
			message: "voice is not listening",
		}
	}
	session, started, err := s.voiceRuntime.BeginTurn(
		req.ConversationID,
		settings,
		voiceSourceDeviceProfileID(req, settings),
		deviceID(req),
	)
	if err != nil {
		if errors.Is(err, service.ErrFollowupSourceMismatch) {
			return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
				status:  http.StatusConflict,
				message: "voice follow-up source mismatch",
			}
		}
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusConflict,
			message: "voice follow-up window expired",
		}
	}
	agentID := s.voiceAgentID(req.AgentID, settings)
	conversationID := session.ConversationID
	slog.Default().InfoContext(ctx, "voice wake conversation started",
		"conversation_id", conversationID,
		"agent_id", agentID,
		"device_profile_id", voiceSourceDeviceProfileID(req, settings),
		"device_id", deviceID(req),
		"new_session", started,
	)
	if s.voiceDispatcher != nil {
		s.voiceDispatcher.EmitVoiceWakeDetected(deviceID(req), conversationID)
		s.voiceDispatcher.EmitVoiceStateChanged(deviceID(req), service.VoiceStatePayload{
			Enabled:       true,
			Muted:         false,
			State:         service.WakeStateDetected,
			ServiceStatus: "ready",
		})
		if started {
			s.voiceDispatcher.EmitConversationEvent(
				service.EventConversationStarted,
				deviceID(req),
				conversationID,
				map[string]any{"agentId": agentID},
			)
		}
	}
	return VoiceFinalTranscriptResponse{
		Followup: VoiceFollowupResponse{
			Active:    true,
			ExpiresAt: session.ExpiresAt.Format(time.RFC3339Nano),
			Turns:     session.Turns,
			MaxTurns:  service.MaxConversationTurns,
		},
	}, nil
}

func (s *Server) detectWake(
	ctx context.Context,
	req VoiceFinalTranscriptRequest,
	utterance service.CapturedUtterance,
) (bool, error) {
	providerStore, ok := s.voiceStore.(interface {
		ActiveWakeProvider(context.Context, string, string) (service.WakeProvider, error)
	})
	if !ok {
		return false, errors.New("wake provider store is unavailable")
	}
	wake, err := providerStore.ActiveWakeProvider(ctx, req.DeviceProfileID, req.DeviceID)
	if err != nil || wake == nil {
		return false, err
	}
	detection, err := wake.DetectWake(ctx, utterance)
	if err != nil {
		return false, err
	}
	if !detection.Detected {
		slog.Default().DebugContext(ctx, "voice wake not detected",
			"device_profile_id", req.DeviceProfileID,
			"device_id", req.DeviceID,
			"provider_id", detection.ProviderID,
			"model_id", detection.ModelID,
			"confidence", detection.Confidence,
		)
		return false, nil
	}
	slog.Default().InfoContext(ctx, "voice wake detected",
		"device_profile_id", req.DeviceProfileID,
		"device_id", req.DeviceID,
		"provider_id", detection.ProviderID,
		"model_id", detection.ModelID,
		"confidence", detection.Confidence,
	)
	return true, nil
}

func (s *Server) activeSTTProvider(ctx context.Context, deviceProfileID string) (service.STTProvider, error) {
	providerStore, ok := s.voiceStore.(interface {
		ActiveSTTProvider(context.Context, string) (service.STTProvider, error)
	})
	if !ok {
		return nil, errors.New("STT provider store is unavailable")
	}
	provider, err := providerStore.ActiveSTTProvider(ctx, deviceProfileID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("STT provider is unavailable")
	}
	return provider, nil
}

func (s *Server) newLocalVoiceService(
	ctx context.Context,
	deviceProfileID string,
	deviceID string,
	capture service.AudioCapture,
	vad service.VoiceActivityDetector,
) (*service.LocalVoiceService, error) {
	settings, err := s.voiceStore.VoiceSettings(ctx, deviceProfileID)
	if err != nil {
		return nil, err
	}
	if deviceProfileID == "" {
		deviceProfileID = settings.DeviceProfileID
	}
	if deviceID == "" {
		deviceID = deviceProfileID
	}
	if _, err := s.activeSTTProvider(ctx, deviceProfileID); err != nil {
		return nil, err
	}
	var wakeProvider service.WakeProvider
	if providerStore, ok := s.voiceStore.(interface {
		ActiveWakeProvider(context.Context, string, string) (service.WakeProvider, error)
	}); ok {
		wakeProvider, err = providerStore.ActiveWakeProvider(ctx, deviceProfileID, deviceID)
		if err != nil {
			return nil, err
		}
	}
	return service.NewLocalVoiceService(
		service.VoiceServiceConfig{
			Enabled:  settings.Enabled,
			Muted:    settings.Muted,
			DeviceID: deviceID,
		},
		capture,
		vad,
		wakeProvider,
		s.voiceDispatcher,
		func(utterance service.CapturedUtterance) {
			_, err := s.submitVoiceUtterance(ctx, VoiceFinalTranscriptRequest{
				DeviceProfileID: deviceProfileID,
				DeviceID:        deviceID,
			}, utterance)
			if err != nil && s.voiceDispatcher != nil {
				s.voiceDispatcher.EmitVoiceStateChanged(deviceID, service.VoiceStatePayload{
					Enabled:       settings.Enabled,
					Muted:         settings.Muted,
					State:         service.ServiceStateError,
					ServiceStatus: "degraded",
				})
			}
		},
	), nil
}

func (s *Server) submitVoiceUtterance(
	ctx context.Context,
	req VoiceFinalTranscriptRequest,
	utterance service.CapturedUtterance,
) (VoiceFinalTranscriptResponse, error) {
	sttProvider, err := s.activeSTTProvider(ctx, req.DeviceProfileID)
	if err != nil {
		return VoiceFinalTranscriptResponse{}, err
	}
	started := time.Now()
	slog.Default().DebugContext(ctx, "voice stt started",
		"device_profile_id", req.DeviceProfileID,
		"device_id", req.DeviceID,
		"frames", len(utterance.Frames),
		"audio_ms", utterance.EndedAt.Sub(utterance.StartedAt).Milliseconds(),
	)
	result, err := sttProvider.Transcribe(ctx, utterance)
	if err != nil {
		slog.Default().WarnContext(ctx, "voice stt failed",
			"device_profile_id", req.DeviceProfileID,
			"device_id", req.DeviceID,
			"duration_ms", time.Since(started).Milliseconds(),
			"error", err,
		)
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusServiceUnavailable,
			message: "transcription_failed",
		}
	}
	slog.Default().InfoContext(ctx, "voice stt completed",
		"device_profile_id", req.DeviceProfileID,
		"device_id", req.DeviceID,
		"provider_id", result.ProviderID,
		"model_id", result.ModelID,
		"language", result.Language,
		"duration_ms", time.Since(started).Milliseconds(),
		"transcript_bytes", len(result.Text),
	)
	transcript, err := service.FinalTranscriptFromSTT(result, req.DeviceProfileID, req.DeviceID)
	if err != nil {
		return VoiceFinalTranscriptResponse{}, err
	}
	req.Text = transcript.Text
	req.DeviceProfileID = transcript.DeviceProfileID
	req.DeviceID = transcript.DeviceID
	if req.wakeDetected {
		settings, err := s.voiceStore.VoiceSettings(ctx, req.DeviceProfileID)
		if err != nil {
			return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
				status:  http.StatusInternalServerError,
				message: "voice settings are unavailable",
			}
		}
		req.Text = stripWakePhraseFromTranscript(req.Text, settings)
		if req.Text == "" {
			return s.beginWakeConversation(ctx, req)
		}
	}
	return s.submitFinalTranscript(ctx, req)
}

func stripWakePhraseFromTranscript(text string, settings service.Settings) string {
	text = strings.TrimSpace(text)
	for _, phrase := range []string{
		settings.WakeWordPhrase,
		strings.NewReplacer("_", " ", "-", " ").Replace(settings.WakeWordModelID),
	} {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		if len(text) < len(phrase) || !strings.EqualFold(text[:len(phrase)], phrase) {
			continue
		}
		return strings.TrimSpace(strings.TrimLeft(text[len(phrase):], " ,.!?:;-"))
	}
	return text
}

func (s *Server) handleFinalTranscriptRequest(
	w http.ResponseWriter,
	r *http.Request,
	req VoiceFinalTranscriptRequest,
) {
	response, err := s.submitFinalTranscript(r.Context(), req)
	if err != nil {
		var transcriptErr voiceTranscriptError
		if errors.As(err, &transcriptErr) {
			httphelper.WriteError(w, transcriptErr.status, transcriptErr.message)
			return
		}
		httphelper.WriteError(w, http.StatusInternalServerError, "voice transcript could not be processed")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, response)
}

func headerInt(r *http.Request, name string, fallback int) int {
	value := strings.TrimSpace(r.Header.Get(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func utteranceHasSpeech(utterance service.CapturedUtterance, vad service.VoiceActivityDetector) bool {
	for _, frame := range utterance.Frames {
		if vad.Speech(frame) {
			return true
		}
	}
	return false
}

func (s *Server) submitFinalTranscript(
	ctx context.Context,
	req VoiceFinalTranscriptRequest,
) (VoiceFinalTranscriptResponse, error) {
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusBadRequest,
			message: "text is required",
		}
	}
	settings, err := s.voiceStore.VoiceSettings(ctx, req.DeviceProfileID)
	if err != nil {
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusInternalServerError,
			message: "voice settings are unavailable",
		}
	}
	if !settings.Enabled || settings.Muted {
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusConflict,
			message: "voice is not listening",
		}
	}

	session, started, err := s.voiceRuntime.BeginTurn(
		req.ConversationID,
		settings,
		voiceSourceDeviceProfileID(req, settings),
		deviceID(req),
	)
	if err != nil {
		if errors.Is(err, service.ErrFollowupExpired) {
			s.voiceDispatcher.EmitConversationEvent(
				service.EventConversationEnded,
				deviceID(req),
				req.ConversationID,
				map[string]any{"reason": "followup_expired"},
			)
			return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
				status:  http.StatusConflict,
				message: "voice follow-up window expired",
			}
		}
		if errors.Is(err, service.ErrFollowupSourceMismatch) {
			return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
				status:  http.StatusConflict,
				message: "voice follow-up source mismatch",
			}
		}
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusInternalServerError,
			message: "voice conversation is unavailable",
		}
	}
	conversationID := session.ConversationID
	agentID := s.voiceAgentID(req.AgentID, settings)
	slog.Default().InfoContext(ctx, "voice final transcript routed",
		"conversation_id", conversationID,
		"agent_id", agentID,
		"device_profile_id", voiceSourceDeviceProfileID(req, settings),
		"device_id", deviceID(req),
		"new_session", started,
		"text_bytes", len(req.Text),
	)

	if req.wakeDetected {
		s.voiceDispatcher.EmitVoiceWakeDetected(deviceID(req), conversationID)
		s.voiceDispatcher.EmitVoiceStateChanged(deviceID(req), service.VoiceStatePayload{
			Enabled:       true,
			Muted:         false,
			State:         service.WakeStateDetected,
			ServiceStatus: "ready",
		})
	}
	if started {
		s.voiceDispatcher.EmitConversationEvent(
			service.EventConversationStarted,
			deviceID(req),
			conversationID,
			map[string]any{"agentId": agentID},
		)
	}
	s.voiceDispatcher.EmitVoiceTranscript(
		service.EventVoiceTranscriptFinal,
		deviceID(req),
		conversationID,
		req.Text,
	)

	detail, err := s.turnRunner.Run(
		ctx,
		conversationID,
		service.ConversationTurnRequest{
			AgentID: agentID,
			Text:    req.Text,
		},
		s.voiceAgentEventCallback(deviceID(req)),
	)
	if err != nil {
		slog.Default().WarnContext(ctx, "voice agent turn failed",
			"conversation_id", conversationID,
			"agent_id", agentID,
			"device_id", deviceID(req),
			"error", err,
		)
		s.voiceRuntime.End(conversationID)
		s.voiceDispatcher.EmitConversationEvent(
			service.EventConversationEnded,
			deviceID(req),
			conversationID,
			map[string]any{"reason": "agent_failure"},
		)
		return VoiceFinalTranscriptResponse{}, voiceTranscriptError{
			status:  http.StatusBadGateway,
			message: "agent turn could not be completed",
		}
	}

	session = s.voiceRuntime.CompleteTurn(conversationID, settings)
	if service.ConversationComplete(session) {
		slog.Default().InfoContext(ctx, "voice conversation completed",
			"conversation_id", conversationID,
			"agent_id", agentID,
			"device_id", deviceID(req),
			"turns", session.Turns,
			"reason", "followup_limit_reached",
		)
		s.voiceRuntime.End(conversationID)
		s.voiceDispatcher.EmitConversationEvent(
			service.EventConversationEnded,
			deviceID(req),
			conversationID,
			map[string]any{
				"reason":   "followup_limit_reached",
				"turns":    session.Turns,
				"maxTurns": service.MaxConversationTurns,
			},
		)
		return VoiceFinalTranscriptResponse{
			Conversation: detail,
			Followup: VoiceFollowupResponse{
				Active:   false,
				Turns:    session.Turns,
				MaxTurns: service.MaxConversationTurns,
			},
		}, nil
	}
	s.voiceDispatcher.EmitConversationEvent(
		service.EventConversationFollowupStarted,
		deviceID(req),
		conversationID,
		map[string]any{
			"expiresAt": session.ExpiresAt.Format(time.RFC3339Nano),
			"turns":     session.Turns,
			"maxTurns":  service.MaxConversationTurns,
		},
	)
	slog.Default().InfoContext(ctx, "voice follow-up listening",
		"conversation_id", conversationID,
		"agent_id", agentID,
		"device_id", deviceID(req),
		"turns", session.Turns,
		"expires_at", session.ExpiresAt.Format(time.RFC3339Nano),
	)

	return VoiceFinalTranscriptResponse{
		Conversation: detail,
		Followup: VoiceFollowupResponse{
			Active:    true,
			ExpiresAt: session.ExpiresAt.Format(time.RFC3339Nano),
			Turns:     session.Turns,
			MaxTurns:  service.MaxConversationTurns,
		},
	}, nil
}

func decodeStrictJSON(r io.Reader, dst any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON data")
	}
	return nil
}

func (s *Server) voiceAgentID(requested string, settings service.Settings) string {
	if agentID := strings.TrimSpace(requested); agentID != "" {
		return agentID
	}
	if agentID := strings.TrimSpace(settings.PreferredAgentID); agentID != "" {
		return agentID
	}
	if agentID := strings.TrimSpace(s.cfg.Voice.PreferredAgentID); agentID != "" {
		return agentID
	}
	for _, agent := range s.agentsManager.List(context.Background(), false) {
		if agent.Enabled {
			return agent.ID
		}
	}
	return ""
}

func (s *Server) voiceAgentEventCallback(deviceID string) func(service.Event) error {
	var ttsBuffer strings.Builder
	spokeFromDeltas := false
	type ttsChunk struct {
		conversationID string
		taskID         string
		text           string
	}
	ttsQueue := make(chan ttsChunk, 8)
	var startTTSWorker sync.Once
	var closeTTSQueue sync.Once

	enqueueTTS := func(event service.Event, text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		startTTSWorker.Do(func() {
			go func() {
				for chunk := range ttsQueue {
					s.speakVoiceAssistantTextNow(
						context.Background(),
						deviceID,
						chunk.conversationID,
						chunk.taskID,
						chunk.text,
					)
				}
			}()
		})
		ttsQueue <- ttsChunk{
			conversationID: event.ConversationID,
			taskID:         event.TaskID,
			text:           text,
		}
	}

	speakBuffered := func(event service.Event, force bool) {
		text := strings.TrimSpace(ttsBuffer.String())
		if text == "" {
			return
		}
		if !force && !readyForTTSChunk(text) {
			return
		}
		ttsBuffer.Reset()
		spokeFromDeltas = true
		enqueueTTS(event, text)
	}

	return func(event service.Event) error {
		switch event.Kind {
		case service.EventTurnStarted:
			s.voiceDispatcher.EmitConversationEvent(
				service.EventConversationTurnStarted,
				deviceID,
				event.ConversationID,
				map[string]any{
					"agentId": event.AgentID,
					"status":  event.Status,
				},
			)
		case service.EventTurnCompleted:
			payload := map[string]any{
				"agentId": event.AgentID,
				"status":  "completed",
			}
			assistantText := ""
			if event.Detail != nil {
				payload["status"] = event.Detail.Conversation.Status
				payload["taskId"] = event.Detail.Conversation.LatestTaskID
				if text := latestAssistantMessageText(*event.Detail); text != "" {
					assistantText = text
					payload["text"] = text
				}
			}
			s.voiceDispatcher.EmitConversationEvent(
				service.EventConversationTurnCompleted,
				deviceID,
				event.ConversationID,
				payload,
			)
			if ttsBuffer.Len() > 0 {
				speakBuffered(event, true)
			} else if !spokeFromDeltas && assistantText != "" {
				taskID := event.TaskID
				if event.Detail != nil && event.Detail.Conversation.LatestTaskID != "" {
					taskID = event.Detail.Conversation.LatestTaskID
				}
				s.speakVoiceAssistantText(
					context.Background(),
					deviceID,
					event.ConversationID,
					taskID,
					assistantText,
				)
			}
			closeTTSQueue.Do(func() { close(ttsQueue) })
		case service.EventStatusChanged:
			s.voiceDispatcher.EmitConversationEvent(
				service.EventConversationTurnStarted,
				deviceID,
				event.ConversationID,
				map[string]any{
					"agentId": event.AgentID,
					"taskId":  event.TaskID,
					"status":  event.Status,
				},
			)
		case service.EventAssistantDelta:
			s.voiceDispatcher.EmitConversationEvent(
				service.EventConversationAssistantDelta,
				deviceID,
				event.ConversationID,
				map[string]any{
					"agentId": event.AgentID,
					"taskId":  event.TaskID,
					"text":    event.Text,
					"append":  event.Append,
				},
			)
			if event.Text != "" {
				if !event.Append {
					ttsBuffer.Reset()
				}
				ttsBuffer.WriteString(event.Text)
				speakBuffered(event, false)
			}
		case service.EventTurnFailed:
			closeTTSQueue.Do(func() { close(ttsQueue) })
			s.voiceDispatcher.EmitConversationEvent(
				service.EventConversationTurnCompleted,
				deviceID,
				event.ConversationID,
				map[string]any{
					"agentId": event.AgentID,
					"status":  "failed",
				},
			)
		}
		return nil
	}
}

func (s *Server) speakVoiceAssistantText(
	ctx context.Context,
	deviceID, conversationID, taskID, text string,
) {
	go s.speakVoiceAssistantTextNow(ctx, deviceID, conversationID, taskID, text)
}

func (s *Server) speakVoiceAssistantTextNow(
	ctx context.Context,
	deviceID, conversationID, taskID, text string,
) {
	text = strings.TrimSpace(text)
	if text == "" || s.voiceSpeaker == nil {
		return
	}
	settings, err := s.voiceStore.VoiceSettings(ctx, service.DefaultDeviceProfileID)
	if err != nil ||
		!settings.TTSEnabled ||
		strings.TrimSpace(settings.TTSProviderID) == "" {
		return
	}
	ttsCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()
	_, _ = s.voiceSpeaker.Speak(
		ttsCtx,
		deviceID,
		service.TTSActionSpeak,
		service.TTSRequest{
			Text:           text,
			ConversationID: conversationID,
			TurnID:         taskID,
		},
	)
}

func readyForTTSChunk(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return endsWithSentenceBoundary(text) || len(text) >= minTTSChunkBytes
}

func endsWithSentenceBoundary(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	switch text[len(text)-1] {
	case '.', '!', '?', '\n':
		return true
	default:
		return false
	}
}

func latestAssistantMessageText(detail service.ConversationDetail) string {
	for i := len(detail.Messages) - 1; i >= 0; i-- {
		message := detail.Messages[i]
		if message.Role == "assistant" {
			if text := strings.TrimSpace(message.Content); text != "" {
				return text
			}
		}
	}
	return ""
}

func deviceID(req VoiceFinalTranscriptRequest) string {
	if id := strings.TrimSpace(req.DeviceID); id != "" {
		return id
	}
	if id := strings.TrimSpace(req.DeviceProfileID); id != "" {
		return id
	}
	return "default-display"
}

func voiceSourceDeviceProfileID(req VoiceFinalTranscriptRequest, settings service.Settings) string {
	if id := strings.TrimSpace(req.DeviceProfileID); id != "" {
		return id
	}
	if id := strings.TrimSpace(settings.DeviceProfileID); id != "" {
		return id
	}
	return "default-display"
}
