package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	a2aclient "jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/home"
	"jute-dash/internal/registry"
	"jute-dash/internal/store"
	"jute-dash/internal/weather"
)

type Server struct {
	cfg           config.Config
	registry      registry.Registry
	weather       weather.Provider
	messages      a2aclient.MessageSender
	cardFetcher   *a2aclient.AgentCardFetcher
	setup         store.SetupStatus
	layout        store.WidgetLayout
	layoutStore   WidgetLayoutStore
	cardStore     AgentCardStore
	conversations ConversationStore
	events        *EventBroker
	voice         config.VoiceConfig
	voiceStore    VoiceSettingsStore
	mu            sync.Mutex
	started       time.Time
	version       string
}

type WidgetLayoutStore interface {
	WidgetLayout(ctx context.Context, profileID string) (store.WidgetLayout, error)
	SaveWidgetLayout(ctx context.Context, layout store.WidgetLayout) (store.WidgetLayout, error)
	ResetWidgetLayout(ctx context.Context, profileID string) (store.WidgetLayout, error)
}

type VoiceSettingsStore interface {
	VoiceSettings(ctx context.Context, deviceProfileID string) (store.VoiceSettings, error)
	SetVoiceMuted(ctx context.Context, deviceProfileID string, muted bool) (store.VoiceSettings, error)
	CancelVoice(ctx context.Context, deviceProfileID string) (store.VoiceSettings, error)
	VoiceProviders(ctx context.Context) ([]store.VoiceProviderPack, error)
}

type AgentCardStore interface {
	AgentCardCache(ctx context.Context, agentID string) (store.AgentCardCache, error)
	SaveAgentCardCache(ctx context.Context, cache store.AgentCardCache) error
}

type ConversationStore interface {
	CreateConversation(ctx context.Context, conversation store.Conversation) (store.Conversation, error)
	ListConversations(ctx context.Context) ([]store.Conversation, error)
	Conversation(ctx context.Context, id string) (store.ConversationDetail, error)
	AddConversationMessage(ctx context.Context, message store.ConversationMessage) (store.ConversationMessage, error)
	UpdateConversationMessage(ctx context.Context, messageID, content, status, taskID string, appendContent bool) (store.ConversationMessage, error)
	UpdateConversationState(ctx context.Context, conversationID, status, a2aContextID, taskID string) (store.Conversation, error)
	DeleteConversation(ctx context.Context, id string) error
	AddConversationEvent(ctx context.Context, event store.ConversationEvent) (store.ConversationEvent, error)
	ConversationEventsSince(ctx context.Context, sinceID int64) ([]store.ConversationEvent, error)
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
}

type MessageRequest struct {
	AgentID        string `json:"agentId"`
	Text           string `json:"text"`
	ConversationID string `json:"conversationId,omitempty"`
}

type MessageResponse struct {
	ConversationID string `json:"conversationId"`
	TaskID         string `json:"taskId,omitempty"`
	AgentID        string `json:"agentId"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

type VoiceStatusResponse struct {
	Enabled                 bool   `json:"enabled"`
	Muted                   bool   `json:"muted"`
	State                   string `json:"state"`
	ServiceStatus           string `json:"serviceStatus"`
	DeviceProfileID         string `json:"deviceProfileId"`
	WakeWordModelID         string `json:"wakeWordModelId"`
	STTProviderID           string `json:"sttProviderId"`
	TTSProviderID           string `json:"ttsProviderId"`
	STTModelID              string `json:"sttModelId"`
	TTSModelID              string `json:"ttsModelId"`
	TTSVoiceID              string `json:"ttsVoiceId"`
	PreferredAgentID        string `json:"preferredAgentId"`
	CloudOptIn              bool   `json:"cloudOptIn"`
	CommandProvidersEnabled bool   `json:"commandProvidersEnabled"`
	FollowupWindowSeconds   int    `json:"followupWindowSeconds"`
	MicrophoneProfile       string `json:"microphoneProfile"`
	UpdatedAt               string `json:"updatedAt"`
}

func New(cfg config.Config, version string) http.Handler {
	return NewWithWeatherProvider(cfg, version, weather.NewClient())
}

func NewWithWeatherProvider(cfg config.Config, version string, weatherProvider weather.Provider) http.Handler {
	return newServer(cfg, version, weatherProvider, nil, store.SetupStatus{Complete: true, Missing: []string{}}, store.DefaultWidgetLayout(), nil)
}

func NewWithMessageSender(cfg config.Config, version string, messageSender a2aclient.MessageSender) http.Handler {
	return newServer(cfg, version, weather.NewClient(), messageSender, store.SetupStatus{Complete: true, Missing: []string{}}, store.DefaultWidgetLayout(), nil)
}

func NewWithSetupStatus(cfg config.Config, version string, setup store.SetupStatus) http.Handler {
	return NewWithSetupStatusAndLayout(cfg, version, setup, store.DefaultWidgetLayout())
}

func NewWithSetupStatusAndLayout(cfg config.Config, version string, setup store.SetupStatus, layout store.WidgetLayout) http.Handler {
	return newServer(cfg, version, weather.NewClient(), nil, setup, layout, nil)
}

func NewWithSetupStatusAndLayoutStore(cfg config.Config, version string, setup store.SetupStatus, layoutStore WidgetLayoutStore) http.Handler {
	layout := store.DefaultWidgetLayout()
	if layoutStore != nil {
		if loaded, err := layoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	return newServer(cfg, version, weather.NewClient(), nil, setup, layout, layoutStore)
}

func newServer(cfg config.Config, version string, weatherProvider weather.Provider, messageSender a2aclient.MessageSender, setup store.SetupStatus, layout store.WidgetLayout, layoutStore WidgetLayoutStore) http.Handler {
	if weatherProvider == nil {
		weatherProvider = weather.NewClient()
	}
	if messageSender == nil {
		messageSender = a2aclient.NewJSONRPCClient()
	}
	var voiceStore VoiceSettingsStore
	if candidate, ok := layoutStore.(VoiceSettingsStore); ok {
		voiceStore = candidate
	}
	var cardStore AgentCardStore
	if candidate, ok := layoutStore.(AgentCardStore); ok {
		cardStore = candidate
	}
	var conversationStore ConversationStore
	if candidate, ok := layoutStore.(ConversationStore); ok {
		conversationStore = candidate
	}
	server := &Server{
		cfg:           cfg,
		registry:      registry.New(cfg.Agents),
		weather:       weatherProvider,
		messages:      messageSender,
		cardFetcher:   a2aclient.NewAgentCardFetcher(),
		setup:         normalizeSetupStatus(setup),
		layout:        normalizeWidgetLayout(layout),
		layoutStore:   layoutStore,
		cardStore:     cardStore,
		conversations: conversationStore,
		events:        NewEventBroker(),
		voice:         cfg.Voice,
		voiceStore:    voiceStore,
		started:       time.Now().UTC(),
		version:       version,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/config", server.handleConfig)
	mux.HandleFunc("/api/v1/home", server.handleHome)
	mux.HandleFunc("/api/v1/agents", server.handleAgents)
	mux.HandleFunc("/api/v1/agents/", server.handleAgentSubroutes)
	mux.HandleFunc("/api/v1/messages", server.handleMessages)
	mux.HandleFunc("/api/v1/conversations", server.handleConversations)
	mux.HandleFunc("/api/v1/conversations/", server.handleConversationSubroutes)
	mux.HandleFunc("/api/v1/events", server.handleEvents)
	mux.HandleFunc("/api/v1/setup/status", server.handleSetupStatus)
	mux.HandleFunc("/api/v1/widgets/catalog", server.handleWidgetCatalog)
	mux.HandleFunc("/api/v1/widgets/layout", server.handleWidgetLayout)
	mux.HandleFunc("/api/v1/widgets/layout/reset", server.handleWidgetLayoutReset)
	mux.HandleFunc("/api/v1/voice/status", server.handleVoiceStatus)
	mux.HandleFunc("/api/v1/voice/mute", server.handleVoiceMute)
	mux.HandleFunc("/api/v1/voice/unmute", server.handleVoiceUnmute)
	mux.HandleFunc("/api/v1/voice/cancel", server.handleVoiceCancel)
	mux.HandleFunc("/api/v1/voice/providers", server.handleVoiceProviders)

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
		"agents": s.agentsWithDiscovery(r.Context(), true),
	})
}

func (s *Server) handleAgentSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[1] != "refresh-card" {
		writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	agentID := strings.TrimSpace(parts[0])
	agent, ok := s.registry.Find(agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	cache := s.refreshAgentCard(r.Context(), agent)
	enriched := s.agentWithDiscovery(agent, cache)
	writeJSON(w, http.StatusOK, enriched)
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.setup)
}

func (s *Server) handleWidgetCatalog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"widgets": store.WidgetCatalog(),
	})
}

func (s *Server) handleWidgetLayout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		layout, err := s.currentWidgetLayout(r.Context(), r.URL.Query().Get("profileId"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "widget layout is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, layout)
	case http.MethodPut:
		var layout store.WidgetLayout
		if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		if strings.TrimSpace(layout.ProfileID) == "" {
			layout.ProfileID = s.layout.ProfileID
		}
		saved, err := s.saveWidgetLayout(r.Context(), layout)
		if err != nil {
			if errors.Is(err, store.ErrInvalidLayout) {
				writeError(w, http.StatusBadRequest, "invalid widget layout")
				return
			}
			writeError(w, http.StatusInternalServerError, "widget layout could not be saved")
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (s *Server) handleWidgetLayoutReset(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	profileID := strings.TrimSpace(r.URL.Query().Get("profileId"))
	if profileID == "" {
		profileID = s.layout.ProfileID
	}
	layout := store.DefaultWidgetLayout()
	layout.ProfileID = profileID

	var saved store.WidgetLayout
	var err error
	if s.layoutStore != nil {
		saved, err = s.layoutStore.ResetWidgetLayout(r.Context(), profileID)
	} else {
		saved, err = store.NormalizeWidgetLayout(layout)
		if err == nil {
			s.layout = saved
		}
	}
	if err != nil {
		if errors.Is(err, store.ErrInvalidLayout) {
			writeError(w, http.StatusBadRequest, "invalid widget layout")
			return
		}
		writeError(w, http.StatusInternalServerError, "widget layout could not be reset")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	status, err := s.currentVoiceStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice status is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceMute(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	status, err := s.setVoiceMuted(r.Context(), true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice mute state could not be updated")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceUnmute(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	status, err := s.setVoiceMuted(r.Context(), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice mute state could not be updated")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceCancel(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	status, err := s.cancelVoice(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice state could not be cancelled")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleVoiceProviders(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	providers, err := s.voiceProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voice providers are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (s *Server) currentWidgetLayout(ctx context.Context, profileID string) (store.WidgetLayout, error) {
	if s.layoutStore == nil {
		return normalizeWidgetLayout(s.layout), nil
	}
	layout, err := s.layoutStore.WidgetLayout(ctx, profileID)
	if err != nil {
		return store.WidgetLayout{}, err
	}
	s.layout = normalizeWidgetLayout(layout)
	return s.layout, nil
}

func (s *Server) saveWidgetLayout(ctx context.Context, layout store.WidgetLayout) (store.WidgetLayout, error) {
	var saved store.WidgetLayout
	var err error
	if s.layoutStore != nil {
		saved, err = s.layoutStore.SaveWidgetLayout(ctx, layout)
	} else {
		saved, err = store.NormalizeWidgetLayout(layout)
	}
	if err != nil {
		return store.WidgetLayout{}, err
	}
	s.layout = normalizeWidgetLayout(saved)
	return s.layout, nil
}

func (s *Server) currentVoiceStatus(ctx context.Context) (VoiceStatusResponse, error) {
	if s.voiceStore != nil {
		settings, err := s.voiceStore.VoiceSettings(ctx, "")
		if err != nil {
			return VoiceStatusResponse{}, err
		}
		return voiceStatusFromSettings(settings), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return voiceStatusFromConfig(s.voice, time.Now().UTC()), nil
}

func (s *Server) setVoiceMuted(ctx context.Context, muted bool) (VoiceStatusResponse, error) {
	if s.voiceStore != nil {
		settings, err := s.voiceStore.SetVoiceMuted(ctx, "", muted)
		if err != nil {
			return VoiceStatusResponse{}, err
		}
		return voiceStatusFromSettings(settings), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.voice.MutedByDefault = muted
	return voiceStatusFromConfig(s.voice, time.Now().UTC()), nil
}

func (s *Server) cancelVoice(ctx context.Context) (VoiceStatusResponse, error) {
	if s.voiceStore != nil {
		settings, err := s.voiceStore.CancelVoice(ctx, "")
		if err != nil {
			return VoiceStatusResponse{}, err
		}
		return voiceStatusFromSettings(settings), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return voiceStatusFromConfig(s.voice, time.Now().UTC()), nil
}

func (s *Server) voiceProviders(ctx context.Context) ([]store.VoiceProviderPack, error) {
	if s.voiceStore == nil {
		return []store.VoiceProviderPack{}, nil
	}
	return s.voiceStore.VoiceProviders(ctx)
}

func voiceStatusFromSettings(settings store.VoiceSettings) VoiceStatusResponse {
	return VoiceStatusResponse{
		Enabled:                 settings.Enabled,
		Muted:                   settings.Muted,
		State:                   voiceState(settings.Enabled, settings.Muted),
		ServiceStatus:           voiceServiceStatus(settings.Enabled, settings.STTProviderID, settings.TTSProviderID),
		DeviceProfileID:         settings.DeviceProfileID,
		WakeWordModelID:         settings.WakeWordModelID,
		STTProviderID:           settings.STTProviderID,
		TTSProviderID:           settings.TTSProviderID,
		STTModelID:              settings.STTModelID,
		TTSModelID:              settings.TTSModelID,
		TTSVoiceID:              settings.TTSVoiceID,
		PreferredAgentID:        settings.PreferredAgentID,
		CloudOptIn:              settings.CloudOptIn,
		CommandProvidersEnabled: settings.CommandProvidersEnabled,
		FollowupWindowSeconds:   settings.FollowupWindowSeconds,
		MicrophoneProfile:       settings.MicrophoneProfile,
		UpdatedAt:               settings.UpdatedAt,
	}
}

func voiceStatusFromConfig(voice config.VoiceConfig, now time.Time) VoiceStatusResponse {
	return VoiceStatusResponse{
		Enabled:                 voice.Enabled,
		Muted:                   voice.MutedByDefault,
		State:                   voiceState(voice.Enabled, voice.MutedByDefault),
		ServiceStatus:           voiceServiceStatus(voice.Enabled, voice.STTProviderID, voice.TTSProviderID),
		DeviceProfileID:         "default-display",
		WakeWordModelID:         voice.WakeWordModelID,
		STTProviderID:           voice.STTProviderID,
		TTSProviderID:           voice.TTSProviderID,
		STTModelID:              voice.STTModelID,
		TTSModelID:              voice.TTSModelID,
		TTSVoiceID:              voice.TTSVoiceID,
		PreferredAgentID:        voice.PreferredAgentID,
		CloudOptIn:              voice.CloudOptIn,
		CommandProvidersEnabled: voice.CommandProvidersEnabled,
		FollowupWindowSeconds:   voice.FollowupWindowSeconds,
		MicrophoneProfile:       voice.MicrophoneProfile,
		UpdatedAt:               now.Format(time.RFC3339Nano),
	}
}

func voiceState(enabled, muted bool) string {
	if muted {
		return "muted"
	}
	if enabled {
		return "wake_listening"
	}
	return "idle"
}

func voiceServiceStatus(enabled bool, sttProviderID, _ string) string {
	if !enabled || strings.TrimSpace(sttProviderID) == "" {
		return "not_configured"
	}
	return "ready"
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
	selected := s.selectedAgentInterface(r.Context(), agent)
	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		writeError(w, http.StatusNotImplemented, "agent protocol binding is not implemented yet")
		return
	}

	bearerToken, ok := agentBearerToken(configuredAgent)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "agent credentials are not available")
		return
	}

	result, err := s.messages.SendMessage(r.Context(), a2aclient.SendMessageRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		Text:            req.Text,
		BearerToken:     bearerToken,
		ConversationID:  strings.TrimSpace(req.ConversationID),
		Extensions:      selected.Extensions,
		Metadata:        selected.Metadata,
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
		TaskID:         result.TaskID,
		AgentID:        agent.ID,
		Status:         result.Status,
		Message:        result.Text,
	})
}

type selectedAgentInterface struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	Streaming       bool
	Extensions      []string
	Metadata        map[string]any
}

func (s *Server) selectedAgentInterface(ctx context.Context, agent registry.Agent) selectedAgentInterface {
	selected := selectedAgentInterface{
		EndpointURL:     agent.EndpointURL,
		ProtocolBinding: agent.ProtocolBinding,
		ProtocolVersion: a2aclient.ProtocolVersion10,
	}
	cache, ok := s.currentAgentCardCache(ctx, agent)
	if ok && cache.SelectedEndpointURL != "" {
		selected.EndpointURL = cache.SelectedEndpointURL
		selected.ProtocolBinding = cache.SelectedProtocolBinding
		selected.ProtocolVersion = cache.SelectedProtocolVersion
		selected.Streaming = cache.Streaming
		if cache.DashboardContextSupported {
			selected.Extensions = []string{a2aclient.DashboardContextExtensionURI}
			selected.Metadata = map[string]any{
				a2aclient.DashboardContextExtensionURI: s.dashboardContext(ctx),
			}
		}
	}
	return selected
}

func (s *Server) agentsWithDiscovery(ctx context.Context, refreshMissing bool) []registry.Agent {
	agents := s.registry.List()
	for i := range agents {
		var cache store.AgentCardCache
		var ok bool
		if refreshMissing {
			cache, ok = s.currentAgentCardCache(ctx, agents[i])
		} else if s.cardStore != nil {
			cache, ok = s.loadAgentCardCache(ctx, agents[i].ID)
		}
		if ok {
			agents[i] = s.agentWithDiscovery(agents[i], cache)
		}
	}
	return agents
}

func (s *Server) agentWithDiscovery(agent registry.Agent, cache store.AgentCardCache) registry.Agent {
	agent.CardStatus = cache.CardStatus
	agent.CardFetchedAt = cache.FetchedAt
	agent.CardError = cache.CardError
	agent.SelectedEndpointURL = cache.SelectedEndpointURL
	agent.SelectedProtocolBinding = cache.SelectedProtocolBinding
	agent.SelectedProtocolVersion = cache.SelectedProtocolVersion
	agent.Skills = append([]a2aclient.AgentSkill(nil), cache.Skills...)
	agent.Streaming = cache.Streaming
	agent.DashboardContextSupported = cache.DashboardContextSupported
	if agent.SelectedEndpointURL != "" {
		agent.EndpointURL = agent.SelectedEndpointURL
	}
	if agent.SelectedProtocolBinding != "" {
		agent.ProtocolBinding = agent.SelectedProtocolBinding
	}
	return agent
}

func (s *Server) currentAgentCardCache(ctx context.Context, agent registry.Agent) (store.AgentCardCache, bool) {
	if s.cardStore == nil {
		return store.AgentCardCache{}, false
	}
	if cache, ok := s.loadAgentCardCache(ctx, agent.ID); ok && cache.CardStatus == "available" {
		return cache, true
	}
	return s.refreshAgentCard(ctx, agent), true
}

func (s *Server) loadAgentCardCache(ctx context.Context, agentID string) (store.AgentCardCache, bool) {
	if s.cardStore == nil {
		return store.AgentCardCache{}, false
	}
	cache, err := s.cardStore.AgentCardCache(ctx, agentID)
	if err != nil {
		return store.AgentCardCache{}, false
	}
	return cache, true
}

func (s *Server) refreshAgentCard(ctx context.Context, agent registry.Agent) store.AgentCardCache {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	cache := store.AgentCardCache{
		AgentID:                 agent.ID,
		CardStatus:              "unavailable",
		CardError:               "agent card is unavailable",
		SelectedEndpointURL:     agent.EndpointURL,
		SelectedProtocolBinding: agent.ProtocolBinding,
		SelectedProtocolVersion: a2aclient.ProtocolVersion10,
		FetchedAt:               now,
		ExpiresAt:               now,
	}
	configuredAgent, _ := s.configuredAgent(agent.ID)
	bearerToken, _ := agentBearerToken(configuredAgent)
	result, err := s.cardFetcher.Fetch(ctx, agent.CardURL, bearerToken)
	if err != nil {
		cache.CardError = "agent card could not be fetched"
		s.saveAgentCardCache(ctx, cache)
		return cache
	}
	selected, err := a2aclient.SelectInterface(result.Card, agent.EndpointURL, agent.ProtocolBinding)
	if err != nil {
		cache.CardJSON = result.Raw
		cache.CardError = "agent card has no compatible A2A 1.0 interface"
		cache.FetchedAt = result.FetchedAt.Format(time.RFC3339Nano)
		cache.ExpiresAt = result.FetchedAt.Add(10 * time.Minute).Format(time.RFC3339Nano)
		cache.Skills = result.Card.Skills
		cache.Streaming = result.Card.Capabilities.Streaming
		cache.DashboardContextSupported = a2aclient.SupportsDashboardContext(result.Card)
		s.saveAgentCardCache(ctx, cache)
		return cache
	}
	cache.CardJSON = result.Raw
	cache.CardStatus = "available"
	cache.CardError = ""
	cache.SelectedEndpointURL = selected.EndpointURL
	cache.SelectedProtocolBinding = selected.ProtocolBinding
	cache.SelectedProtocolVersion = selected.ProtocolVersion
	cache.Streaming = result.Card.Capabilities.Streaming
	cache.DashboardContextSupported = a2aclient.SupportsDashboardContext(result.Card)
	cache.Skills = result.Card.Skills
	cache.FetchedAt = result.FetchedAt.Format(time.RFC3339Nano)
	cache.ExpiresAt = result.FetchedAt.Add(10 * time.Minute).Format(time.RFC3339Nano)
	s.saveAgentCardCache(ctx, cache)
	return cache
}

func (s *Server) saveAgentCardCache(ctx context.Context, cache store.AgentCardCache) {
	if s.cardStore == nil {
		return
	}
	_ = s.cardStore.SaveAgentCardCache(ctx, cache)
}

func (s *Server) dashboardContext(ctx context.Context) map[string]any {
	layout, err := s.currentWidgetLayout(ctx, "")
	if err != nil {
		layout = s.layout
	}
	visibleIDs := []string{}
	widgets := []map[string]any{}
	for _, widget := range layout.Widgets {
		if !widget.Visible {
			continue
		}
		visibleIDs = append(visibleIDs, widget.ID)
		publicContext := map[string]any{}
		switch widget.Kind {
		case "weather":
			publicContext["locationName"] = s.cfg.Weather.LocationName
			publicContext["provider"] = s.cfg.Weather.Provider
		case "date-time":
			publicContext["timezone"] = s.cfg.Home.Timezone
			publicContext["locale"] = s.cfg.Home.Locale
		case "chat-history":
			publicContext["conversationHistoryVisible"] = true
		}
		widgets = append(widgets, map[string]any{
			"id":            widget.ID,
			"kind":          widget.Kind,
			"title":         widget.Title,
			"size":          widget.Size,
			"publicContext": publicContext,
		})
	}
	return map[string]any{
		"schema": a2aclient.DashboardContextExtensionURI,
		"display": map[string]any{
			"deviceId":        "default-display",
			"profile":         layout.ProfileID,
			"locale":          s.cfg.Home.Locale,
			"timezone":        s.cfg.Home.Timezone,
			"interactionMode": "touch",
		},
		"dashboard": map[string]any{
			"layoutId":         layout.ProfileID,
			"visibleWidgetIds": visibleIDs,
		},
		"widgets": widgets,
	}
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Last-Event-ID")
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
	writeMethodNotAllowed(w, method)
	return false
}

func writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
