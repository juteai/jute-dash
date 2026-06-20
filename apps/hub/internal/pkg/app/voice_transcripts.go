package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
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
	sttProvider, err := s.activeSTTProvider(ctx, deviceProfileID)
	if err != nil {
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
			result, err := sttProvider.Transcribe(ctx, utterance)
			if err == nil {
				transcript, transcriptErr := service.FinalTranscriptFromSTT(result, deviceProfileID, deviceID)
				if transcriptErr != nil {
					err = transcriptErr
				} else {
					_, err = s.submitFinalTranscript(ctx, VoiceFinalTranscriptRequest{
						Text:            transcript.Text,
						DeviceProfileID: transcript.DeviceProfileID,
						DeviceID:        transcript.DeviceID,
					})
				}
			}
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

	speakBuffered := func(ctx context.Context, event service.Event, force bool) {
		text := strings.TrimSpace(ttsBuffer.String())
		if text == "" {
			return
		}
		if !force && !endsWithSentenceBoundary(text) {
			return
		}
		ttsBuffer.Reset()
		spokeFromDeltas = true
		s.speakVoiceAssistantText(ctx, deviceID, event.ConversationID, event.TaskID, text)
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
				speakBuffered(context.Background(), event, true)
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
				speakBuffered(context.Background(), event, false)
			}
		case service.EventTurnFailed:
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
	go func() {
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
	}()
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
