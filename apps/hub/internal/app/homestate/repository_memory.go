package homestate

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	household HouseholdSettings
	rooms     []RoomConfig
	tiles     []TileConfig
}

func NewMemoryRepository(setup SetupStatus) *MemoryRepository {
	return &MemoryRepository{
		household: HouseholdSettings{
			Setup: setup,
		},
	}
}

func NewMemoryRepositoryWithConfig(
	home HomeConfig,
	display any,
	rooms []RoomConfig,
	tiles []TileConfig,
) *MemoryRepository {
	return &MemoryRepository{
		household: HouseholdSettings{
			Home:    home,
			Display: display,
			Setup:   SetupStatus{Complete: true},
		},
		rooms: append([]RoomConfig(nil), rooms...),
		tiles: append([]TileConfig(nil), tiles...),
	}
}

func (m *MemoryRepository) SetupStatus(_ context.Context) (SetupStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.household.Setup, nil
}

func (m *MemoryRepository) HouseholdSettings(_ context.Context) (HouseholdSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.household, nil
}

func (m *MemoryRepository) SaveHouseholdSettings(
	_ context.Context,
	settings HouseholdSettings,
) (HouseholdSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.household = settings
	m.household.Setup = SetupStatus{Complete: true}
	return m.household, nil
}

func (m *MemoryRepository) Rooms(_ context.Context) ([]RoomConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RoomConfig(nil), m.rooms...), nil
}

func (m *MemoryRepository) SaveRooms(_ context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	m.rooms = normalized
	return append([]RoomConfig(nil), m.rooms...), nil
}

func (m *MemoryRepository) Tiles(_ context.Context) ([]TileConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]TileConfig(nil), m.tiles...), nil
}

func (m *MemoryRepository) SaveTiles(_ context.Context, tiles []TileConfig) ([]TileConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	m.tiles = normalized
	return append([]TileConfig(nil), m.tiles...), nil
}
