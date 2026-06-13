package app

import (
	"context"
	"encoding/json"
	"log/slog"
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
	"jute-dash/apps/hub/internal/app/widgetactions"
	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/internal/pkg/httphelper"
	"jute-dash/apps/hub/internal/pkg/registry"
	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

type Server struct {
	cfg                config.Config
	agentsManager      *agents.AgentManager
	messages           a2aclient.MessageSender
	setup              homestate.SetupStatus
	layout             dashboard.WidgetLayout
	layoutStore        dashboard.LayoutStore
	settings           homestate.SettingsStore
	voice              voice.Config
	voiceStore         voice.Store
	configPath         string
	syncer             filesync.Syncer
	display            *displayactions.Dispatcher
	voiceDispatcher    *voice.Dispatcher
	connectionResolver widgets.ConnectionResolver
	secretStore        secretStore
	actionDispatcher   *widgetactions.Dispatcher
	spotifyOAuth       *spotifyOAuthController
	turnRunner         *agents.Runner
	handler            http.Handler
	mu                 sync.Mutex
	started            time.Time
	version            string
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
	return newServer(
		cfg,
		version,
		nil,
		homestate.SetupStatus{Complete: true},
		layout,
		nil,
		nil,
		nil,
		"",
		nil,
		nil,
	)
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
	return newServer(cfg, version, nil, setup, layout, nil, nil, nil, "", nil, nil)
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
		nil,
		nil,
		"",
		nil,
		nil,
	)
}

func NewServer(
	cfg config.Config,
	version string,
	setup homestate.SetupStatus,
	layoutStore dashboard.LayoutStore,
	settingsStore homestate.SettingsStore,
	voiceStore voice.Store,
	configPath string,
	display *displayactions.Dispatcher,
) *Server {
	layout := dashboard.DefaultWidgetLayout()
	if layoutStore != nil {
		if loaded, err := layoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	return newServer(
		cfg,
		version,
		nil,
		setup,
		layout,
		layoutStore,
		settingsStore,
		voiceStore,
		configPath,
		display,
		nil,
	)
}

func NewServerWithSecrets(
	cfg config.Config,
	version string,
	setup homestate.SetupStatus,
	layoutStore dashboard.LayoutStore,
	settingsStore homestate.SettingsStore,
	voiceStore voice.Store,
	configPath string,
	display *displayactions.Dispatcher,
	secrets secretStore,
) *Server {
	layout := dashboard.DefaultWidgetLayout()
	if layoutStore != nil {
		if loaded, err := layoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	return newServer(
		cfg,
		version,
		nil,
		setup,
		layout,
		layoutStore,
		settingsStore,
		voiceStore,
		configPath,
		display,
		secrets,
	)
}

func newServer(
	cfg config.Config,
	version string,
	messageSender a2aclient.MessageSender,
	setup homestate.SetupStatus,
	layout dashboard.WidgetLayout,
	layoutStore dashboard.LayoutStore,
	settingsStore homestate.SettingsStore,
	voiceStore voice.Store,
	configPath string,
	display *displayactions.Dispatcher,
	secretStore secretStore,
) *Server {
	if messageSender == nil {
		messageSender = a2aclient.NewJSONRPCClient()
	}

	activeLayoutStore := layoutStore
	if activeLayoutStore == nil {
		activeLayoutStore = dashboard.NewMemoryRepositoryWithLayout(layout)
	}

	activeVoiceStore := voiceStore
	if activeVoiceStore == nil {
		activeVoiceStore = voice.NewMemoryRepositoryFromConfig(cfg.Voice)
	}

	activeSettingsStore := settingsStore
	if activeSettingsStore == nil {
		activeSettingsStore = homestate.NewMemoryRepository(setup)
	}

	var dbStore filesync.ConfigStore
	if candidate, ok := activeLayoutStore.(filesync.ConfigStore); ok {
		dbStore = candidate
	} else {
		type configStoreProvider interface {
			ConfigStore() any
		}
		if provider, ok := activeLayoutStore.(configStoreProvider); ok {
			if cs, ok := provider.ConfigStore().(filesync.ConfigStore); ok {
				dbStore = cs
			}
		}
	}

	var syncer filesync.Syncer
	if configPath != "" {
		syncer = filesync.NewFileSyncer(configPath, dbStore)
	} else {
		syncer = filesync.NewInMemorySyncer(cfg)
	}

	// Sync on Load synchronously from YAML to SQLite
	if configPath != "" {
		_ = syncOnLoad(context.Background(), syncer, activeLayoutStore, activeSettingsStore, activeVoiceStore)
	}

	// Reload layout and setup status from active database stores
	if activeLayoutStore != nil {
		if loaded, err := activeLayoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	if activeSettingsStore != nil {
		if status, err := activeSettingsStore.SetupStatus(context.Background()); err == nil {
			setup = status
		}
	}

	// Start River background queue if activeLayoutStore implements it
	if qs, ok := activeLayoutStore.(interface {
		StartQueue(syncer filesync.Syncer) error
	}); ok {
		_ = qs.StartQueue(syncer)
	} else {
		type configStoreProvider interface {
			ConfigStore() any
		}
		if provider, ok := activeLayoutStore.(configStoreProvider); ok {
			type queueStarter interface {
				StartQueue(syncer filesync.Syncer) error
			}
			if qs, ok := provider.ConfigStore().(queueStarter); ok {
				_ = qs.StartQueue(syncer)
			}
		}
	}

	if display == nil {
		display = displayactions.NewDispatcher()
	}

	agentCards := agents.NewCardService(cfg.A2A)
	agentsManager := agents.NewAgentManager(syncer, agentCards, configPath)

	voiceDispatcher := voice.NewDispatcher()
	connectionResolver := newConnectionResolver(activeSettingsStore, secretStore)

	server := &Server{
		cfg:                cfg,
		agentsManager:      agentsManager,
		messages:           messageSender,
		setup:              setup,
		layout:             layout,
		layoutStore:        activeLayoutStore,
		settings:           activeSettingsStore,
		voice:              cfg.Voice,
		voiceStore:         activeVoiceStore,
		configPath:         configPath,
		syncer:             syncer,
		display:            display,
		voiceDispatcher:    voiceDispatcher,
		connectionResolver: connectionResolver,
		secretStore:        secretStore,
		started:            time.Now().UTC(),
		version:            version,
	}
	actionDispatcher := widgetactions.NewDispatcher(
		activeLayoutStore,
		connectionResolver,
		server.buildWidgetSkillsSnapshot,
	)
	server.actionDispatcher = actionDispatcher
	server.spotifyOAuth = newSpotifyOAuthController(server)

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
	mux.HandleFunc("/api/v1/integrations/spotify/auth", server.spotifyOAuth.handleAuth)
	mux.HandleFunc("/api/v1/integrations/spotify/callback", server.spotifyOAuth.handleCallback)
	mux.HandleFunc("/api/v1/integrations/spotify/web-playback-token", server.spotifyOAuth.handleWebPlaybackToken)
	mux.HandleFunc("/api/v1/widgets/", server.handleWidgetAction)

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

	dashboard.NewControllerWithHydrator(
		server.layoutStore,
		dashboard.NewHydrator(server.connectionResolver),
		func(saved dashboard.WidgetLayout) {
			server.mu.Lock()
			server.layout = saved
			server.mu.Unlock()
		},
	).RegisterRoutes(mux)

	dashboard.NewBackgroundsController(backgroundsDir).RegisterRoutes(mux)

	voice.NewController(
		server.voiceStore,
		server.voiceDispatcher,
	).RegisterRoutes(mux)

	agents.NewController(agents.ControllerOptions{
		Manager:             server.agentsManager,
		Messages:            server.messages,
		TurnRunner:          server.turnRunner,
		GetDashboardContext: server.dashboardContext,
	}).RegisterRoutes(mux)

	// SSE broker mount
	broker := events.NewBroker(server.display, server.voiceDispatcher)
	mux.Handle("/api/v1/events", broker)

	handler := withCommonHeaders(withCORS(mux))
	server.handler = RequestLogger(slog.Default() /*nolint:sloglint // use default global logger */)(handler)
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
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
				ID:             w.ID,
				Type:           w.Kind,
				Title:          w.Title,
				X:              w.X,
				Y:              w.Y,
				W:              w.W,
				H:              w.H,
				MinW:           w.MinW,
				MinH:           w.MinH,
				Size:           w.Size,
				Visible:        w.Visible,
				Mode:           w.Mode,
				Settings:       w.Settings,
				ConnectionRefs: w.ConnectionRefs,
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
		Name: serverSettings.Home.Name,
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

	snapshot := dashboard.Project(ctx, layout)
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, A2A-Version")
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

func syncOnLoad(
	ctx context.Context,
	syncer filesync.Syncer,
	layoutStore dashboard.LayoutStore,
	settingsStore homestate.SettingsStore,
	_ voice.Store,
) error {
	cfg, err := syncer.Load(ctx)
	if err != nil {
		return err
	}

	// 1. Sync Household Settings
	if _, err := settingsStore.SaveHouseholdSettings(ctx, homestate.HouseholdSettings{
		Home:    cfg.Home,
		Display: cfg.Display,
	}); err != nil {
		return err
	}

	// 2. Sync Rooms
	if _, err := settingsStore.SaveRooms(ctx, cfg.Rooms); err != nil {
		return err
	}

	// 3. Sync Tiles
	if _, err := settingsStore.SaveTiles(ctx, cfg.Tiles); err != nil {
		return err
	}

	// 4. Sync Widget Layout
	catalog := dashboard.RegisteredCatalog()
	catalogMap := make(map[string]dashboard.WidgetCatalogItem, len(catalog))
	for _, item := range catalog {
		catalogMap[item.Kind] = item
	}
	layout, err := dashboard.WidgetLayoutFromDashboardConfig(cfg.Dashboard, catalogMap)
	if err == nil {
		if _, err := layoutStore.SaveWidgetLayout(ctx, layout); err != nil {
			return err
		}
	}

	return nil
}

type widgetActionRequest struct {
	Arguments map[string]any `json:"arguments"`
	Actor     string         `json:"actor"`
	Confirmed bool           `json:"confirmed"`
}

func (s *Server) handleWidgetAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httphelper.WriteMethodNotAllowed(w, http.MethodPost)
		return
	}
	instanceID, actionID, ok := parseWidgetActionPath(r.URL.Path)
	if !ok {
		httphelper.WriteError(w, http.StatusNotFound, "widget action route not found")
		return
	}
	var req widgetActionRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
	}
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}
	if req.Actor == "" {
		req.Actor = "display"
	}
	result := s.actionDispatcher.Dispatch(r.Context(), widgetactions.Request{
		WidgetInstanceID: instanceID,
		ActionID:         actionID,
		Arguments:        req.Arguments,
		Actor:            req.Actor,
		Confirmed:        req.Confirmed,
	})
	if result.Issue != nil {
		httphelper.WriteJSON(w, result.HTTPStatus, map[string]any{"status": "error", "issue": result.Issue})
		return
	}
	httphelper.WriteJSON(w, result.HTTPStatus, result.Body)
}

func (s *Server) InvokeWidgetAction(
	ctx context.Context,
	widgetInstanceID string,
	actionID string,
	arguments map[string]any,
	actor string,
	confirmed bool,
) (map[string]any, error) {
	return s.actionDispatcher.InvokeWidgetAction(
		ctx,
		widgetInstanceID,
		actionID,
		arguments,
		actor,
		confirmed,
	)
}

func parseWidgetActionPath(path string) (string, string, bool) {
	const prefix = "/api/v1/widgets/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[1] != "actions" {
		return "", "", false
	}
	instanceID := strings.TrimSpace(parts[0])
	actionID := strings.TrimSpace(parts[2])
	return instanceID, actionID, instanceID != "" && actionID != ""
}

func (s *Server) buildWidgetSkillsSnapshot(
	ctx context.Context,
	layout dashboard.WidgetLayout,
) widgetskills.Snapshot {
	cfg := s.cfg
	if s.configPath != "" {
		if loaded, err := config.LoadConfig(s.configPath); err == nil {
			loaded.Server = s.cfg.Server
			loaded.MCP = s.cfg.MCP
			cfg = loaded
		}
	}

	layout = dashboard.NewHydrator(s.connectionResolver).HydrateWidgetLayout(ctx, layout)

	agentsList := []widgetskills.Agent{}
	regAgents := s.agentsManager.List(ctx, false)
	for _, agent := range regAgents {
		agentsList = append(agentsList, widgetskills.Agent{
			ID:              agent.ID,
			Name:            agent.Name,
			Description:     agent.Description,
			ProtocolBinding: agent.ProtocolBinding,
			Enabled:         agent.Enabled,
			Capabilities:    append([]string(nil), agent.Capabilities...),
			MCPScopes:       append([]string(nil), agent.MCPScopes...),
			AuthConfigured:  agent.AuthConfigured,
		})
	}

	timezone := "UTC"
	locale := "en"
	for _, w := range layout.Widgets {
		if w.Kind == "date-time" {
			if tzVal, ok := w.Settings["timezone"].(string); ok && tzVal != "" {
				timezone = tzVal
			}
			if locVal, ok := w.Settings["locale"].(string); ok && locVal != "" {
				locale = locVal
			}
			break
		}
	}

	wsCfg := widgetskills.Config{}
	wsCfg.Home.Locale = locale
	wsCfg.Home.Timezone = timezone
	wsCfg.Voice.PreferredAgentID = cfg.Voice.PreferredAgentID

	rooms, err := s.settings.Rooms(ctx)
	if err != nil {
		rooms = cfg.Rooms
	}
	tiles, err := s.settings.Tiles(ctx)
	if err != nil {
		tiles = cfg.Tiles
	}

	wsRooms := make([]widgetskills.RoomConfig, len(rooms))
	for i, r := range rooms {
		wsRooms[i] = widgetskills.RoomConfig{
			ID:      r.ID,
			Name:    r.Name,
			Summary: r.Summary,
			Status:  r.Status,
		}
	}
	wsCfg.Rooms = wsRooms

	wsTiles := make([]widgetskills.TileConfig, len(tiles))
	for i, t := range tiles {
		wsTiles[i] = widgetskills.TileConfig{
			ID:     t.ID,
			Kind:   t.Kind,
			Label:  t.Label,
			Value:  t.Value,
			Detail: t.Detail,
		}
	}
	wsCfg.Tiles = wsTiles

	wsWidgets := make([]widgetskills.WidgetInstance, len(layout.Widgets))
	for i, w := range layout.Widgets {
		wsWidgets[i] = widgetskills.WidgetInstance{
			ID:             w.ID,
			Kind:           w.Kind,
			Title:          w.Title,
			X:              w.X,
			Y:              w.Y,
			W:              w.W,
			H:              w.H,
			Visible:        w.Visible,
			Mode:           w.Mode,
			Size:           w.Size,
			Settings:       w.Settings,
			ConnectionRefs: w.ConnectionRefs,
			Data:           w.Data,
		}
	}
	wsLayout := widgetskills.WidgetLayout{
		ProfileID: layout.ProfileID,
		Widgets:   wsWidgets,
	}

	return widgetskills.Snapshot{
		Config:      wsCfg,
		Layout:      wsLayout,
		Agents:      agentsList,
		GeneratedAt: time.Now().UTC(),
	}
}
