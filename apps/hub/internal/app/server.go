package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

type Server struct {
	cfg             config.Config
	agentsManager   *agents.AgentManager
	messages        a2aclient.MessageSender
	setup           homestate.SetupStatus
	layout          dashboard.WidgetLayout
	layoutStore     dashboard.LayoutStore
	settings        homestate.SettingsStore
	voice           voice.Config
	voiceStore      voice.Store
	configPath      string
	syncer          filesync.Syncer
	display         *displayactions.Dispatcher
	voiceDispatcher *voice.Dispatcher
	turnRunner      *agents.Runner
	mu              sync.Mutex
	started         time.Time
	version         string
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
	return newServer(cfg, version, nil, homestate.SetupStatus{Complete: true}, layout, nil, nil, nil, "", nil)
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
	return newServer(cfg, version, nil, setup, layout, nil, nil, nil, "", nil)
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
) http.Handler {
	layout := dashboard.DefaultWidgetLayout()
	if layoutStore != nil {
		if loaded, err := layoutStore.WidgetLayout(context.Background(), ""); err == nil {
			layout = loaded
		}
	}
	return newServer(cfg, version, nil, setup, layout, layoutStore, settingsStore, voiceStore, configPath, display)
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
) http.Handler {
	if messageSender == nil {
		messageSender = a2aclient.NewJSONRPCClient()
	}

	activeLayoutStore := layoutStore
	if activeLayoutStore == nil {
		activeLayoutStore = dashboard.NewMemoryRepositoryWithLayout(layout)
	}

	//nolint:reassign // hook is designed to be set by the hub package
	widgets.SaveSettingsHook = func(ctx context.Context, instanceID string, settings map[string]any) error {
		if activeLayoutStore == nil {
			return errors.New("layout store not initialized")
		}
		layout, err := activeLayoutStore.WidgetLayout(ctx, "")
		if err != nil {
			return err
		}
		updated := false
		for i, widget := range layout.Widgets {
			if widget.ID == instanceID {
				if layout.Widgets[i].Settings == nil {
					layout.Widgets[i].Settings = make(map[string]any)
				}
				for k, v := range settings {
					layout.Widgets[i].Settings[k] = v
				}
				updated = true
				break
			}
		}
		if updated {
			_, err = activeLayoutStore.SaveWidgetLayout(ctx, layout)
			return err
		}
		return nil
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

	server := &Server{
		cfg:             cfg,
		agentsManager:   agentsManager,
		messages:        messageSender,
		setup:           setup,
		layout:          layout,
		layoutStore:     activeLayoutStore,
		settings:        activeSettingsStore,
		voice:           cfg.Voice,
		voiceStore:      activeVoiceStore,
		configPath:      configPath,
		syncer:          syncer,
		display:         display,
		voiceDispatcher: voiceDispatcher,
		started:         time.Now().UTC(),
		version:         version,
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
	mux.HandleFunc("/api/widgets/smart-home/dispatch", server.handleSmartHomeDispatch)
	mux.HandleFunc("/api/widgets/music-player/action", server.handleMusicPlayerAction)
	mux.HandleFunc("/api/widgets/music-player/select", server.handleMusicPlayerSelect)
	mux.HandleFunc("/api/widgets/philips-hue/register", server.handleHueRegister)
	mux.HandleFunc("/api/widgets/spotify/auth", server.handleSpotifyAuth)
	mux.HandleFunc("/api/widgets/spotify/callback", server.handleSpotifyCallback)

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
	return RequestLogger(slog.Default() /*nolint:sloglint // use default global logger */)(handler)
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

type smartHomeDispatchRequest struct {
	InstanceID  string `json:"instanceId"`
	InstanceID2 string `json:"instance_id"`
	DeviceID    string `json:"deviceId"`
	Action      string `json:"action"`
	Command     string `json:"command"`
	Value       any    `json:"value"`
}

func (s *Server) handleSmartHomeDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httphelper.WriteMethodNotAllowed(w, http.MethodPost)
		return
	}

	if r.Body == nil {
		httphelper.WriteError(w, http.StatusBadRequest, "request body is required")
		return
	}

	var req smartHomeDispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	instID := req.InstanceID
	if instID == "" {
		instID = req.InstanceID2
	}
	if instID == "" {
		instID = "smart-home-widget-1"
	}

	action := req.Action
	if action == "" {
		action = req.Command
	}

	if instID == "" || req.DeviceID == "" || action == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "missing required fields (instanceId, deviceId, action)")
		return
	}

	// Validate action list
	validActions := map[string]bool{
		"turn_on":         true,
		"turn_off":        true,
		"toggle":          true,
		"brightness":      true,
		"set_brightness":  true,
		"set_temperature": true,
	}
	if !validActions[action] {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid command action")
		return
	}

	// Validate set_temperature range
	if action == "set_temperature" && req.Value != nil {
		if val, ok := req.Value.(float64); ok {
			if val < 5.0 || val > 50.0 {
				httphelper.WriteError(w, http.StatusBadRequest, "temperature value out of range (5-50)")
				return
			}
		}
	}

	layout, err := s.layoutStore.WidgetLayout(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "failed to load widget layout")
		return
	}

	kind := "zigbee2mqtt" // default fallback
	for _, inst := range layout.Widgets {
		if inst.ID == instID {
			kind = inst.Kind
			break
		}
	}

	widget, exists := widgets.Get(kind)
	if !exists {
		httphelper.WriteError(w, http.StatusNotFound, fmt.Sprintf("widget kind %q not registered", kind))
		return
	}

	actionWidget, ok := widget.(widgets.ActionWidget)
	if !ok {
		httphelper.WriteError(w, http.StatusBadRequest, fmt.Sprintf("widget kind %q does not support actions", kind))
		return
	}

	snapshot := s.buildWidgetSkillsSnapshot(r.Context(), layout)

	args := map[string]any{
		"deviceId": req.DeviceID,
		"action":   action,
		"value":    req.Value,
	}

	res, err := actionWidget.InvokeAction(r.Context(), snapshot, instID, "control_device", args)
	if err != nil {
		if strings.Contains(err.Error(), "device not found") {
			httphelper.WriteError(w, http.StatusNotFound, err.Error())
		} else {
			httphelper.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	_, _ = s.display.Notify("Device controlled successfully", "info")

	httphelper.WriteJSON(w, http.StatusOK, res)
}

type musicPlayerActionRequest struct {
	InstanceID  string         `json:"instance_id"`
	InstanceID2 string         `json:"instanceId"`
	Action      string         `json:"action"`
	Client      string         `json:"client"`
	Arguments   map[string]any `json:"arguments"`
}

func (s *Server) handleMusicPlayerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httphelper.WriteMethodNotAllowed(w, http.MethodPost)
		return
	}

	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	var req musicPlayerActionRequest
	if val, ok := raw["instance_id"].(string); ok {
		req.InstanceID = val
	}
	if val, ok := raw["instanceId"].(string); ok {
		req.InstanceID2 = val
	}
	if val, ok := raw["action"].(string); ok {
		req.Action = val
	}
	if val, ok := raw["client"].(string); ok {
		req.Client = val
	}
	if args, ok := raw["arguments"].(map[string]any); ok {
		req.Arguments = args
	} else {
		req.Arguments = make(map[string]any)
	}

	// Merge flat root values
	for k, v := range raw {
		if k != "instance_id" && k != "instanceId" && k != "action" && k != "client" && k != "arguments" {
			req.Arguments[k] = v
		}
	}

	if val, ok := raw["value"]; ok {
		req.Arguments["value"] = val
		req.Arguments["volume"] = val
	}

	if req.Action == "volume" {
		req.Action = "set_volume"
	}
	if req.Action == "select" {
		req.Action = "select_client"
	}

	instID := req.InstanceID
	if instID == "" {
		instID = req.InstanceID2
	}
	if instID == "" {
		instID = "music-player-widget-1"
	}

	if instID == "" || req.Action == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "missing required fields (instance_id, action)")
		return
	}

	layout, err := s.layoutStore.WidgetLayout(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "failed to load widget layout")
		return
	}

	kind := "spotify" // default fallback
	for _, inst := range layout.Widgets {
		if inst.ID == instID {
			kind = inst.Kind
			break
		}
	}

	widget, exists := widgets.Get(kind)
	if !exists {
		httphelper.WriteError(w, http.StatusNotFound, fmt.Sprintf("widget kind %q not registered", kind))
		return
	}

	actionWidget, ok := widget.(widgets.ActionWidget)
	if !ok {
		httphelper.WriteError(w, http.StatusBadRequest, fmt.Sprintf("widget kind %q does not support actions", kind))
		return
	}

	snapshot := s.buildWidgetSkillsSnapshot(r.Context(), layout)

	if req.Client != "" {
		_, errSelect := actionWidget.InvokeAction(
			r.Context(),
			snapshot,
			instID,
			"select_client",
			map[string]any{"client": req.Client},
		)
		if errSelect != nil {
			httphelper.WriteError(w, http.StatusBadRequest, errSelect.Error())
			return
		}
	}

	res, err := actionWidget.InvokeAction(r.Context(), snapshot, instID, req.Action, req.Arguments)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "volume") ||
			strings.Contains(errMsg, "client") ||
			strings.Contains(errMsg, "action") ||
			strings.Contains(errMsg, "arguments") {
			httphelper.WriteError(w, http.StatusBadRequest, errMsg)
		} else {
			httphelper.WriteError(w, http.StatusInternalServerError, errMsg)
		}
		return
	}

	httphelper.WriteJSON(w, http.StatusOK, res)
}

func (s *Server) handleMusicPlayerSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httphelper.WriteMethodNotAllowed(w, http.MethodPost)
		return
	}
	var req struct {
		InstanceID  string `json:"instance_id"`
		InstanceID2 string `json:"instanceId"`
		Client      string `json:"client"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Client == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Client cannot be empty")
		return
	}

	if len(req.Client) > 256 || strings.Contains(req.Client, "../") {
		httphelper.WriteError(w, http.StatusBadRequest, "Invalid client name")
		return
	}

	instID := req.InstanceID
	if instID == "" {
		instID = req.InstanceID2
	}
	if instID == "" {
		instID = "music-player-widget-1"
	}

	layout, err := s.layoutStore.WidgetLayout(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "failed to load widget layout")
		return
	}

	kind := "spotify" // default fallback
	for _, inst := range layout.Widgets {
		if inst.ID == instID {
			kind = inst.Kind
			break
		}
	}

	widget, exists := widgets.Get(kind)
	if !exists {
		httphelper.WriteError(w, http.StatusNotFound, fmt.Sprintf("widget kind %q not registered", kind))
		return
	}
	actionWidget, ok := widget.(widgets.ActionWidget)
	if !ok {
		httphelper.WriteError(w, http.StatusBadRequest, fmt.Sprintf("widget kind %q does not support actions", kind))
		return
	}
	snapshot := s.buildWidgetSkillsSnapshot(r.Context(), layout)
	res, err := actionWidget.InvokeAction(
		r.Context(),
		snapshot,
		instID,
		"select_client",
		map[string]any{"client": req.Client},
	)
	if err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, res)
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

	layout = dashboard.HydrateWidgetLayout(ctx, layout)

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
			ID:       w.ID,
			Kind:     w.Kind,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			Visible:  w.Visible,
			Mode:     w.Mode,
			Size:     w.Size,
			Settings: w.Settings,
			Data:     w.Data,
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

type hueRegisterRequest struct {
	InstanceID string `json:"instance_id"`
	BridgeIP   string `json:"bridge_ip"`
}

func (s *Server) handleHueRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httphelper.WriteMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req hueRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if req.InstanceID == "" || req.BridgeIP == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "missing instance_id or bridge_ip")
		return
	}

	payload := map[string]any{
		"devicetype": "jute_dash#local_hub",
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	url := fmt.Sprintf("http://%s/api", req.BridgeIP)
	postReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	postReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to connect to bridge: %v", err))
		return
	}
	defer resp.Body.Close()

	var responseList []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&responseList); err != nil {
		httphelper.WriteError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to decode bridge response: %v", err),
		)
		return
	}

	if len(responseList) == 0 {
		httphelper.WriteError(w, http.StatusInternalServerError, "empty response from bridge")
		return
	}

	first := responseList[0]
	if errMap, ok := first["error"].(map[string]any); ok {
		desc, _ := errMap["description"].(string)
		httphelper.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bridge registration failed: %s", desc))
		return
	}

	successMap, ok := first["success"].(map[string]any)
	if !ok {
		httphelper.WriteError(w, http.StatusInternalServerError, "invalid response from bridge")
		return
	}

	username, _ := successMap["username"].(string)
	if username == "" {
		httphelper.WriteError(w, http.StatusInternalServerError, "no username returned from bridge")
		return
	}

	layout, err := s.layoutStore.WidgetLayout(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "failed to load layout")
		return
	}

	updated := false
	for i, widget := range layout.Widgets {
		if widget.ID == req.InstanceID {
			if layout.Widgets[i].Settings == nil {
				layout.Widgets[i].Settings = make(map[string]any)
			}
			layout.Widgets[i].Settings["username"] = username
			layout.Widgets[i].Settings["bridge_ip"] = req.BridgeIP
			updated = true
			break
		}
	}

	if updated {
		if _, err := s.layoutStore.SaveWidgetLayout(r.Context(), layout); err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save layout: %v", err))
			return
		}
	}

	httphelper.WriteJSON(w, http.StatusOK, map[string]any{
		"username": username,
	})
}

func (s *Server) handleSpotifyAuth(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instance_id")
	if instanceID == "" {
		instanceID = "spotify-widget-1"
	}

	layout, err := s.layoutStore.WidgetLayout(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "failed to load widget layout")
		return
	}

	var clientID string
	for _, widget := range layout.Widgets {
		if widget.ID == instanceID {
			if cid, ok := widget.Settings["client_id"].(string); ok {
				clientID = cid
			}
			break
		}
	}

	if clientID == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Spotify Client ID is not configured in widget settings")
		return
	}

	redirectURI := "http://localhost:8787/api/widgets/spotify/callback"
	authURL := fmt.Sprintf(
		"https://accounts.spotify.com/authorize?client_id=%s&response_type=code&redirect_uri=%s"+
			"&scope=user-read-playback-state%%20user-modify-playback-state&state=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(instanceID),
	)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleSpotifyCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	instanceID := r.URL.Query().Get("state")

	if code == "" || instanceID == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "missing code or state in callback")
		return
	}

	layout, err := s.layoutStore.WidgetLayout(r.Context(), "")
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "failed to load layout")
		return
	}

	var clientID, clientSecret string
	for _, widget := range layout.Widgets {
		if widget.ID == instanceID {
			if cid, ok := widget.Settings["client_id"].(string); ok {
				clientID = cid
			}
			if secret, ok := widget.Settings["client_secret"].(string); ok {
				clientSecret = secret
			}
			break
		}
	}

	if clientID == "" || clientSecret == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Spotify credentials not found in settings")
		return
	}

	redirectURI := "http://localhost:8787/api/widgets/spotify/callback"
	tokenURL := "https://accounts.spotify.com/api/token" //nolint:gosec // URL is not a secret credential

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		tokenURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	authHeader := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", clientID, clientSecret)))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", authHeader))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errData map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errData)
		httphelper.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Spotify returned error: %v", errData))
		return
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	expiresAt := time.Now().Unix() + tokenResp.ExpiresIn

	updated := false
	for i, widget := range layout.Widgets {
		if widget.ID == instanceID {
			if layout.Widgets[i].Settings == nil {
				layout.Widgets[i].Settings = make(map[string]any)
			}
			layout.Widgets[i].Settings["access_token"] = tokenResp.AccessToken
			if tokenResp.RefreshToken != "" {
				layout.Widgets[i].Settings["refresh_token"] = tokenResp.RefreshToken
			}
			layout.Widgets[i].Settings["expires_at"] = expiresAt
			updated = true
			break
		}
	}

	if updated {
		if _, err := s.layoutStore.SaveWidgetLayout(r.Context(), layout); err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
