package filesync

import (
	"context"
	"path/filepath"
	"testing"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/model"
)

func TestInMemorySyncerSyncAgentsUpdatesCurrentConfig(t *testing.T) {
	syncer := NewInMemorySyncer(config.DefaultConfig())
	agents := []model.AgentConfig{{ID: "kronk", Name: "Kronk"}}

	if err := syncer.SyncAgents(context.Background(), agents); err != nil {
		t.Fatalf("SyncAgents() error = %v", err)
	}
	got, err := syncer.AgentsConfig(context.Background())
	if err != nil {
		t.Fatalf("AgentsConfig() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "kronk" {
		t.Fatalf("agents = %+v", got)
	}
}

func TestFileSyncerSyncWritesDatabaseConfigIntoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	fileCfg := config.DefaultConfig()
	fileCfg.Home.Name = "File Home"
	if err := config.SaveYAML(path, fileCfg); err != nil {
		t.Fatalf("SaveYAML() error = %v", err)
	}
	dbCfg := config.DefaultConfig()
	dbCfg.Home.Name = "DB Home"
	syncer := NewFileSyncer(path, fixtureConfigStore{cfg: dbCfg})

	if err := syncer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	got, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Home.Name != "DB Home" {
		t.Fatalf("home name = %q", got.Home.Name)
	}
}

type fixtureConfigStore struct {
	cfg config.Config
}

func (s fixtureConfigStore) Config(context.Context) (config.Config, error) {
	return s.cfg, nil
}
