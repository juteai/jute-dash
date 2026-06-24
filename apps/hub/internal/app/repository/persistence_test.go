package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHomeRepositoryPersistsAdapterConnectionJSON(t *testing.T) {
	db := openRepositoryTestDB(t, &AdapterConnectionDB{})
	repo := NewHomeRepository(db)

	saved, err := repo.SaveAdapterConnection(context.Background(), AdapterConnection{
		ID:         "spotify-main",
		Kind:       "spotify",
		Name:       "Spotify",
		Settings:   map[string]any{"client_id": "abc"},
		SecretRefs: map[string]string{"refresh_token": "secret/ref"},
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("SaveAdapterConnection() error = %v", err)
	}
	if !saved.Enabled || saved.Settings["client_id"] != "abc" || saved.SecretRefs["refresh_token"] != "secret/ref" {
		t.Fatalf("unexpected saved connection: %+v", saved)
	}

	list, err := repo.AdapterConnections(context.Background())
	if err != nil {
		t.Fatalf("AdapterConnections() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != "spotify-main" {
		t.Fatalf("unexpected connection list: %+v", list)
	}
}

func TestVoiceRepositorySelectsCommandSTTProviderFromPersistedManifest(t *testing.T) {
	db := openRepositoryTestDB(t, &SettingsDB{}, &ProviderPackDB{})
	if err := db.Create(&SettingsDB{
		DeviceProfileID:         DefaultDeviceProfileID,
		Enabled:                 1,
		STTProviderID:           "local-stt",
		STTModelID:              "tiny",
		CommandProvidersEnabled: 1,
	}).Error; err != nil {
		t.Fatalf("seed voice settings: %v", err)
	}
	if err := db.Create(&ProviderPackDB{
		ID:            "local-stt",
		Name:          "Local STT",
		Kind:          ProviderKindSTT,
		TransportType: "command",
		HealthStatus:  "available",
		ManifestJSON: `{
			"id": "local-stt",
			"name": "Local STT",
			"version": "1.0.0",
			"kind": "stt",
			"transport": {"type": "command", "command": "/usr/bin/true", "args": ["--model", "{modelId}", "{inputPath}"]},
			"capabilities": {"offline": true, "languages": ["en-GB"]}
		}`,
	}).Error; err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	provider, err := NewVoiceRepository(db).ActiveSTTProvider(context.Background(), "")
	if err != nil {
		t.Fatalf("ActiveSTTProvider() error = %v", err)
	}
	command, ok := provider.(CommandSTTProvider)
	if !ok {
		t.Fatalf("ActiveSTTProvider() = %T, want CommandSTTProvider", provider)
	}
	if command.ProviderID != "local-stt" || command.Command != "/usr/bin/true" ||
		command.ModelID != "tiny" || command.Language != "en-GB" {
		t.Fatalf("unexpected command provider: %+v", command)
	}
}

func openRepositoryTestDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}
