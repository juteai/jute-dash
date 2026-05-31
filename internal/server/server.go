package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fmt"
	a2aclient "jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/displayactions"
	"jute-dash/internal/home"
	"jute-dash/internal/registry"
	"jute-dash/internal/store"
	"jute-dash/internal/weather"
	"jute-dash/widgets"
	_ "jute-dash/widgets/chathistory"
	_ "jute-dash/widgets/datetime"
	_ "jute-dash/widgets/markets"
	_ "jute-dash/widgets/rss"
	_ "jute-dash/widgets/weather"
)

type Server struct {
	cfg         config.Config
	registry    registry.Registry
	weather     weather.Provider
	messages    a2aclient.MessageSender
	cardFetcher *a2aclient.AgentCardFetcher
	setup       store.SetupStatus
	layout      store.WidgetLayout
	layoutStore WidgetLayoutStore
	settings    HouseholdSettingsStore
	voice       config.VoiceConfig
	voiceStore  VoiceSettingsStore
	configPath  string
	agentCards  map[string]agentCardCache
	display     *displayactions.Dispatcher
	mu          sync.Mutex
	started     time.Time
	version     string
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

type HouseholdSettingsStore interface {
	HouseholdSettings(ctx context.Context) (store.HouseholdSettings, error)
	SaveHouseholdSettings(ctx context.Context, settings store.HouseholdSettings) (store.HouseholdSettings, error)
	Rooms(ctx context.Context) ([]config.RoomConfig, error)
	SaveRooms(ctx context.Context, rooms []config.RoomConfig) ([]config.RoomConfig, error)
	Tiles(ctx context.Context) ([]config.TileConfig, error)
	SaveTiles(ctx context.Context, tiles []config.TileConfig) ([]config.TileConfig, error)
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
}

type StatusResponse struct {
	Status      string             `json:"status"`
	Version     string             `json:"version"`
	StartedAt   time.Time          `json:"startedAt"`
	Setup       store.SetupStatus  `json:"setup"`
	Config      ConfigStatus       `json:"config"`
	EventStream EventStreamStatus  `json:"eventStream"`
	MCP         MCPStatus          `json:"mcp"`
	Agents      AgentStatusSummary `json:"agents"`
	Voice       VoiceStatusSummary `json:"voice"`
}

type ConfigStatus struct {
	HasBootstrapConfig bool `json:"hasBootstrapConfig"`
	WritableYAML       bool `json:"writableYaml"`
}

type EventStreamStatus struct {
	Available bool `json:"available"`
}

type MCPStatus struct {
	Enabled       bool   `json:"enabled"`
	ServiceStatus string `json:"serviceStatus"`
	Transport     string `json:"transport"`
	ListenAddress string `json:"listenAddress"`
	Path          string `json:"path"`
	AuthMode      string `json:"authMode"`
	AllowLAN      bool   `json:"allowLan"`
}

type AgentStatusSummary struct {
	Total                     int `json:"total"`
	Enabled                   int `json:"enabled"`
	Disabled                  int `json:"disabled"`
	Available                 int `json:"available"`
	Unavailable               int `json:"unavailable"`
	DashboardContextSupported int `json:"dashboardContextSupported"`
	MCPScoped                 int `json:"mcpScoped"`
}

type VoiceStatusSummary struct {
	Enabled       bool   `json:"enabled"`
	ServiceStatus string `json:"serviceStatus"`
	State         string `json:"state"`
}

var errInvalidHouseholdSettings = errors.New("invalid household settings")

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
	return NewWithSetupStatusAndLayoutStoreAndConfigPath(cfg, version, setup, layoutStore, "")
}

func NewWithSetupStatusAndLayoutStoreAndConfigPath(cfg config.Config, version string, setup store.SetupStatus, layoutStore WidgetLayoutStore, configPath string) http.Handler {
	return NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(cfg, version, setup, layoutStore, configPath, nil)
}

func NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(cfg config.Config, version string, setup store.SetupStatus, layoutStore WidgetLayoutStore, configPath string, display *displayactions.Dispatcher) http.Handler {
	layout := store.DefaultWidgetLayout()
	if layoutStore != nil {
		if loaded, err := layoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	return newServer(cfg, version, weather.NewClient(), nil, setup, layout, layoutStore, configPath, display)
}

func newServer(cfg config.Config, version string, weatherProvider weather.Provider, messageSender a2aclient.MessageSender, setup store.SetupStatus, layout store.WidgetLayout, layoutStore WidgetLayoutStore, args ...any) http.Handler {
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
	var settingsStore HouseholdSettingsStore
	if candidate, ok := layoutStore.(HouseholdSettingsStore); ok {
		settingsStore = candidate
	}
	activeConfigPath := ""
	var display *displayactions.Dispatcher
	for _, arg := range args {
		switch value := arg.(type) {
		case string:
			activeConfigPath = value
		case *displayactions.Dispatcher:
			display = value
		}
	}
	if display == nil {
		display = displayactions.NewDispatcher()
	}
	server := &Server{
		cfg:         cfg,
		registry:    registry.New(cfg.Agents),
		weather:     weatherProvider,
		messages:    messageSender,
		cardFetcher: a2aclient.NewAgentCardFetcher(),
		setup:       normalizeSetupStatus(setup),
		layout:      normalizeWidgetLayout(layout),
		layoutStore: layoutStore,
		settings:    settingsStore,
		voice:       cfg.Voice,
		voiceStore:  voiceStore,
		configPath:  activeConfigPath,
		agentCards:  map[string]agentCardCache{},
		display:     display,
		started:     time.Now().UTC(),
		version:     version,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/status", server.handleStatus)
	mux.HandleFunc("/api/v1/config", server.handleConfig)
	mux.HandleFunc("/api/v1/home", server.handleHome)
	mux.HandleFunc("/api/v1/agents", server.handleAgents)
	mux.HandleFunc("/api/v1/agents/", server.handleAgentSubroutes)
	mux.HandleFunc("/api/v1/messages", server.handleMessages)
	mux.HandleFunc("/api/v1/conversations", server.handleConversations)
	mux.HandleFunc("/api/v1/conversations/", server.handleConversationSubroutes)
	mux.HandleFunc("/api/v1/events", server.handleEvents)
	mux.HandleFunc("/api/v1/setup/status", server.handleSetupStatus)
	mux.HandleFunc("/api/v1/settings/household", server.handleHouseholdSettings)
	mux.HandleFunc("/api/v1/settings/rooms", server.handleRoomSettings)
	mux.HandleFunc("/api/v1/settings/tiles", server.handleTileSettings)
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

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	voiceStatus, err := s.currentVoiceStatus(r.Context())
	if err != nil {
		voiceStatus = voiceStatusFromConfig(s.voice, time.Now().UTC())
	}
	status := StatusResponse{
		Status:      s.overallStatus(r.Context()),
		Version:     s.version,
		StartedAt:   s.started,
		Setup:       s.setup,
		Config:      s.configStatus(),
		EventStream: EventStreamStatus{Available: true},
		MCP:         mcpStatusFromConfig(s.cfg.MCP),
		Agents:      s.agentStatusSummary(r.Context()),
		Voice: VoiceStatusSummary{
			Enabled:       voiceStatus.Enabled,
			ServiceStatus: voiceStatus.ServiceStatus,
			State:         voiceStatus.State,
		},
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) overallStatus(ctx context.Context) string {
	if !s.setup.Complete {
		return "degraded"
	}
	mcpStatus := mcpStatusFromConfig(s.cfg.MCP)
	if mcpStatus.Enabled && mcpStatus.ServiceStatus != "enabled" {
		return "degraded"
	}
	summary := s.agentStatusSummary(ctx)
	if summary.Enabled > 0 && summary.Unavailable >= summary.Enabled {
		return "degraded"
	}
	return "ok"
}

func (s *Server) configStatus() ConfigStatus {
	ext := strings.ToLower(filepath.Ext(s.configPath))
	return ConfigStatus{
		HasBootstrapConfig: strings.TrimSpace(s.configPath) != "",
		WritableYAML:       ext == ".yaml" || ext == ".yml",
	}
}

func mcpStatusFromConfig(cfg config.MCPConfig) MCPStatus {
	status := MCPStatus{
		Enabled:       cfg.Enabled,
		ServiceStatus: "disabled",
		Transport:     cfg.Transport,
		ListenAddress: cfg.ListenAddress,
		Path:          cfg.Path,
		AuthMode:      cfg.Auth.Mode,
		AllowLAN:      cfg.AllowLAN,
	}
	if !cfg.Enabled {
		return status
	}
	if strings.TrimSpace(cfg.Transport) == "" || strings.TrimSpace(cfg.ListenAddress) == "" || strings.TrimSpace(cfg.Path) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	if strings.TrimSpace(cfg.Auth.Mode) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	status.ServiceStatus = "enabled"
	return status
}

func mergeHouseholdSettings(current, next store.HouseholdSettings) store.HouseholdSettings {
	if strings.TrimSpace(next.Home.Name) == "" {
		next.Home.Name = current.Home.Name
	}
	if strings.TrimSpace(next.Home.Timezone) == "" {
		next.Home.Timezone = current.Home.Timezone
	}
	if strings.TrimSpace(next.Home.Locale) == "" {
		next.Home.Locale = current.Home.Locale
	}
	if strings.TrimSpace(next.Display.Theme) == "" {
		next.Display.Theme = current.Display.Theme
	}
	if strings.TrimSpace(next.Display.AccentColor) == "" {
		next.Display.AccentColor = current.Display.AccentColor
	}
	if strings.TrimSpace(next.Display.IdleMode) == "" {
		next.Display.IdleMode = current.Display.IdleMode
	}
	if strings.TrimSpace(next.Weather.Provider) == "" {
		next.Weather.Provider = current.Weather.Provider
	}
	if strings.TrimSpace(next.Weather.LocationName) == "" {
		next.Weather.LocationName = current.Weather.LocationName
	}
	if strings.TrimSpace(next.Weather.TemperatureUnit) == "" {
		next.Weather.TemperatureUnit = current.Weather.TemperatureUnit
	}
	if strings.TrimSpace(next.Weather.WindSpeedUnit) == "" {
		next.Weather.WindSpeedUnit = current.Weather.WindSpeedUnit
	}
	next.Setup = current.Setup
	return next
}

func validateHouseholdSettings(settings store.HouseholdSettings) error {
	if strings.TrimSpace(settings.Home.Name) == "" {
		return fmt.Errorf("%w: home.name is required", errInvalidHouseholdSettings)
	}
	if _, err := time.LoadLocation(settings.Home.Timezone); err != nil {
		return fmt.Errorf("%w: home.timezone is invalid", errInvalidHouseholdSettings)
	}
	if strings.TrimSpace(settings.Home.Locale) == "" {
		return fmt.Errorf("%w: home.locale is required", errInvalidHouseholdSettings)
	}
	cfg := config.Default()
	cfg.Home = settings.Home
	cfg.Display = settings.Display
	cfg.Weather = settings.Weather
	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("%w: %v", errInvalidHouseholdSettings, err)
	}
	return nil
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
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"agents": s.agentsWithDiscovery(r.Context(), true),
		})
	case http.MethodPost:
		var req struct {
			CardURL string `json:"cardUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		agent, err := s.addAgentFromCard(r.Context(), req.CardURL)
		if err != nil {
			writeAgentConfigError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, agent)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (s *Server) handleAgentSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	agentID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPatch:
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON request body")
				return
			}
			agent, err := s.patchAgent(agentID, req.Enabled)
			if err != nil {
				writeAgentConfigError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, agent)
		case http.MethodDelete:
			if err := s.deleteAgent(agentID); err != nil {
				writeAgentConfigError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		default:
			writeMethodNotAllowed(w, http.MethodPatch+", "+http.MethodDelete)
		}
		return
	}
	if len(parts) != 2 || parts[1] != "refresh-card" {
		writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
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

func (s *Server) handleHouseholdSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := s.currentHouseholdSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "household settings are unavailable")
			return
		}
		writeJSON(w, http.StatusOK, settings)
	case http.MethodPatch:
		var settings store.HouseholdSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		saved, err := s.saveHouseholdSettings(r.Context(), settings)
		if err != nil {
			if errors.Is(err, errInvalidHouseholdSettings) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "household settings could not be saved")
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPatch)
	}
}

func (s *Server) handleRoomSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rooms, err := s.currentRooms(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "room settings are unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
	case http.MethodPut:
		var req struct {
			Rooms []config.RoomConfig `json:"rooms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		rooms, err := s.saveRooms(r.Context(), req.Rooms)
		if err != nil {
			if errors.Is(err, store.ErrInvalidSettings) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "room settings could not be saved")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (s *Server) handleTileSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tiles, err := s.currentTiles(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "tile settings are unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tiles": tiles})
	case http.MethodPut:
		var req struct {
			Tiles []config.TileConfig `json:"tiles"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		tiles, err := s.saveTiles(r.Context(), req.Tiles)
		if err != nil {
			if errors.Is(err, store.ErrInvalidSettings) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "tile settings could not be saved")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tiles": tiles})
	default:
		writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (s *Server) handleWidgetCatalog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	items := widgets.List()
	catalog := make([]widgets.WidgetCatalogItem, 0, len(items))
	for _, it := range items {
		catalog = append(catalog, it.CatalogInfo())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"widgets": catalog,
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

func (s *Server) currentConfig() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.configPath == "" {
		return s.cfg
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return s.cfg
	}
	cfg.Server = s.cfg.Server
	cfg.MCP = s.cfg.MCP
	s.cfg = cfg
	return s.cfg
}

func (s *Server) currentHouseholdSettings(ctx context.Context) (store.HouseholdSettings, error) {
	if s.configPath != "" {
		cfg := s.currentConfig()
		return store.HouseholdSettings{
			Home:    cfg.Home,
			Display: cfg.Display,
			Weather: cfg.Weather,
			Setup:   s.setup,
		}, nil
	}
	if s.settings != nil {
		return s.settings.HouseholdSettings(ctx)
	}
	return store.HouseholdSettings{
		Home:    s.cfg.Home,
		Display: s.cfg.Display,
		Weather: s.cfg.Weather,
		Setup:   s.setup,
	}, nil
}

func (s *Server) saveHouseholdSettings(ctx context.Context, settings store.HouseholdSettings) (store.HouseholdSettings, error) {
	current, err := s.currentHouseholdSettings(ctx)
	if err != nil {
		return store.HouseholdSettings{}, err
	}
	settings = mergeHouseholdSettings(current, settings)
	if err := validateHouseholdSettings(settings); err != nil {
		return store.HouseholdSettings{}, err
	}

	if s.configPath != "" {
		s.mu.Lock()
		next := s.cfg
		next.Home = settings.Home
		next.Display = settings.Display
		next.Weather = settings.Weather
		if err := config.SaveYAML(s.configPath, next); err != nil {
			s.mu.Unlock()
			return store.HouseholdSettings{}, err
		}
		s.cfg = next
		s.setup = store.SetupStatus{Complete: true, Missing: []string{}}
		s.mu.Unlock()
		return s.currentHouseholdSettings(ctx)
	}

	if s.settings != nil {
		saved, err := s.settings.SaveHouseholdSettings(ctx, settings)
		if err != nil {
			return store.HouseholdSettings{}, err
		}
		s.mu.Lock()
		s.cfg.Home = saved.Home
		s.cfg.Display = saved.Display
		s.cfg.Weather = saved.Weather
		s.setup = saved.Setup
		s.mu.Unlock()
		return saved, nil
	}

	s.mu.Lock()
	s.cfg.Home = settings.Home
	s.cfg.Display = settings.Display
	s.cfg.Weather = settings.Weather
	s.setup = store.SetupStatus{Complete: true, Missing: []string{}}
	s.mu.Unlock()
	return s.currentHouseholdSettings(ctx)
}

func (s *Server) currentRooms(ctx context.Context) ([]config.RoomConfig, error) {
	if s.configPath != "" {
		cfg := s.currentConfig()
		return cfg.Rooms, nil
	}
	if s.settings != nil {
		return s.settings.Rooms(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]config.RoomConfig(nil), s.cfg.Rooms...), nil
}

func (s *Server) saveRooms(ctx context.Context, rooms []config.RoomConfig) ([]config.RoomConfig, error) {
	if s.configPath != "" {
		normalized, err := store.NormalizeRooms(rooms)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		next := s.cfg
		next.Rooms = normalized
		if err := config.SaveYAML(s.configPath, next); err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.cfg = next
		s.mu.Unlock()
		return s.currentRooms(ctx)
	}

	if s.settings != nil {
		saved, err := s.settings.SaveRooms(ctx, rooms)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.cfg.Rooms = saved
		s.mu.Unlock()
		return saved, nil
	}

	normalized, err := store.NormalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cfg.Rooms = normalized
	s.mu.Unlock()
	return s.currentRooms(ctx)
}

func (s *Server) currentTiles(ctx context.Context) ([]config.TileConfig, error) {
	if s.configPath != "" {
		cfg := s.currentConfig()
		return cfg.Tiles, nil
	}
	if s.settings != nil {
		return s.settings.Tiles(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]config.TileConfig(nil), s.cfg.Tiles...), nil
}

func (s *Server) saveTiles(ctx context.Context, tiles []config.TileConfig) ([]config.TileConfig, error) {
	if s.configPath != "" {
		normalized, err := store.NormalizeTiles(tiles)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		next := s.cfg
		next.Tiles = normalized
		if err := config.SaveYAML(s.configPath, next); err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.cfg = next
		s.mu.Unlock()
		return s.currentTiles(ctx)
	}

	if s.settings != nil {
		saved, err := s.settings.SaveTiles(ctx, tiles)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.cfg.Tiles = saved
		s.mu.Unlock()
		return saved, nil
	}

	normalized, err := store.NormalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cfg.Tiles = normalized
	s.mu.Unlock()
	return s.currentTiles(ctx)
}

func (s *Server) currentWidgetLayout(ctx context.Context, profileID string) (store.WidgetLayout, error) {
	if s.configPath != "" {
		cfg := s.currentConfig()

		widgetInstances := make([]store.WidgetInstance, 0, len(cfg.Dashboard.Widgets))
		for _, w := range cfg.Dashboard.Widgets {
			instance := store.WidgetInstance{
				ID:       w.ID,
				Kind:     w.Type,
				Title:    w.Title,
				X:        w.X,
				Y:        w.Y,
				W:        w.W,
				H:        w.H,
				MinW:     1,
				MinH:     1,
				Size:     "medium",
				Visible:  w.Visible,
				Settings: w.Settings,
			}
			if provider, ok := widgets.Get(w.Type); ok {
				info := provider.CatalogInfo()
				instance.MinW = info.MinW
				instance.MinH = info.MinH
				instance.Size = info.DefaultSize

				data, err := provider.FetchData(ctx, w.Settings)
				if err == nil {
					instance.Data = data
				}
			}
			if instance.Settings == nil {
				instance.Settings = map[string]any{}
			}
			widgetInstances = append(widgetInstances, instance)
		}

		s.layout = store.WidgetLayout{
			ProfileID: "default",
			Widgets:   widgetInstances,
		}
		return normalizeWidgetLayout(s.layout), nil
	}

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
	if s.configPath != "" {
		s.mu.Lock()

		newWidgets := make([]config.DashboardWidgetConfig, 0, len(layout.Widgets))
		for _, w := range layout.Widgets {
			newWidgets = append(newWidgets, config.DashboardWidgetConfig{
				ID:       w.ID,
				Type:     w.Kind,
				Title:    w.Title,
				X:        w.X,
				Y:        w.Y,
				W:        w.W,
				H:        w.H,
				Visible:  w.Visible,
				Settings: w.Settings,
			})
		}
		s.cfg.Dashboard.Widgets = newWidgets

		err := config.SaveYAML(s.configPath, s.cfg)
		s.mu.Unlock()

		if err != nil {
			return store.WidgetLayout{}, fmt.Errorf("save config: %w", err)
		}

		return s.currentWidgetLayout(ctx, layout.ProfileID)
	}

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
	var status VoiceStatusResponse
	var err error
	if s.voiceStore != nil {
		var settings store.VoiceSettings
		settings, err = s.voiceStore.SetVoiceMuted(ctx, "", muted)
		if err == nil {
			status = voiceStatusFromSettings(settings)
		}
	} else {
		s.mu.Lock()
		s.voice.MutedByDefault = muted
		status = voiceStatusFromConfig(s.voice, time.Now().UTC())
		s.mu.Unlock()
	}
	if err != nil {
		return VoiceStatusResponse{}, err
	}

	s.display.EmitVoiceStateChanged("default-display", displayactions.VoiceStatePayload{
		Enabled:       status.Enabled,
		Muted:         status.Muted,
		State:         status.State,
		ServiceStatus: status.ServiceStatus,
	})

	return status, nil
}

func (s *Server) cancelVoice(ctx context.Context) (VoiceStatusResponse, error) {
	var status VoiceStatusResponse
	var err error
	if s.voiceStore != nil {
		var settings store.VoiceSettings
		settings, err = s.voiceStore.CancelVoice(ctx, "")
		if err == nil {
			status = voiceStatusFromSettings(settings)
		}
	} else {
		s.mu.Lock()
		status = voiceStatusFromConfig(s.voice, time.Now().UTC())
		s.mu.Unlock()
	}
	if err != nil {
		return VoiceStatusResponse{}, err
	}

	s.display.EmitVoiceStateChanged("default-display", displayactions.VoiceStatePayload{
		Enabled:       status.Enabled,
		Muted:         status.Muted,
		State:         status.State,
		ServiceStatus: status.ServiceStatus,
	})

	return status, nil
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

type agentCardCache struct {
	AgentID                   string
	CardJSON                  string
	CardStatus                string
	CardError                 string
	SelectedEndpointURL       string
	SelectedProtocolBinding   string
	SelectedProtocolVersion   string
	Streaming                 bool
	DashboardContextSupported bool
	Skills                    []a2aclient.AgentSkill
	FetchedAt                 string
	ExpiresAt                 string
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
	s.mu.Lock()
	agents := s.registry.List()
	s.mu.Unlock()
	for i := range agents {
		if configured, ok := s.configuredAgent(agents[i].ID); ok {
			agents[i].AuthConfigured = configured.Auth != nil
			agents[i].AuthAvailable = agentAuthAvailable(configured)
		}
		var cache agentCardCache
		var ok bool
		if refreshMissing {
			cache, ok = s.currentAgentCardCache(ctx, agents[i])
		} else {
			cache, ok = s.loadAgentCardCache(ctx, agents[i].ID)
		}
		if ok {
			agents[i] = s.agentWithDiscovery(agents[i], cache)
		}
	}
	return agents
}

func (s *Server) agentStatusSummary(ctx context.Context) AgentStatusSummary {
	agents := s.agentsWithDiscovery(ctx, false)
	summary := AgentStatusSummary{Total: len(agents)}
	for _, agent := range agents {
		if agent.Enabled {
			summary.Enabled++
		} else {
			summary.Disabled++
		}
		if agent.Enabled && agent.CardStatus == "available" && agentAuthAvailableFromPublic(agent) {
			summary.Available++
		}
		if agent.Enabled && agent.CardStatus != "" && agent.CardStatus != "available" {
			summary.Unavailable++
		}
		if agent.DashboardContextSupported {
			summary.DashboardContextSupported++
		}
		if len(agent.MCPScopes) > 0 {
			summary.MCPScoped++
		}
	}
	return summary
}

func (s *Server) agentWithDiscovery(agent registry.Agent, cache agentCardCache) registry.Agent {
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

func (s *Server) currentAgentCardCache(ctx context.Context, agent registry.Agent) (agentCardCache, bool) {
	if cache, ok := s.loadAgentCardCache(ctx, agent.ID); ok && cache.CardStatus == "available" {
		return cache, true
	}
	return s.refreshAgentCard(ctx, agent), true
}

func (s *Server) loadAgentCardCache(ctx context.Context, agentID string) (agentCardCache, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cache, ok := s.agentCards[agentID]
	return cache, ok
}

func (s *Server) refreshAgentCard(ctx context.Context, agent registry.Agent) agentCardCache {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	cache := agentCardCache{
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

func (s *Server) saveAgentCardCache(ctx context.Context, cache agentCardCache) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentCards[cache.AgentID] = cache
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
		if widget.Data != nil {
			publicContext["data"] = widget.Data
		}
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

func agentAuthAvailable(agent config.AgentConfig) bool {
	_, ok := agentBearerToken(agent)
	return ok
}

func agentAuthAvailableFromPublic(agent registry.Agent) bool {
	return !agent.AuthConfigured || agent.AuthAvailable
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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
