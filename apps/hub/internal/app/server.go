package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/events"
	"jute-dash/apps/hub/internal/app/filesync"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/voice"
	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/internal/pkg/httphelper"
	"jute-dash/apps/hub/internal/pkg/registry"
)

type Server struct {
	cfg           config.Config
	agentsManager *agents.AgentManager
	messages      a2aclient.MessageSender
	setup         homestate.SetupStatus
	layout        dashboard.WidgetLayout
	layoutStore   dashboard.LayoutStore
	settings      homestate.SettingsStore
	voice         voice.Config
	voiceStore    voice.Store
	configPath    string
	syncer        filesync.Syncer
	display       *displayactions.Dispatcher
	turnRunner    *agents.Runner
	mu            sync.Mutex
	started       time.Time
	version       string
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
}

type StatusResponse struct {
	Status      string                    `json:"status"`
	Version     string                    `json:"version"`
	StartedAt   time.Time                 `json:"startedAt"`
	Setup       homestate.SetupStatus     `json:"setup"`
	Config      ConfigStatus              `json:"config"`
	EventStream EventStreamStatus         `json:"eventStream"`
	MCP         mcpStatus                 `json:"mcp"`
	Agents      agents.AgentStatusSummary `json:"agents"`
	Voice       VoiceStatusSummary        `json:"voice"`
}

type ConfigStatus struct {
	HasBootstrapConfig bool `json:"hasBootstrapConfig"`
	WritableYAML       bool `json:"writableYaml"`
}

type EventStreamStatus struct {
	Available bool `json:"available"`
}

type mcpStatus struct {
	Enabled       bool   `json:"enabled"`
	ServiceStatus string `json:"serviceStatus"`
	Transport     string `json:"transport"`
	ListenAddress string `json:"listenAddress"`
	Path          string `json:"path"`
	AuthMode      string `json:"authMode"`
	AllowLAN      bool   `json:"allowLAN"`
}

type VoiceStatusSummary struct {
	Enabled       bool   `json:"enabled"`
	ServiceStatus string `json:"serviceStatus"`
	State         string `json:"state"`
}

func New(cfg config.Config, version string) http.Handler {
	layout := dashboard.DefaultWidgetLayout()
	return newServer(cfg, version, nil, homestate.SetupStatus{Complete: true}, layout, nil, "", nil)
}

func NewWithSetupStatus(
	cfg config.Config,
	version string,
	setup homestate.SetupStatus,
) http.Handler {
	return NewWithSetupStatusAndLayout(cfg, version, setup, dashboard.DefaultWidgetLayout())
}

func NewWithSetupStatusAndLayout(
	cfg config.Config,
	version string,
	setup homestate.SetupStatus,
	layout dashboard.WidgetLayout,
) http.Handler {
	return newServer(cfg, version, nil, setup, layout, nil, "", nil)
}

func NewWithMessageSender(
	cfg config.Config,
	version string,
	messageSender a2aclient.MessageSender,
) http.Handler {
	return newServer(
		cfg,
		version,
		messageSender,
		homestate.SetupStatus{Complete: true},
		dashboard.DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)
}

func NewWithSetupStatusAndLayoutStore(
	cfg config.Config,
	version string,
	setup homestate.SetupStatus,
	layoutStore dashboard.LayoutStore,
) http.Handler {
	return NewWithSetupStatusAndLayoutStoreAndConfigPath(cfg, version, setup, layoutStore, "")
}

func NewWithSetupStatusAndLayoutStoreAndConfigPath(
	cfg config.Config,
	version string,
	setup homestate.SetupStatus,
	layoutStore dashboard.LayoutStore,
	configPath string,
) http.Handler {
	return NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(
		cfg,
		version,
		setup,
		layoutStore,
		configPath,
		nil,
	)
}

func NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(
	cfg config.Config,
	version string,
	setup homestate.SetupStatus,
	layoutStore dashboard.LayoutStore,
	configPath string,
	display *displayactions.Dispatcher,
) http.Handler {
	layout := dashboard.DefaultWidgetLayout()
	if configPath != "" {
		var dbStore filesync.ConfigStore
		if candidate, ok := layoutStore.(filesync.ConfigStore); ok {
			dbStore = candidate
		}
		syncer := filesync.NewFileSyncer(configPath, dbStore)
		yamlStore := dashboard.NewYAMLRepository(syncer)
		if catalog := dashboard.RegisteredCatalog(); len(catalog) > 0 {
			yamlStore.SetCatalog(catalog)
		}
		if loaded, err := yamlStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	} else if layoutStore != nil {
		if loaded, err := layoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	return newServer(cfg, version, nil, setup, layout, layoutStore, configPath, display)
}

func newServer(
	cfg config.Config,
	version string,
	messageSender a2aclient.MessageSender,
	setup homestate.SetupStatus,
	layout dashboard.WidgetLayout,
	layoutStore dashboard.LayoutStore,
	configPath string,
	display *displayactions.Dispatcher,
) http.Handler {
	if messageSender == nil {
		messageSender = a2aclient.NewJSONRPCClient()
	}

	var activeLayoutStore dashboard.LayoutStore
	var activeVoiceStore voice.Store
	var activeSettingsStore homestate.SettingsStore

	var dbStore filesync.ConfigStore
	if candidate, ok := layoutStore.(filesync.ConfigStore); ok {
		dbStore = candidate
	}

	var syncer filesync.Syncer
	if configPath != "" {
		syncer = filesync.NewFileSyncer(configPath, dbStore)
	} else {
		syncer = filesync.NewInMemorySyncer(cfg)
	}

	if configPath != "" {
		activeLayoutStore = dashboard.NewYAMLRepository(syncer)
		activeVoiceStore = voice.NewYAMLRepository(syncer)
		activeSettingsStore = homestate.NewYAMLRepository(syncer)
	} else if layoutStore != nil {
		activeLayoutStore = layoutStore
		if candidate, ok := layoutStore.(voice.Store); ok {
			activeVoiceStore = candidate
		}
		if candidate, ok := layoutStore.(homestate.SettingsStore); ok {
			activeSettingsStore = candidate
		}
	}

	// Fallbacks
	if activeLayoutStore == nil {
		activeLayoutStore = dashboard.NewMemoryRepositoryWithLayout(layout)
	}
	if activeVoiceStore == nil {
		activeVoiceStore = voice.NewMemoryRepositoryFromConfig(cfg.Voice)
	}
	if activeSettingsStore == nil {
		activeSettingsStore = homestate.NewMemoryRepository(setup)
	}

	if display == nil {
		display = displayactions.NewDispatcher()
	}

	agentCards := agents.NewCardService(cfg.A2A)
	agentsManager := agents.NewAgentManager(syncer, agentCards, configPath)

	server := &Server{
		cfg:           cfg,
		agentsManager: agentsManager,
		messages:      messageSender,
		setup:         setup,
		layout:        layout,
		layoutStore:   activeLayoutStore,
		settings:      activeSettingsStore,
		voice:         cfg.Voice,
		voiceStore:    activeVoiceStore,
		configPath:    configPath,
		syncer:        syncer,
		display:       display,
		started:       time.Now().UTC(),
		version:       version,
	}

	agents.SetEnvReader(os.Getenv)
	server.turnRunner = agents.NewRunner(agents.RunnerOptions{
		GetRegistry: server.agentsManager.ActiveRegistry,
		GetAgentConfig: func(agentID string) (agents.AgentConfig, bool) {
			return server.agentsManager.ConfiguredAgent(agentID)
		},
		GetAgentCardCache: func(ctx context.Context, agent registry.Agent) (agents.AgentCardCache, bool) {
			configured, _ := server.agentsManager.ConfiguredAgent(agent.ID)
			cache := agentCards.Current(ctx, agent, configured)
			return agents.AgentCardCache{
				SelectedEndpointURL:       cache.SelectedEndpointURL,
				SelectedProtocolBinding:   cache.SelectedProtocolBinding,
				SelectedProtocolVersion:   cache.SelectedProtocolVersion,
				Streaming:                 cache.Streaming,
				DashboardContextSupported: cache.DashboardContextSupported,
			}, true
		},
		GetDashboardContext: func(ctx context.Context) map[string]any {
			return server.dashboardContext(ctx)
		},
		Messages: server.messages,
	})

	if st, ok := activeLayoutStore.(interface {
		SetCatalog([]dashboard.WidgetCatalogItem)
	}); ok {
		st.SetCatalog(dashboard.RegisteredCatalog())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/status", server.handleStatus)
	mux.HandleFunc("/api/v1/config", server.handleConfig)

	// Registrations
	homestate.NewController(
		server.settings,
		func(saved homestate.HouseholdSettings) {
			server.mu.Lock()
			server.setup = saved.Setup
			server.mu.Unlock()
			_ = syncer.Sync(context.Background())
		},
		nil,
		nil,
	).RegisterRoutes(mux)

	dashboard.NewController(
		server.layoutStore,
		func(saved dashboard.WidgetLayout) {
			server.mu.Lock()
			server.layout = saved
			server.mu.Unlock()
			_ = syncer.Sync(context.Background())
		},
	).RegisterRoutes(mux)

	dashboard.NewBackgroundsController(backgroundsDir).RegisterRoutes(mux)

	voice.NewController(
		server.voiceStore,
		server.display,
	).RegisterRoutes(mux)

	agents.NewController(agents.ControllerOptions{
		Manager:             server.agentsManager,
		Messages:            server.messages,
		TurnRunner:          server.turnRunner,
		GetDashboardContext: server.dashboardContext,
	}).RegisterRoutes(mux)

	// SSE broker mount
	broker := events.NewBroker(server.display)
	mux.Handle("/api/v1/events", broker)

	return withCommonHeaders(withCORS(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodGet) {
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Version:   s.version,
		StartedAt: s.started,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodGet) {
		return
	}
	voiceStatus, err := s.currentVoiceStatus(r.Context())
	if err != nil {
		voiceStatus = voice.StatusFromConfig(s.voice, time.Now().UTC())
	}
	status := StatusResponse{
		Status:      s.overallStatus(r.Context()),
		Version:     s.version,
		StartedAt:   s.started,
		Setup:       s.setup,
		Config:      s.configStatus(),
		EventStream: EventStreamStatus{Available: true},
		MCP:         s.mcpStatus(),
		Agents:      s.agentStatusSummary(r.Context()),
		Voice: VoiceStatusSummary{
			Enabled:       voiceStatus.Enabled,
			ServiceStatus: voiceStatus.ServiceStatus,
			State:         voiceStatus.State,
		},
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !httphelper.RequireMethod(w, r, http.MethodGet) {
		return
	}

	ctx := r.Context()

	// 1. Get Home and Display from settings store
	serverSettings, err := s.settings.HouseholdSettings(ctx)
	var home homestate.HomeConfig
	var display any
	if err == nil {
		home = serverSettings.Home
		display = serverSettings.Display
	} else {
		home = s.cfg.Home
		display = s.cfg.Display
	}

	// 2. Get Rooms and Tiles from settings store
	rooms, err := s.settings.Rooms(ctx)
	if err != nil {
		rooms = s.cfg.Rooms
	}
	tiles, err := s.settings.Tiles(ctx)
	if err != nil {
		tiles = s.cfg.Tiles
	}

	// Convert display to dashboard.DisplayConfig safely
	var disp dashboard.DisplayConfig
	if display != nil {
		dispBytes, err := json.Marshal(display)
		if err == nil {
			_ = json.Unmarshal(dispBytes, &disp)
		}
	} else {
		disp = s.cfg.Display
	}
	// 3. Get Dashboard widgets from layout store
	var dbConfig dashboard.DashboardConfig
	layout, err := s.layoutStore.WidgetLayout(ctx, "")
	if err == nil {
		widgets := make([]dashboard.DashboardWidgetConfig, 0, len(layout.Widgets))
		for _, w := range layout.Widgets {
			widgets = append(widgets, dashboard.DashboardWidgetConfig{
				ID:       w.ID,
				Type:     w.Kind,
				Title:    w.Title,
				X:        w.X,
				Y:        w.Y,
				W:        w.W,
				H:        w.H,
				MinW:     w.MinW,
				MinH:     w.MinH,
				Size:     w.Size,
				Visible:  w.Visible,
				Mode:     w.Mode,
				Settings: w.Settings,
			})
		}
		dbConfig.Widgets = widgets
	} else {
		dbConfig = s.cfg.Dashboard
	}

	// 4. Get active agents list and map to agents.PublicAgentConfig
	regAgents := s.agentsManager.List(ctx, false)
	publicAgents := make([]agents.PublicAgentConfig, 0, len(regAgents))
	for _, a := range regAgents {
		publicAgents = append(publicAgents, agents.PublicAgentConfig{
			ID:              a.ID,
			Name:            a.Name,
			Description:     a.Description,
			CardURL:         a.CardURL,
			EndpointURL:     a.EndpointURL,
			ProtocolBinding: a.ProtocolBinding,
			Enabled:         a.Enabled,
			Capabilities:    append([]string(nil), a.Capabilities...),
			MCPScopes:       append([]string(nil), a.MCPScopes...),
			AuthConfigured:  a.AuthConfigured,
			AuthAvailable:   a.AuthAvailable,
		})
	}

	httphelper.WriteJSON(w, http.StatusOK, config.PublicConfig{
		Home:      home,
		Display:   disp,
		Dashboard: dbConfig,
		Agents:    publicAgents,
		Rooms:     rooms,
		Tiles:     tiles,
	})
}

func (s *Server) overallStatus(ctx context.Context) string {
	if !s.setup.Complete {
		return "degraded"
	}
	mcpStatus := s.mcpStatus()
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

func (s *Server) mcpStatus() mcpStatus {
	mCfg := s.cfg.MCP
	status := mcpStatus{
		Enabled:       mCfg.Enabled,
		ServiceStatus: "disabled",
		Transport:     mCfg.Transport,
		ListenAddress: mCfg.ListenAddress,
		Path:          mCfg.Path,
		AuthMode:      mCfg.Auth.Mode,
		AllowLAN:      mCfg.AllowLAN,
	}
	if !mCfg.Enabled {
		return status
	}
	if strings.TrimSpace(mCfg.Transport) == "" || strings.TrimSpace(mCfg.ListenAddress) == "" ||
		strings.TrimSpace(mCfg.Path) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	if strings.TrimSpace(mCfg.Auth.Mode) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	status.ServiceStatus = "enabled"
	return status
}

func (s *Server) currentVoiceStatus(ctx context.Context) (voice.StatusResponse, error) {
	settings, err := s.voiceStore.VoiceSettings(ctx, "")
	if err != nil {
		return voice.StatusResponse{}, err
	}
	return voice.StatusFromSettings(settings), nil
}

func (s *Server) agentStatusSummary(ctx context.Context) agents.AgentStatusSummary {
	return s.agentsManager.StatusSummary(ctx)
}

func (s *Server) dashboardContext(ctx context.Context) map[string]any {
	serverSettings, err := s.settings.HouseholdSettings(ctx)
	if err != nil {
		return map[string]any{}
	}
	home := homestate.HomeConfig{
		Name:     serverSettings.Home.Name,
		Timezone: serverSettings.Home.Timezone,
		Locale:   serverSettings.Home.Locale,
	}

	rooms, err := s.settings.Rooms(ctx)
	if err != nil {
		rooms = []homestate.RoomConfig{}
	}
	tiles, err := s.settings.Tiles(ctx)
	if err != nil {
		tiles = []homestate.TileConfig{}
	}

	state := homestate.FromConfig(home, rooms, tiles, time.Now())

	layout, err := s.layoutStore.WidgetLayout(ctx, "")
	if err != nil {
		layout = dashboard.WidgetLayout{}
	}

	snapshot := dashboard.Project(ctx, layout, home.Locale, home.Timezone)
	return map[string]any{
		"home":      state,
		"dashboard": snapshot,
	}
}

// Helpers

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "jute-dash-hub")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isLocalOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "http://127.0.0.1") ||
		strings.HasPrefix(origin, "https://localhost") ||
		strings.HasPrefix(origin, "https://127.0.0.1")
}
