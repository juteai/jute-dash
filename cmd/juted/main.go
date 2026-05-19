package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"jute-dash/internal/config"
	"jute-dash/internal/server"
	"jute-dash/internal/store"
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

	bootstrapProvided := strings.TrimSpace(*configPath) != "" && needsSeed
	bootstrap := config.Default()
	if bootstrapProvided {
		bootstrap, err = config.Load(*configPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
	}

	result, err := runtimeStore.Initialize(ctx, bootstrap, bootstrapProvided)
	if err != nil {
		log.Fatalf("initialize store: %v", err)
	}

	cfg := result.Config
	cfg.Server = bootstrap.Server
	if *listenOverride != "" {
		cfg.Server.ListenAddress = *listenOverride
	}

	handler := server.NewWithSetupStatusAndLayoutStore(cfg, version, result.Setup, runtimeStore)
	log.Printf("jute data directory: %s", dataDir)
	log.Printf("jute hub listening on http://%s", cfg.Server.ListenAddress)
	if err := http.ListenAndServe(cfg.Server.ListenAddress, handler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
