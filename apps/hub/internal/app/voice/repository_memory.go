package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu              sync.RWMutex
	settings        Settings
	satellites      map[string]SatelliteRecord
	pairingSessions map[string]PairingSessionDB
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		satellites:      map[string]SatelliteRecord{},
		pairingSessions: map[string]PairingSessionDB{},
		settings: Settings{
			DeviceProfileID: DefaultDeviceProfileID,
			Muted:           true,
			WakeSensitivity: 0.5,
			TTSLocale:       "en",
			TTSSpeed:        1,
			TTSVolume:       1,
			UpdatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
}

func NewMemoryRepositoryFromConfig(cfg Config) *MemoryRepository {
	return &MemoryRepository{
		satellites:      map[string]SatelliteRecord{},
		pairingSessions: map[string]PairingSessionDB{},
		settings: Settings{
			DeviceProfileID:         DefaultDeviceProfileID,
			Enabled:                 cfg.Enabled,
			Muted:                   cfg.MutedByDefault,
			WakeWordModelID:         cfg.WakeWordModelID,
			WakeWordPhrase:          cfg.WakeWordPhrase,
			WakeSensitivity:         cfg.WakeSensitivity,
			STTProviderID:           cfg.STTProviderID,
			TTSProviderID:           cfg.TTSProviderID,
			STTModelID:              cfg.STTModelID,
			TTSModelID:              cfg.TTSModelID,
			TTSVoiceID:              cfg.TTSVoiceID,
			TTSEnabled:              cfg.TTSEnabled,
			TTSLocale:               cfg.TTSLocale,
			TTSSpeed:                cfg.TTSSpeed,
			TTSVolume:               cfg.TTSVolume,
			PreferredAgentID:        cfg.PreferredAgentID,
			CloudOptIn:              cfg.CloudOptIn,
			CommandProvidersEnabled: cfg.CommandProvidersEnabled,
			SensitiveOutputPolicy:   cfg.SensitiveOutputPolicy,
			FollowupWindowSeconds:   cfg.FollowupWindowSeconds,
			MicrophoneProfile:       cfg.MicrophoneProfile,
			UpdatedAt:               time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
}

func (m *MemoryRepository) VoiceSettings(_ context.Context, _ string) (Settings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings, nil
}

func (m *MemoryRepository) SaveVoiceSettings(_ context.Context, req SettingsUpdateRequest) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := applySettingsUpdate(m.settings, req)
	if problems := validateSettings(next); len(problems) > 0 {
		return Settings{}, fmt.Errorf("invalid voice settings: %s", strings.Join(problems, "; "))
	}
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.settings = next
	return m.settings, nil
}

func (m *MemoryRepository) SetVoiceMuted(_ context.Context, _ string, muted bool) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.Muted = muted
	m.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.settings, nil
}

func (m *MemoryRepository) CancelVoice(_ context.Context, _ string) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.settings, nil
}

func (m *MemoryRepository) VoiceProviders(_ context.Context) ([]ProviderPack, error) {
	return []ProviderPack{}, nil
}

func (m *MemoryRepository) TTSVoices(_ context.Context, providerID, _ string) (TTSVoicesResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if providerID == "" {
		providerID = m.settings.TTSProviderID
	}
	return TTSVoicesResponse{
		ProviderID:      providerID,
		HealthStatus:    "disabled",
		SetupStatus:     "disabled",
		SelectedVoiceID: m.settings.TTSVoiceID,
		SelectedModelID: m.settings.TTSModelID,
		Locale:          m.settings.TTSLocale,
		Speed:           m.settings.TTSSpeed,
		Volume:          m.settings.TTSVolume,
		Voices:          []TTSVoice{},
	}, nil
}

func (m *MemoryRepository) SaveSatelliteInstall(
	_ context.Context,
	record SatelliteRecord,
) (SatelliteProjection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next, err := normalizeSatelliteRecord(record, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return SatelliteProjection{}, err
	}
	m.satellites[next.ID] = next
	return satelliteProjection(next), nil
}

func (m *MemoryRepository) VoiceSatellite(_ context.Context, id string) (SatelliteProjection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.satellites[strings.TrimSpace(id)]
	if !ok {
		return SatelliteProjection{}, fmt.Errorf("load voice satellite install: %w", ErrNotFound)
	}
	return satelliteProjection(record), nil
}

func (m *MemoryRepository) AuthenticateSatellite(_ context.Context, id, authProof string) (SatelliteProjection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.satellites[strings.TrimSpace(id)]
	if !ok {
		return SatelliteProjection{}, fmt.Errorf("load voice satellite install: %w", ErrNotFound)
	}
	if strings.TrimSpace(authProof) == "" || strings.TrimSpace(authProof) != record.CredentialSecretRef {
		return SatelliteProjection{}, errors.New("satellite authentication failed")
	}
	if record.Status == SatelliteStatusRevoked {
		return SatelliteProjection{}, errors.New("satellite is revoked")
	}
	if !record.Enabled {
		return SatelliteProjection{}, errors.New("satellite is disabled")
	}
	return satelliteProjection(record), nil
}

func (m *MemoryRepository) VoiceSatellites(_ context.Context) ([]SatelliteProjection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	satellites := make([]SatelliteProjection, 0, len(m.satellites))
	for _, record := range m.satellites {
		satellites = append(satellites, satelliteProjection(record))
	}
	return satellites, nil
}

func (m *MemoryRepository) RevokeSatellite(_ context.Context, id string) (SatelliteProjection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id = strings.TrimSpace(id)
	record, ok := m.satellites[id]
	if !ok {
		return SatelliteProjection{}, fmt.Errorf("revoke voice satellite: %w", ErrNotFound)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Status = SatelliteStatusRevoked
	record.Enabled = false
	record.RevokedAt = now
	record.UpdatedAt = now
	m.satellites[id] = record
	return satelliteProjection(record), nil
}

func (m *MemoryRepository) UpdateSatellite(
	_ context.Context,
	id string,
	req SatelliteUpdateRequest,
) (SatelliteProjection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id = strings.TrimSpace(id)
	record, ok := m.satellites[id]
	if !ok {
		return SatelliteProjection{}, fmt.Errorf("update voice satellite: %w", ErrNotFound)
	}
	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if name == "" {
			return SatelliteProjection{}, errors.New("satellite display name is required")
		}
		record.DisplayName = name
	}
	if req.RoomLabel != nil {
		record.RoomLabel = strings.TrimSpace(*req.RoomLabel)
	}
	if req.DeviceProfileID != nil {
		deviceProfileID := strings.TrimSpace(*req.DeviceProfileID)
		if deviceProfileID == "" {
			return SatelliteProjection{}, errors.New("satellite device profile ID is required")
		}
		record.DeviceProfileID = deviceProfileID
	}
	if req.Enabled != nil {
		if record.Status == SatelliteStatusRevoked && *req.Enabled {
			return SatelliteProjection{}, errors.New("revoked satellite cannot be re-enabled")
		}
		record.Enabled = *req.Enabled
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if req.Revoke != nil && *req.Revoke {
		record.Status = SatelliteStatusRevoked
		record.Enabled = false
		record.RevokedAt = now
	}
	record.UpdatedAt = now
	m.satellites[id] = record
	return satelliteProjection(record), nil
}

func (m *MemoryRepository) CreatePairingSession(
	_ context.Context,
	req PairingSessionCreateRequest,
) (PairingSessionProjection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, err := pairingSessionDBFromRequest(req, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return PairingSessionProjection{}, err
	}
	m.pairingSessions[row.ID] = row
	return pairingSessionProjection(row), nil
}

func (m *MemoryRepository) ClaimPairingSession(
	_ context.Context,
	id, pairingCode string,
	at time.Time,
) (PairingSessionProjection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id = strings.TrimSpace(id)
	row, ok := m.pairingSessions[id]
	if !ok {
		return PairingSessionProjection{}, fmt.Errorf("load voice satellite pairing session: %w", ErrNotFound)
	}
	if strings.TrimSpace(row.ClaimedAt) != "" {
		return PairingSessionProjection{}, errors.New("pairing session has already been claimed")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, row.ExpiresAt)
	if err != nil {
		return PairingSessionProjection{}, fmt.Errorf("invalid pairing session expiry: %w", err)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if !at.Before(expiresAt) {
		return PairingSessionProjection{}, errors.New("pairing session has expired")
	}
	if hashPairingCode(pairingCode) != row.PairingCodeHash {
		return PairingSessionProjection{}, errors.New("pairing code does not match")
	}
	row.ClaimedAt = at.UTC().Format(time.RFC3339Nano)
	m.pairingSessions[id] = row
	return pairingSessionProjection(row), nil
}
