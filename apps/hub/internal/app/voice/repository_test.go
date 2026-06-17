package voice

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSatelliteInstallProjectionOmitsCredentialSecretReference(t *testing.T) {
	repo, db := openVoiceRepository(t)
	ctx := context.Background()

	satellite, err := repo.SaveSatelliteInstall(ctx, SatelliteRecord{
		ID:                  "sat-kitchen",
		DisplayName:         "Kitchen Satellite",
		RoomLabel:           "Kitchen",
		DeviceProfileID:     "kitchen-voice",
		Status:              SatelliteStatusPaired,
		Version:             "0.1.0",
		CredentialSecretRef: "env:JUTE_SATELLITE_TOKEN",
	})
	if err != nil {
		t.Fatalf("SaveSatelliteInstall() error = %v", err)
	}

	if satellite.ID != "sat-kitchen" ||
		satellite.DisplayName != "Kitchen Satellite" ||
		satellite.DeviceProfileID != "kitchen-voice" ||
		satellite.Status != SatelliteStatusPaired ||
		satellite.Version != "0.1.0" {
		t.Fatalf("unexpected satellite projection: %+v", satellite)
	}
	assertJSONOmits(t, satellite, "JUTE_SATELLITE_TOKEN", "credential", "secret")

	var stored SatelliteInstallDB
	if err := db.First(&stored, "id = ?", "sat-kitchen").Error; err != nil {
		t.Fatalf("load stored satellite: %v", err)
	}
	if stored.CredentialSecretRef != "env:JUTE_SATELLITE_TOKEN" {
		t.Fatalf("expected credential secret reference to persist internally, got %q", stored.CredentialSecretRef)
	}

	listed, err := repo.VoiceSatellites(ctx)
	if err != nil {
		t.Fatalf("VoiceSatellites() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "sat-kitchen" {
		t.Fatalf("unexpected satellite list: %+v", listed)
	}
	assertJSONOmits(t, listed, "JUTE_SATELLITE_TOKEN", "credential", "secret")
}

func TestRevokedSatelliteIsDistinctFromOfflineAndMisconfigured(t *testing.T) {
	repo, _ := openVoiceRepository(t)
	ctx := context.Background()

	for _, status := range []string{SatelliteStatusOffline, SatelliteStatusMisconfigured} {
		_, err := repo.SaveSatelliteInstall(ctx, SatelliteRecord{
			ID:                  "sat-" + status,
			DisplayName:         status,
			Status:              status,
			CredentialSecretRef: "env:SAT_" + strings.ToUpper(status),
		})
		if err != nil {
			t.Fatalf("SaveSatelliteInstall(%s) error = %v", status, err)
		}
	}

	revoked, err := repo.SaveSatelliteInstall(ctx, SatelliteRecord{
		ID:                  "sat-revoked",
		DisplayName:         "Revoked",
		Status:              SatelliteStatusPaired,
		CredentialSecretRef: "env:SAT_REVOKED",
	})
	if err != nil {
		t.Fatalf("SaveSatelliteInstall(revoked seed) error = %v", err)
	}
	revoked, err = repo.RevokeSatellite(ctx, revoked.ID)
	if err != nil {
		t.Fatalf("RevokeSatellite() error = %v", err)
	}
	if revoked.Status != SatelliteStatusRevoked || revoked.RevokedAt == "" {
		t.Fatalf("expected revoked status with timestamp, got %+v", revoked)
	}

	listed, err := repo.VoiceSatellites(ctx)
	if err != nil {
		t.Fatalf("VoiceSatellites() error = %v", err)
	}
	statuses := map[string]string{}
	for _, satellite := range listed {
		statuses[satellite.ID] = satellite.Status
	}
	if statuses["sat-offline"] != SatelliteStatusOffline ||
		statuses["sat-misconfigured"] != SatelliteStatusMisconfigured ||
		statuses["sat-revoked"] != SatelliteStatusRevoked {
		t.Fatalf("satellite statuses were not distinct: %+v", statuses)
	}
}

func TestRevokedSatelliteCannotBeReenabledBySettingsUpdate(t *testing.T) {
	repo, _ := openVoiceRepository(t)
	ctx := context.Background()

	satellite, err := repo.SaveSatelliteInstall(ctx, SatelliteRecord{
		ID:                  "sat-revoked",
		DisplayName:         "Revoked",
		Status:              SatelliteStatusPaired,
		CredentialSecretRef: "env:SAT_REVOKED",
	})
	if err != nil {
		t.Fatalf("SaveSatelliteInstall() error = %v", err)
	}
	revoked, err := repo.RevokeSatellite(ctx, satellite.ID)
	if err != nil {
		t.Fatalf("RevokeSatellite() error = %v", err)
	}
	if revoked.Enabled {
		t.Fatalf("revoked satellite should be disabled: %+v", revoked)
	}

	enabled := true
	if _, err := repo.UpdateSatellite(ctx, satellite.ID, SatelliteUpdateRequest{Enabled: &enabled}); err == nil {
		t.Fatal("expected revoked satellite re-enable to fail")
	}
	reloaded, err := repo.VoiceSatellite(ctx, satellite.ID)
	if err != nil {
		t.Fatalf("VoiceSatellite() error = %v", err)
	}
	if reloaded.Status != SatelliteStatusRevoked || reloaded.Enabled {
		t.Fatalf("revoked satellite was re-enabled: %+v", reloaded)
	}
}

func TestPairingSessionExpiresAndCannotBeReused(t *testing.T) {
	repo, db := openVoiceRepository(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 15, 16, 30, 0, 0, time.UTC)

	expiring, err := repo.CreatePairingSession(ctx, PairingSessionCreateRequest{
		ID:                  "pair-expiring",
		PairingCode:         "123456",
		DeviceProfileID:     "kitchen-voice",
		CredentialSecretRef: "env:PAIRING_SECRET",
		ExpiresAt:           base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreatePairingSession(expiring) error = %v", err)
	}
	assertJSONOmits(t, expiring, "123456", "PAIRING_SECRET", "credential", "secret")
	if _, err := repo.ClaimPairingSession(ctx, "pair-expiring", "123456", base.Add(2*time.Minute)); err == nil {
		t.Fatal("expected expired pairing session claim to fail")
	}

	fresh, err := repo.CreatePairingSession(ctx, PairingSessionCreateRequest{
		ID:                  "pair-fresh",
		PairingCode:         "654321",
		DeviceProfileID:     "kitchen-voice",
		CredentialSecretRef: "env:FRESH_PAIRING_SECRET",
		ExpiresAt:           base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreatePairingSession(fresh) error = %v", err)
	}
	if fresh.ClaimedAt != "" {
		t.Fatalf("new pairing session should be unclaimed: %+v", fresh)
	}

	claimed, err := repo.ClaimPairingSession(ctx, "pair-fresh", "654321", base.Add(30*time.Second))
	if err != nil {
		t.Fatalf("ClaimPairingSession() error = %v", err)
	}
	if claimed.ClaimedAt == "" {
		t.Fatalf("expected claimed timestamp: %+v", claimed)
	}
	if _, err := repo.ClaimPairingSession(ctx, "pair-fresh", "654321", base.Add(45*time.Second)); err == nil {
		t.Fatal("expected reused pairing session claim to fail")
	}

	var stored PairingSessionDB
	if err := db.First(&stored, "id = ?", "pair-fresh").Error; err != nil {
		t.Fatalf("load stored pairing session: %v", err)
	}
	if stored.PairingCodeHash == "654321" || strings.Contains(stored.PairingCodeHash, "654321") {
		t.Fatalf("pairing code was stored in raw form: %+v", stored)
	}
	assertJSONOmits(t, pairingSessionProjection(stored), "654321", "FRESH_PAIRING_SECRET", "credential", "secret")
}

func openVoiceRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/voice.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&SettingsDB{},
		&ProviderPackDB{},
		&SatelliteInstallDB{},
		&PairingSessionDB{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return NewRepository(db), db
}

func assertJSONOmits(t *testing.T, value any, forbidden ...string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	payload := string(data)
	for _, needle := range forbidden {
		if strings.Contains(strings.ToLower(payload), strings.ToLower(needle)) {
			t.Fatalf("projection leaked %q in JSON %s", needle, payload)
		}
	}
}
