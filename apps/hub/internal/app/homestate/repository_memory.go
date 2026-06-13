package homestate

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu          sync.RWMutex
	household   HouseholdSettings
	rooms       []RoomConfig
	tiles       []TileConfig
	connections map[string]AdapterConnection
}

func NewMemoryRepository(setup SetupStatus) *MemoryRepository {
	return &MemoryRepository{
		household: HouseholdSettings{
			Setup: setup,
		},
		connections: map[string]AdapterConnection{},
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
		rooms:       append([]RoomConfig(nil), rooms...),
		tiles:       append([]TileConfig(nil), tiles...),
		connections: map[string]AdapterConnection{},
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

func (m *MemoryRepository) AdapterConnections(_ context.Context) ([]AdapterConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connections := make([]AdapterConnection, 0, len(m.connections))
	for _, connection := range m.connections {
		connections = append(connections, cloneAdapterConnection(connection))
	}
	return connections, nil
}

func (m *MemoryRepository) AdapterConnection(_ context.Context, id string) (AdapterConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connection, ok := m.connections[id]
	if !ok {
		return AdapterConnection{}, ErrInvalidSettings
	}
	return cloneAdapterConnection(connection), nil
}

func (m *MemoryRepository) SaveAdapterConnection(
	_ context.Context,
	connection AdapterConnection,
) (AdapterConnection, error) {
	if connection.ID == "" || connection.Kind == "" || connection.Name == "" {
		return AdapterConnection{}, ErrInvalidSettings
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connections == nil {
		m.connections = map[string]AdapterConnection{}
	}
	m.connections[connection.ID] = cloneAdapterConnection(connection)
	return cloneAdapterConnection(m.connections[connection.ID]), nil
}

func cloneAdapterConnection(connection AdapterConnection) AdapterConnection {
	out := connection
	out.Settings = map[string]any{}
	for k, v := range connection.Settings {
		out.Settings[k] = v
	}
	out.SecretRefs = map[string]string{}
	for k, v := range connection.SecretRefs {
		out.SecretRefs[k] = v
	}
	return out
}
