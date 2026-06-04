package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"jute-dash/apps/hub/internal/app"
	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/mcp"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/pkg/widgetskills"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configPath := flag.String("config", os.Getenv("JUTE_CONFIG"), "optional path to Jute bootstrap config YAML or JSON")
	dataDirOverride := flag.String("data-dir", os.Getenv("JUTE_DATA_DIR"), "override Jute runtime data directory")
	listenOverride := flag.String("listen", os.Getenv("JUTE_LISTEN"), "override listen address")
	flag.Parse()

	ctx := context.Background()

	dataDir, err := app.ResolveDataDir(*dataDirOverride)
	if err != nil {
		return fmt.Errorf("resolve data directory: %w", err)
	}
	runtimeStore, err := app.Open(app.DatabasePath(dataDir))
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() {
		if err := runtimeStore.Close(); err != nil {
			log.Printf("close store: %v", err)
		}
	}()

	needsSeed, err := runtimeStore.IsSeeded(ctx)
	if err != nil {
		return fmt.Errorf("inspect store: %w", err)
	}
	// needsSeed is true if count == 0, wait, runtimeStore.IsSeeded returns true ifCount > 0, so needsSeed should be !seeded
	seeded := needsSeed
	needsSeed = !seeded

	bootstrap := config.DefaultConfig()
	configProvided := strings.TrimSpace(*configPath) != ""
	if configProvided {
		bootstrap, err = config.LoadConfig(*configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
	}
	bootstrapProvided := configProvided && needsSeed

	result, err := runtimeStore.Initialize(ctx, bootstrap, bootstrapProvided)
	if err != nil {
		return fmt.Errorf("initialize store: %w", err)
	}

	cfg, ok := result.Config.(config.Config)
	if !ok {
		return fmt.Errorf("unexpected config type: %T", result.Config)
	}
	cfg.Server = bootstrap.Server
	cfg.MCP = bootstrap.MCP
	if configProvided {
		cfg.Agents = bootstrap.Agents
	}
	if *listenOverride != "" {
		cfg.Server.ListenAddress = *listenOverride
	}

	displayActions := displayactions.NewDispatcher()
	handler := app.NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(
		cfg,
		version,
		result.Setup,
		runtimeStore,
		*configPath,
		displayActions,
	)
	log.Printf("jute data directory: %q", dataDir) //nolint:gosec // Quoted local path is useful startup diagnostics.

	var mcpServer *http.Server
	if cfg.MCP.Enabled {
		mcpProvider := &mcpSnapshotProvider{
			cfg:        cfg,
			configPath: *configPath,
			store:      runtimeStore,
		}
		mcpMux := http.NewServeMux()
		mcpMux.Handle(cfg.MCP.Path, mcp.NewHandler(cfg.MCP, version, mcpProvider, displayActions))
		mcpServer = &http.Server{
			Addr:              cfg.MCP.ListenAddress,
			Handler:           mcpMux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		go func() {
			log.Printf("jute MCP bridge listening on http://%s%s", cfg.MCP.ListenAddress, cfg.MCP.Path)
			if err := mcpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("serve MCP bridge: %v", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := mcpServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("shutdown MCP bridge: %v", err)
			}
		}()
	}

	log.Printf("jute hub listening on http://%s", cfg.Server.ListenAddress)
	hubServer := &http.Server{
		Addr:              cfg.Server.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := hubServer.ListenAndServe(); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

type mcpSnapshotProvider struct {
	cfg        config.Config
	configPath string
	store      *app.Store
}

func (p *mcpSnapshotProvider) Snapshot(ctx context.Context) (widgetskills.Snapshot, error) {
	cfg := p.currentConfig()
	layout, err := p.store.WidgetLayout(ctx, "")
	if err != nil {
		return widgetskills.Snapshot{}, err
	}
	layout = dashboard.HydrateWidgetLayout(ctx, layout)
	agentsList := []widgetskills.Agent{}
	for _, agent := range cfg.Agents {
		agentsList = append(agentsList, widgetskills.Agent{
			ID:              agent.ID,
			Name:            agent.Name,
			Description:     agent.Description,
			ProtocolBinding: agent.ProtocolBinding,
			Enabled:         agent.Enabled,
			Capabilities:    append([]string(nil), agent.Capabilities...),
			MCPScopes:       append([]string(nil), agent.MCPScopes...),
			AuthConfigured:  agent.Auth != nil,
		})
	}

	wsCfg := widgetskills.Config{}
	wsCfg.Home.Locale = cfg.Home.Locale
	wsCfg.Home.Timezone = cfg.Home.Timezone
	wsCfg.Voice.PreferredAgentID = cfg.Voice.PreferredAgentID

	wsRooms := make([]widgetskills.RoomConfig, len(cfg.Rooms))
	for i, r := range cfg.Rooms {
		wsRooms[i] = widgetskills.RoomConfig{
			ID:      r.ID,
			Name:    r.Name,
			Summary: r.Summary,
			Status:  r.Status,
		}
	}
	wsCfg.Rooms = wsRooms

	wsTiles := make([]widgetskills.TileConfig, len(cfg.Tiles))
	for i, t := range cfg.Tiles {
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
	}, nil
}

func (p *mcpSnapshotProvider) currentConfig() config.Config {
	if strings.TrimSpace(p.configPath) == "" {
		return p.cfg
	}
	cfg, err := config.LoadConfig(p.configPath)
	if err != nil {
		return p.cfg
	}
	cfg.Server = p.cfg.Server
	cfg.MCP = p.cfg.MCP
	return cfg
}
