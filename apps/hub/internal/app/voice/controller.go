package voice

import (
	"context"
	"encoding/json"
	"net/http"

	"jute-dash/apps/hub/internal/pkg/displayactions"
)

type Store interface {
	VoiceSettings(ctx context.Context, deviceProfileID string) (Settings, error)
	SetVoiceMuted(ctx context.Context, deviceProfileID string, muted bool) (Settings, error)
	CancelVoice(ctx context.Context, deviceProfileID string) (Settings, error)
	VoiceProviders(ctx context.Context) ([]ProviderPack, error)
}

type DisplayEmitter interface {
	EmitVoiceStateChanged(deviceProfileID string, payload displayactions.VoiceStatePayload) displayactions.VoiceEvent
}

type Controller struct {
	store   Store
	display DisplayEmitter
}

func NewController(store Store, display DisplayEmitter) *Controller {
	return &Controller{
		store:   store,
		display: display,
	}
}

func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/voice/status", c.handleVoiceStatus)
	mux.HandleFunc("/api/v1/voice/mute", c.handleVoiceMute)
	mux.HandleFunc("/api/v1/voice/unmute", c.handleVoiceUnmute)
	mux.HandleFunc("/api/v1/voice/cancel", c.handleVoiceCancel)
	mux.HandleFunc("/api/v1/voice/providers", c.handleVoiceProviders)
}

func (c *Controller) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		c.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	settings, err := c.store.VoiceSettings(r.Context(), "")
	if err != nil {
		c.writeError(w, http.StatusInternalServerError, "voice settings are unavailable")
		return
	}
	c.writeJSON(w, http.StatusOK, StatusFromSettings(settings))
}

func (c *Controller) handleVoiceMute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		c.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	settings, err := c.store.SetVoiceMuted(r.Context(), "", true)
	if err != nil {
		c.writeError(w, http.StatusInternalServerError, "could not mute voice")
		return
	}
	status := StatusFromSettings(settings)
	if c.display != nil {
		c.display.EmitVoiceStateChanged("default-display", displayactions.VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	c.writeJSON(w, http.StatusOK, status)
}

func (c *Controller) handleVoiceUnmute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		c.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	settings, err := c.store.SetVoiceMuted(r.Context(), "", false)
	if err != nil {
		c.writeError(w, http.StatusInternalServerError, "could not unmute voice")
		return
	}
	status := StatusFromSettings(settings)
	if c.display != nil {
		c.display.EmitVoiceStateChanged("default-display", displayactions.VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	c.writeJSON(w, http.StatusOK, status)
}

func (c *Controller) handleVoiceCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		c.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	settings, err := c.store.CancelVoice(r.Context(), "")
	if err != nil {
		c.writeError(w, http.StatusInternalServerError, "could not cancel voice operation")
		return
	}
	status := StatusFromSettings(settings)
	if c.display != nil {
		c.display.EmitVoiceStateChanged("default-display", displayactions.VoiceStatePayload{
			Enabled:       status.Enabled,
			Muted:         status.Muted,
			State:         status.State,
			ServiceStatus: status.ServiceStatus,
		})
	}
	c.writeJSON(w, http.StatusOK, status)
}

func (c *Controller) handleVoiceProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		c.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	providers, err := c.store.VoiceProviders(r.Context())
	if err != nil {
		c.writeError(w, http.StatusInternalServerError, "could not list voice providers")
		return
	}
	c.writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// Helpers

func (c *Controller) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (c *Controller) writeError(w http.ResponseWriter, status int, message string) {
	c.writeJSON(w, status, map[string]string{"error": message})
}

func (c *Controller) writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	c.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
