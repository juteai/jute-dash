package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	a2aclient "jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/home"
	"jute-dash/internal/registry"
	"jute-dash/internal/store"
	"jute-dash/internal/weather"
)

type Server struct {
	cfg      config.Config
	registry registry.Registry
	weather  weather.Provider
	messages a2aclient.MessageSender
	setup    store.SetupStatus
	layout   store.WidgetLayout
	started  time.Time
	version  string
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
}

type MessageRequest struct {
	AgentID string `json:"agentId"`
	Text    string `json:"text"`
}

type MessageResponse struct {
	ConversationID string `json:"conversationId"`
	AgentID        string `json:"agentId"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

func New(cfg config.Config, version string) http.Handler {
	return NewWithWeatherProvider(cfg, version, weather.NewClient())
}

func NewWithWeatherProvider(cfg config.Config, version string, weatherProvider weather.Provider) http.Handler {
	return newServer(cfg, version, weatherProvider, nil, store.SetupStatus{Complete: true, Missing: []string{}}, store.DefaultWidgetLayout())
}

func NewWithMessageSender(cfg config.Config, version string, messageSender a2aclient.MessageSender) http.Handler {
	return newServer(cfg, version, weather.NewClient(), messageSender, store.SetupStatus{Complete: true, Missing: []string{}}, store.DefaultWidgetLayout())
}

func NewWithSetupStatus(cfg config.Config, version string, setup store.SetupStatus) http.Handler {
	return NewWithSetupStatusAndLayout(cfg, version, setup, store.DefaultWidgetLayout())
}

func NewWithSetupStatusAndLayout(cfg config.Config, version string, setup store.SetupStatus, layout store.WidgetLayout) http.Handler {
	return newServer(cfg, version, weather.NewClient(), nil, setup, layout)
}

func newServer(cfg config.Config, version string, weatherProvider weather.Provider, messageSender a2aclient.MessageSender, setup store.SetupStatus, layout store.WidgetLayout) http.Handler {
	if weatherProvider == nil {
		weatherProvider = weather.NewClient()
	}
	if messageSender == nil {
		messageSender = a2aclient.NewJSONRPCClient()
	}
	server := &Server{
		cfg:      cfg,
		registry: registry.New(cfg.Agents),
		weather:  weatherProvider,
		messages: messageSender,
		setup:    normalizeSetupStatus(setup),
		layout:   normalizeWidgetLayout(layout),
		started:  time.Now().UTC(),
		version:  version,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/config", server.handleConfig)
	mux.HandleFunc("/api/v1/home", server.handleHome)
	mux.HandleFunc("/api/v1/agents", server.handleAgents)
	mux.HandleFunc("/api/v1/messages", server.handleMessages)
	mux.HandleFunc("/api/v1/setup/status", server.handleSetupStatus)
	mux.HandleFunc("/api/v1/widgets/layout", server.handleWidgetLayout)

	return withCommonHeaders(mux)
}

func normalizeSetupStatus(setup store.SetupStatus) store.SetupStatus {
	if setup.Missing == nil {
		setup.Missing = []string{}
	}
	return setup
}

func normalizeWidgetLayout(layout store.WidgetLayout) store.WidgetLayout {
	if strings.TrimSpace(layout.ProfileID) == "" {
		layout.ProfileID = store.DefaultWidgetLayout().ProfileID
	}
	if layout.Widgets == nil {
		layout.Widgets = []store.WidgetInstance{}
	}
	for i := range layout.Widgets {
		if layout.Widgets[i].Settings == nil {
			layout.Widgets[i].Settings = map[string]any{}
		}
	}
	return layout
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Version:   s.version,
		StartedAt: s.started,
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.Public())
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, home.FromConfig(s.cfg, time.Now(), s.weather.Current(r.Context(), s.cfg.Weather)))
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": s.registry.List(),
	})
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.setup)
}

func (s *Server) handleWidgetLayout(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.layout)
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Text = strings.TrimSpace(req.Text)
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agentId is required")
		return
	}
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	agent, ok := s.registry.Find(req.AgentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if !agent.Enabled {
		writeError(w, http.StatusConflict, "agent is disabled")
		return
	}

	configuredAgent, ok := s.configuredAgent(req.AgentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if configuredAgent.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
		return
	}

	bearerToken, ok := agentBearerToken(configuredAgent)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "agent credentials are not available")
		return
	}

	result, err := s.messages.SendMessage(r.Context(), a2aclient.SendMessageRequest{
		EndpointURL:     configuredAgent.EndpointURL,
		ProtocolBinding: configuredAgent.ProtocolBinding,
		Text:            req.Text,
		BearerToken:     bearerToken,
	})
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, a2aclient.ErrUnsupportedProtocol) {
			status = http.StatusNotImplemented
		}
		writeError(w, status, "agent request failed")
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{
		ConversationID: result.ConversationID,
		AgentID:        agent.ID,
		Status:         result.Status,
		Message:        result.Text,
	})
}

func (s *Server) configuredAgent(id string) (config.AgentConfig, bool) {
	for _, agent := range s.cfg.Agents {
		if agent.ID == id {
			return agent, true
		}
	}
	return config.AgentConfig{}, false
}

func agentBearerToken(agent config.AgentConfig) (string, bool) {
	if agent.Auth == nil {
		return "", true
	}
	if !strings.EqualFold(strings.TrimSpace(agent.Auth.Type), "bearer") {
		return "", false
	}
	token := strings.TrimSpace(os.Getenv(agent.Auth.EnvToken))
	if token == "" {
		return "", false
	}
	return token, true
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
