package voice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"jute-dash/apps/hub/internal/pkg/httphelper"
)

type Store interface {
	VoiceSettings(ctx context.Context, deviceProfileID string) (Settings, error)
	SaveVoiceSettings(ctx context.Context, req SettingsUpdateRequest) (Settings, error)
	SetVoiceMuted(ctx context.Context, deviceProfileID string, muted bool) (Settings, error)
	CancelVoice(ctx context.Context, deviceProfileID string) (Settings, error)
	VoiceProviders(ctx context.Context) ([]ProviderPack, error)
	TTSVoices(ctx context.Context, providerID, deviceProfileID string) (TTSVoicesResponse, error)
}

type DisplayEmitter interface {
	EmitVoiceStateChanged(deviceProfileID string, payload VoiceStatePayload) VoiceEvent
	EmitConversationEvent(eventType, deviceID, conversationID string, payload any) VoiceEvent
	EmitTTSEvent(eventType, deviceID string, response TTSActionResponse) VoiceEvent
}

type CancelledConversation struct {
	ConversationID string
	DeviceID       string
}

type TTSProvider interface {
	Synthesize(ctx context.Context, req TTSRequest) (TTSAudioResult, error)
}

type Controller struct {
	store   Store
	display DisplayEmitter
	speaker *Speaker
	cancel  func() []CancelledConversation
}

func NewController(store Store, display DisplayEmitter, cancel func() []CancelledConversation) *Controller {
	return NewControllerWithSpeaker(store, display, cancel, NewSpeaker(store, display, nil))
}

func NewControllerWithSpeaker(
	store Store,
	display DisplayEmitter,
	cancel func() []CancelledConversation,
	speaker *Speaker,
) *Controller {
	if speaker == nil {
		speaker = NewSpeaker(store, display, nil)
	}
	return &Controller{
		store:   store,
		display: display,
		speaker: speaker,
		cancel:  cancel,
	}
}

func NewControllerWithTTSProvider(
	store Store,
	display DisplayEmitter,
	cancel func() []CancelledConversation,
	provider TTSProvider,
) *Controller {
	return NewControllerWithSpeaker(store, display, cancel, NewSpeaker(store, display, provider))
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

func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
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

func (c *Controller) handleVoiceSettings(w http.ResponseWriter, r *http.Request) {
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
		c.display.EmitVoiceStateChanged(status.DeviceProfileID, VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *Controller) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
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

func (c *Controller) handleVoiceMute(w http.ResponseWriter, r *http.Request) {
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
		c.display.EmitVoiceStateChanged("default-display", VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	if c.cancel != nil {
		c.cancel()
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *Controller) handleVoiceUnmute(w http.ResponseWriter, r *http.Request) {
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
		c.display.EmitVoiceStateChanged("default-display", VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *Controller) handleVoiceCancel(w http.ResponseWriter, r *http.Request) {
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
		c.display.EmitVoiceStateChanged("default-display", VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	var cancelled []CancelledConversation
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
				EventConversationEnded,
				deviceID,
				conversation.ConversationID,
				map[string]any{"reason": "canceled"},
			)
		}
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *Controller) handleVoiceProviders(w http.ResponseWriter, r *http.Request) {
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

func (c *Controller) handleTTSVoices(w http.ResponseWriter, r *http.Request) {
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

func (c *Controller) handleTTSSpeak(w http.ResponseWriter, r *http.Request) {
	c.handleTTSAction(w, r, TTSActionSpeak)
}

func (c *Controller) handleTTSAction(w http.ResponseWriter, r *http.Request, action string) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := DecodeTTSRequest(r.Body)
	if err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid TTS request")
		return
	}
	response, err := c.speaker.Speak(r.Context(), DefaultDeviceProfileID, action, req)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "voice settings are unavailable")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, response)
}

func (c *Controller) handleTTSStop(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req := TTSStopRequest{}
	if r.Body != nil {
		decoded, err := DecodeTTSStopRequest(r.Body)
		if err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid TTS stop request")
			return
		}
		req = decoded
	}
	response := c.speaker.Stop(DefaultDeviceProfileID, req)
	httphelper.WriteJSON(w, http.StatusOK, response)
}
