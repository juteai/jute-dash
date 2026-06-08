package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"jute-dash/apps/hub/internal/app"
	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/mcp"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/pkg/widgetskills"

	_ "jute-dash/widgets/chathistory"
	_ "jute-dash/widgets/datetime"
	_ "jute-dash/widgets/markets"
	_ "jute-dash/widgets/rss"
	_ "jute-dash/widgets/weather"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err) //nolint:sloglint // global slog permitted for startup exit
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", os.Getenv("JUTE_CONFIG"), "optional path to Jute bootstrap config YAML or JSON")
	dataDirOverride := flag.String("data-dir", os.Getenv("JUTE_DATA_DIR"), "override Jute runtime data directory")
	listenOverride := flag.String("listen", os.Getenv("JUTE_LISTEN"), "override listen address")
	flag.Parse()

	ctx := context.Background()

	// Initial fallback logger before config is loaded
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	dataDir, err := app.ResolveDataDir(*dataDirOverride)
	if err != nil {
		return fmt.Errorf("resolve data directory: %w", err)
	}
	app.SetBackgroundsDir(app.BackgroundsDir(dataDir))
	runtimeStore, err := app.Open(app.DatabasePath(dataDir), logger)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() {
		if err := runtimeStore.Close(); err != nil {
			logger.Error("close store failed", "error", err)
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

	// Setup structured logging using config settings
	logHandler, err := app.SetupLogger(cfg.Log, dataDir)
	if err != nil {
		return fmt.Errorf("setup logger: %w", err)
	}
	logger = slog.New(logHandler)
	slog.SetDefault(logger)
	runtimeStore.SetLogger(logger)

	// Redirect standard library log package output to slog
	log.SetFlags(0)
	log.SetOutput(slog.NewLogLogger(logHandler, slog.LevelInfo).Writer())

	displayActions := displayactions.NewDispatcher()
	handler := app.NewServer(
		cfg,
		version,
		result.Setup,
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		*configPath,
		displayActions,
	)
	logger.Info("jute data directory", "path", dataDir)

	baseCtx, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()

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
			BaseContext: func(_ net.Listener) context.Context {
				return baseCtx
			},
		}
		go func() {
			logger.Info(
				"jute MCP bridge listening",
				"url",
				fmt.Sprintf("http://%s%s", cfg.MCP.ListenAddress, cfg.MCP.Path),
			)
			if err := mcpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("serve MCP bridge failed", "error", err)
			}
		}()
	}

	logger.Info("jute hub listening", "url", fmt.Sprintf("http://%s", cfg.Server.ListenAddress))
	hubServer := &http.Server{
		Addr:              cfg.Server.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return baseCtx
		},
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 2)
	go func() {
		if err := hubServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("serve: %w", err)
		}
	}()

	select {
	case sig := <-stop:
		logger.Info("received signal, shutting down gracefully", "signal", sig.String())
	case err := <-errChan:
		return err
	}

	cancelBase()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	if mcpServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mcpServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("shutdown MCP bridge failed", "error", err)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := hubServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown hub server failed", "error", err)
		}
	}()

	wg.Wait()
	return nil
}

type mcpSnapshotProvider struct {
	cfg        config.Config
	configPath string
	store      *app.Store
}

func (p *mcpSnapshotProvider) Snapshot(ctx context.Context) (widgetskills.Snapshot, error) {
	cfg := p.currentConfig()
	layout, err := p.store.DashboardRepo.WidgetLayout(ctx, "")
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
