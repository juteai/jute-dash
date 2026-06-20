package controller

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"jute-dash/apps/hub/internal/app/service"
	"jute-dash/apps/hub/internal/pkg/httphelper"
)

type VoiceStore interface {
	VoiceSettings(ctx context.Context, deviceProfileID string) (Settings, error)
	SaveVoiceSettings(ctx context.Context, req SettingsUpdateRequest) (Settings, error)
	SetVoiceMuted(ctx context.Context, deviceProfileID string, muted bool) (Settings, error)
	CancelVoice(ctx context.Context, deviceProfileID string) (Settings, error)
	VoiceProviders(ctx context.Context) ([]ProviderPack, error)
	TTSVoices(ctx context.Context, providerID, deviceProfileID string) (TTSVoicesResponse, error)
}

type VoiceDisplayEmitter interface {
	EmitVoiceStateChanged(deviceProfileID string, payload service.VoiceStatePayload) service.VoiceEvent
	EmitConversationEvent(eventType, deviceID, conversationID string, payload any) service.VoiceEvent
	EmitTTSEvent(eventType, deviceID string, response service.TTSActionResponse) service.VoiceEvent
}

type VoiceController struct {
	store   VoiceStore
	display VoiceDisplayEmitter
	speaker *service.Speaker
	cancel  func() []service.CancelledConversation
	restart func(context.Context)
	mute    func(bool)
	reset   func()
}

func NewVoiceController(
	store VoiceStore,
	display VoiceDisplayEmitter,
	cancel func() []service.CancelledConversation,
) *VoiceController {
	return NewVoiceControllerWithSpeaker(store, display, cancel, service.NewSpeaker(store, display, nil))
}

func NewVoiceControllerWithSpeaker(
	store VoiceStore,
	display VoiceDisplayEmitter,
	cancel func() []service.CancelledConversation,
	speaker *service.Speaker,
) *VoiceController {
	if speaker == nil {
		speaker = service.NewSpeaker(store, display, nil)
	}
	return &VoiceController{
		store:   store,
		display: display,
		speaker: speaker,
		cancel:  cancel,
	}
}

func NewVoiceControllerWithTTSProvider(
	store VoiceStore,
	display VoiceDisplayEmitter,
	cancel func() []service.CancelledConversation,
	provider service.TTSProvider,
) *VoiceController {
	return NewVoiceControllerWithSpeaker(store, display, cancel, service.NewSpeaker(store, display, provider))
}

func (c *VoiceController) OnRuntimeChanged(
	restart func(context.Context),
	mute func(bool),
	reset func(),
) *VoiceController {
	c.restart = restart
	c.mute = mute
	c.reset = reset
	return c
}

func DecodeSettingsUpdateRequest(r io.Reader) (SettingsUpdateRequest, error) {
	var req SettingsUpdateRequest
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return SettingsUpdateRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return SettingsUpdateRequest{}, errors.New("trailing JSON data")
	}
	return req, nil
}

func (c *VoiceController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/voice/status", c.handleVoiceStatus)
	mux.HandleFunc("/api/v1/voice/settings", c.handleVoiceSettings)
	mux.HandleFunc("/api/v1/voice/mute", c.handleVoiceMute)
	mux.HandleFunc("/api/v1/voice/unmute", c.handleVoiceUnmute)
	mux.HandleFunc("/api/v1/voice/cancel", c.handleVoiceCancel)
	mux.HandleFunc("/api/v1/voice/providers", c.handleVoiceProviders)
	mux.HandleFunc("/api/v1/tts/voices", c.handleTTSVoices)
	mux.HandleFunc("/api/v1/tts/speak", c.handleTTSSpeak)
	mux.HandleFunc("/api/v1/tts/stop", c.handleTTSStop)
}

func (c *VoiceController) handleVoiceSettings(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPatch) {
		return
	}
	req, err := DecodeSettingsUpdateRequest(r.Body)
	if err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid voice settings request")
		return
	}
	settings, err := c.store.SaveVoiceSettings(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "invalid voice settings") {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid voice settings")
			return
		}
		httphelper.WriteError(w, http.StatusInternalServerError, "voice settings could not be saved")
		return
	}
	status := StatusFromSettings(settings)
	if c.display != nil {
		c.display.EmitVoiceStateChanged(status.DeviceProfileID, service.VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	if c.restart != nil {
		c.restart(r.Context())
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *VoiceController) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodGet) {
		return
	}
	settings, err := c.store.VoiceSettings(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "voice settings are unavailable")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, StatusFromSettings(settings))
}

func (c *VoiceController) handleVoiceMute(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	settings, err := c.store.SetVoiceMuted(r.Context(), "", true)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "could not mute voice")
		return
	}
	status := StatusFromSettings(settings)
	if c.display != nil {
		c.display.EmitVoiceStateChanged("default-display", service.VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	if c.mute != nil {
		c.mute(true)
	}
	if c.cancel != nil {
		c.cancel()
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *VoiceController) handleVoiceUnmute(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	settings, err := c.store.SetVoiceMuted(r.Context(), "", false)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "could not unmute voice")
		return
	}
	status := StatusFromSettings(settings)
	if c.display != nil {
		c.display.EmitVoiceStateChanged("default-display", service.VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	if c.mute != nil {
		c.mute(false)
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *VoiceController) handleVoiceCancel(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	settings, err := c.store.CancelVoice(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "could not cancel voice operation")
		return
	}
	status := StatusFromSettings(settings)
	if c.display != nil {
		c.display.EmitVoiceStateChanged("default-display", service.VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	var cancelled []service.CancelledConversation
	if c.cancel != nil {
		cancelled = c.cancel()
	}
	if c.display != nil {
		for _, conversation := range cancelled {
			if conversation.ConversationID == "" {
				continue
			}
			deviceID := conversation.DeviceID
			if deviceID == "" {
				deviceID = "default-display"
			}
			c.display.EmitConversationEvent(
				service.EventConversationEnded,
				deviceID,
				conversation.ConversationID,
				map[string]any{"reason": "canceled"},
			)
		}
	}
	if c.reset != nil {
		c.reset()
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *VoiceController) handleVoiceProviders(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodGet) {
		return
	}
	providers, err := c.store.VoiceProviders(r.Context())
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "could not list voice providers")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (c *VoiceController) handleTTSVoices(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodGet) {
		return
	}
	query := r.URL.Query()
	voices, err := c.store.TTSVoices(
		r.Context(),
		query.Get("providerId"),
		query.Get("deviceProfileId"),
	)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "could not list TTS voices")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, voices)
}

func (c *VoiceController) handleTTSSpeak(w http.ResponseWriter, r *http.Request) {
	c.handleTTSAction(w, r, service.TTSActionSpeak)
}

func (c *VoiceController) handleTTSAction(w http.ResponseWriter, r *http.Request, action string) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := service.DecodeTTSRequest(r.Body)
	if err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid TTS request")
		return
	}
	response, err := c.speaker.Speak(r.Context(), service.DefaultDeviceProfileID, action, req)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "voice settings are unavailable")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, response)
}

func (c *VoiceController) handleTTSStop(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req := service.TTSStopRequest{}
	if r.Body != nil {
		decoded, err := service.DecodeTTSStopRequest(r.Body)
		if err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid TTS stop request")
			return
		}
		req = decoded
	}
	response := c.speaker.Stop(service.DefaultDeviceProfileID, req)
	httphelper.WriteJSON(w, http.StatusOK, response)
}
