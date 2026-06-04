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
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/voice"
	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/internal/pkg/registry"
)

type Server struct {
	cfg         config.Config
	registry    registry.Registry
	messages    a2aclient.MessageSender
	agentCards  *agents.CardService
	setup       homestate.SetupStatus
	layout      dashboard.WidgetLayout
	layoutStore dashboard.LayoutStore
	settings    homestate.SettingsStore
	voice       voice.Config
	voiceStore  voice.Store
	configPath  string
	display     *displayactions.Dispatcher
	turnRunner  *agents.Runner
	mu          sync.Mutex
	started     time.Time
	version     string
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
		yamlStore := makeYAMLStore(configPath)
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

func makeYAMLStore(configPath string) *dashboard.YAMLRepository {
	return dashboard.NewYAMLRepository(
		configPath,
		func(path string) (dashboard.DashboardConfig, error) {
			cfg, err := config.LoadConfig(path)
			if err != nil {
				return dashboard.DashboardConfig{}, err
			}
			return cfg.Dashboard, nil
		},
		func(path string, dCfg dashboard.DashboardConfig) error {
			cfg, err := config.LoadConfig(path)
			if err != nil {
				return err
			}
			cfg.Dashboard = dCfg
			return config.SaveYAML(path, cfg)
		},
	)
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

	if configPath != "" {
		activeLayoutStore = makeYAMLStore(configPath)
		activeVoiceStore = voice.NewYAMLRepository(
			configPath,
			func(path string) (voice.Config, error) {
				c, err := config.LoadConfig(path)
				if err != nil {
					return voice.Config{}, err
				}
				return c.Voice, nil
			},
			func(path string, vCfg voice.Config) error {
				c, err := config.LoadConfig(path)
				if err != nil {
					return err
				}
				c.Voice = vCfg
				return config.SaveYAML(path, c)
			},
		)
		activeSettingsStore = homestate.NewYAMLRepository(
			configPath,
			func(path string) (homestate.HomeConfig, any, homestate.WeatherConfig, []homestate.RoomConfig, []homestate.TileConfig, error) {
				c, err := config.LoadConfig(path)
				if err != nil {
					return homestate.HomeConfig{}, nil, homestate.WeatherConfig{}, nil, nil, err
				}
				return c.Home, c.Display, c.Weather, c.Rooms, c.Tiles, nil
			},
			func(path string, home homestate.HomeConfig, display any, weather homestate.WeatherConfig, rooms []homestate.RoomConfig, tiles []homestate.TileConfig) error {
				c, err := config.LoadConfig(path)
				if err != nil {
					return err
				}
				c.Home = home
				if display != nil {
					var disp dashboard.DisplayConfig
					dispBytes, err := json.Marshal(display)
					if err == nil {
						_ = json.Unmarshal(dispBytes, &disp)
						c.Display = disp
					}
				}
				c.Weather = weather
				c.Rooms = rooms
				c.Tiles = tiles
				return config.SaveYAML(path, c)
			},
		)
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

	regConfigs := make([]registry.AgentConfig, len(cfg.Agents))
	for i, a := range cfg.Agents {
		regConfigs[i] = registry.AgentConfig{
			ID:              a.ID,
			Name:            a.Name,
			Description:     a.Description,
			CardURL:         a.CardURL,
			EndpointURL:     a.EndpointURL,
			ProtocolBinding: a.ProtocolBinding,
			Enabled:         a.Enabled,
			Capabilities:    a.Capabilities,
			MCPScopes:       a.MCPScopes,
			AuthConfigured:  a.Auth != nil,
		}
	}

	server := &Server{
		cfg:         cfg,
		registry:    registry.New(regConfigs),
		messages:    messageSender,
		agentCards:  agents.NewCardService(),
		setup:       setup,
		layout:      layout,
		layoutStore: activeLayoutStore,
		settings:    activeSettingsStore,
		voice:       cfg.Voice,
		voiceStore:  activeVoiceStore,
		configPath:  configPath,
		display:     display,
		started:     time.Now().UTC(),
		version:     version,
	}

	agents.SetEnvReader(os.Getenv)
	server.turnRunner = agents.NewRunner(agents.RunnerOptions{
		Registry:       server.registry,
		GetAgentConfig: server.configuredAgent,
		GetAgentCardCache: func(ctx context.Context, agent registry.Agent) (agents.AgentCardCache, bool) {
			cache, ok := server.currentAgentCardCache(ctx, agent)
			if !ok {
				return agents.AgentCardCache{}, false
			}
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
		st.SetCatalog(dashboard.WidgetCatalog())
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
		},
	).RegisterRoutes(mux)

	voice.NewController(
		server.voiceStore,
		server.display,
	).RegisterRoutes(mux)

	var getAgentsConfig func() []agents.AgentConfig
	var saveAgentsConfig func([]agents.AgentConfig) error
	if configPath != "" {
		getAgentsConfig = func() []agents.AgentConfig {
			c, err := config.LoadConfig(configPath)
			if err != nil {
				return nil
			}
			return c.Agents
		}
		saveAgentsConfig = func(next []agents.AgentConfig) error {
			c, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			c.Agents = next
			return config.SaveYAML(configPath, c)
		}
	} else {
		var memAgentsMu sync.Mutex
		memAgents := cfg.Agents
		getAgentsConfig = func() []agents.AgentConfig {
			memAgentsMu.Lock()
			defer memAgentsMu.Unlock()
			return memAgents
		}
		saveAgentsConfig = func(next []agents.AgentConfig) error {
			memAgentsMu.Lock()
			memAgents = next
			memAgentsMu.Unlock()
			return nil
		}
	}

	agents.NewController(agents.ControllerOptions{
		Registry:            server.registry,
		CardService:         server.agentCards,
		Messages:            server.messages,
		TurnRunner:          server.turnRunner,
		ConfigPath:          configPath,
		GetAgentsConfig:     getAgentsConfig,
		SaveAgentsConfig:    saveAgentsConfig,
		GetDashboardContext: server.dashboardContext,
		OnRegistryUpdated: func(r registry.Registry) {
			server.mu.Lock()
			server.registry = r
			server.mu.Unlock()
		},
	}).RegisterRoutes(mux)

	// SSE broker mount
	broker := events.NewBroker(server.display)
	mux.Handle("/api/v1/events", broker)

	return withCommonHeaders(mux)
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
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.Public())
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
	agentsList := s.agentsWithDiscovery(ctx, false)
	summary := agents.AgentStatusSummary{Total: len(agentsList)}
	for _, agent := range agentsList {
		if agent.Enabled {
			summary.Enabled++
		} else {
			summary.Disabled++
		}
		if agent.Enabled && agent.CardStatus == "available" && s.agentAuthAvailableFromPublic(agent) {
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

func (s *Server) agentAuthAvailableFromPublic(agent registry.Agent) bool {
	configured, ok := s.configuredAgent(agent.ID)
	if !ok {
		return false
	}
	return s.agentAuthAvailable(configured)
}

func (s *Server) agentAuthAvailable(agent agents.AgentConfig) bool {
	if agent.Auth == nil {
		return true
	}
	return strings.TrimSpace(os.Getenv(agent.Auth.EnvToken)) != ""
}

func (s *Server) configuredAgent(agentID string) (agents.AgentConfig, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.cfg.Agents {
		if a.ID == agentID {
			return a, true
		}
	}
	return agents.AgentConfig{}, false
}

func (s *Server) agentsWithDiscovery(ctx context.Context, refreshMissing bool) []registry.Agent {
	s.mu.Lock()
	agentsList := s.registry.List()
	s.mu.Unlock()
	for i := range agentsList {
		var cache agents.AgentCardCache
		var ok bool
		if refreshMissing {
			cache, ok = s.currentAgentCardCache(ctx, agentsList[i])
		} else {
			cache, ok = s.loadAgentCardCache(ctx, agentsList[i].ID)
		}
		if ok {
			agentsList[i] = s.agentWithDiscovery(agentsList[i], cache)
		}
	}
	return agentsList
}

func (s *Server) agentWithDiscovery(agent registry.Agent, cache agents.AgentCardCache) registry.Agent {
	agent.CardStatus = cache.SelectedProtocolBinding
	agent.CardFetchedAt = ""
	agent.CardError = ""
	agent.SelectedEndpointURL = cache.SelectedEndpointURL
	agent.SelectedProtocolBinding = cache.SelectedProtocolBinding
	agent.SelectedProtocolVersion = ""
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

func (s *Server) currentAgentCardCache(ctx context.Context, agent registry.Agent) (agents.AgentCardCache, bool) {
	configured, _ := s.configuredAgent(agent.ID)
	res := s.agentCards.Current(ctx, agent, configured)
	return agents.AgentCardCache{
		SelectedEndpointURL:       res.SelectedEndpointURL,
		SelectedProtocolBinding:   res.SelectedProtocolBinding,
		SelectedProtocolVersion:   res.SelectedProtocolVersion,
		Streaming:                 res.Streaming,
		DashboardContextSupported: res.DashboardContextSupported,
	}, true
}

func (s *Server) loadAgentCardCache(_ context.Context, agentID string) (agents.AgentCardCache, bool) {
	res, ok := s.agentCards.Load(agentID)
	if ok {
		return agents.AgentCardCache{
			SelectedEndpointURL:       res.SelectedEndpointURL,
			SelectedProtocolBinding:   res.SelectedProtocolBinding,
			SelectedProtocolVersion:   res.SelectedProtocolVersion,
			Streaming:                 res.Streaming,
			DashboardContextSupported: res.DashboardContextSupported,
		}, true
	}
	return agents.AgentCardCache{}, false
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

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "jute-dash-hub")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}
