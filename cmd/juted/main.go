package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"jute-dash/internal/config"
	"jute-dash/internal/displayactions"
	"jute-dash/internal/mcpbridge"
	"jute-dash/internal/server"
	"jute-dash/internal/store"
	"jute-dash/internal/weather"
	"jute-dash/internal/widgetskills"
)

var version = "dev"

func main() {
	configPath := flag.String("config", os.Getenv("JUTE_CONFIG"), "optional path to Jute bootstrap config YAML or JSON")
	dataDirOverride := flag.String("data-dir", os.Getenv("JUTE_DATA_DIR"), "override Jute runtime data directory")
	listenOverride := flag.String("listen", os.Getenv("JUTE_LISTEN"), "override listen address")
	flag.Parse()

	ctx := context.Background()

	dataDir, err := store.ResolveDataDir(*dataDirOverride)
	if err != nil {
		log.Fatalf("resolve data directory: %v", err)
	}
	runtimeStore, err := store.Open(store.DatabasePath(dataDir))
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := runtimeStore.Close(); err != nil {
			log.Printf("close store: %v", err)
		}
	}()

	needsSeed, err := runtimeStore.NeedsSeed(ctx)
	if err != nil {
		log.Fatalf("inspect store: %v", err)
	}

	bootstrap := config.Default()
	configProvided := strings.TrimSpace(*configPath) != ""
	if configProvided {
		bootstrap, err = config.Load(*configPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
	}
	bootstrapProvided := configProvided && needsSeed

	result, err := runtimeStore.Initialize(ctx, bootstrap, bootstrapProvided)
	if err != nil {
		log.Fatalf("initialize store: %v", err)
	}

	cfg := result.Config
	cfg.Server = bootstrap.Server
	cfg.MCP = bootstrap.MCP
	if configProvided {
		cfg.Agents = bootstrap.Agents
	}
	if *listenOverride != "" {
		cfg.Server.ListenAddress = *listenOverride
	}

	displayActions := displayactions.NewDispatcher()
	handler := server.NewWithSetupStatusAndLayoutStoreAndConfigPathAndDisplayActions(cfg, version, result.Setup, runtimeStore, *configPath, displayActions)
	log.Printf("jute data directory: %s", dataDir)

	var mcpServer *http.Server
	if cfg.MCP.Enabled {
		mcpProvider := &mcpSnapshotProvider{
			cfg:     cfg,
			store:   runtimeStore,
			weather: weather.NewClient(),
			getAgents: func() []config.AgentConfig {
				return cfg.Agents
			},
		}
		mcpMux := http.NewServeMux()
		mcpMux.Handle(cfg.MCP.Path, mcpbridge.NewHandler(cfg.MCP, version, mcpProvider, displayActions))
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
	if err := http.ListenAndServe(cfg.Server.ListenAddress, handler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

type mcpSnapshotProvider struct {
	cfg       config.Config
	store     *store.Store
	weather   weather.Provider
	getAgents func() []config.AgentConfig
}

func (p *mcpSnapshotProvider) Snapshot(ctx context.Context) (widgetskills.Snapshot, error) {
	layout, err := p.store.WidgetLayout(ctx, "")
	if err != nil {
		return widgetskills.Snapshot{}, err
	}
	agents := []widgetskills.Agent{}
	for _, agent := range p.getAgents() {
		agents = append(agents, widgetskills.Agent{
			ID:              agent.ID,
			Name:            agent.Name,
			Description:     agent.Description,
			ProtocolBinding: agent.ProtocolBinding,
			Enabled:         agent.Enabled,
			Capabilities:    append([]string(nil), agent.Capabilities...),
			AuthConfigured:  agent.Auth != nil,
		})
	}
	return widgetskills.Snapshot{
		Config:      p.cfg,
		Layout:      layout,
		Weather:     p.weather.Current(ctx, p.cfg.Weather),
		Agents:      agents,
		GeneratedAt: time.Now().UTC(),
	}, nil
}
