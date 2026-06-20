package repository

import (
	"context"
	"sync"
)

type MemoryHomeRepository struct {
	mu          sync.RWMutex
	household   HouseholdSettings
	rooms       []RoomConfig
	tiles       []TileConfig
	connections map[string]AdapterConnection
}

func NewMemoryHomeRepository(setup SetupStatus) *MemoryHomeRepository {
	return &MemoryHomeRepository{
		household: HouseholdSettings{
			Setup: setup,
		},
		connections: map[string]AdapterConnection{},
	}
}

func NewMemoryHomeRepositoryWithConfig(
	home HomeConfig,
	display any,
	rooms []RoomConfig,
	tiles []TileConfig,
) *MemoryHomeRepository {
	return &MemoryHomeRepository{
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

func (m *MemoryHomeRepository) SetupStatus(_ context.Context) (SetupStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.household.Setup, nil
}

func (m *MemoryHomeRepository) HouseholdSettings(_ context.Context) (HouseholdSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.household, nil
}

func (m *MemoryHomeRepository) SaveHouseholdSettings(
	_ context.Context,
	settings HouseholdSettings,
) (HouseholdSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.household = settings
	m.household.Setup = SetupStatus{Complete: true}
	return m.household, nil
}

func (m *MemoryHomeRepository) Rooms(_ context.Context) ([]RoomConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RoomConfig(nil), m.rooms...), nil
}

func (m *MemoryHomeRepository) SaveRooms(_ context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	m.rooms = normalized
	return append([]RoomConfig(nil), m.rooms...), nil
}

func (m *MemoryHomeRepository) Tiles(_ context.Context) ([]TileConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]TileConfig(nil), m.tiles...), nil
}

func (m *MemoryHomeRepository) SaveTiles(_ context.Context, tiles []TileConfig) ([]TileConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	m.tiles = normalized
	return append([]TileConfig(nil), m.tiles...), nil
}

func (m *MemoryHomeRepository) AdapterConnections(_ context.Context) ([]AdapterConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connections := make([]AdapterConnection, 0, len(m.connections))
	for _, connection := range m.connections {
		connections = append(connections, cloneAdapterConnection(connection))
	}
	return connections, nil
}

func (m *MemoryHomeRepository) AdapterConnection(_ context.Context, id string) (AdapterConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connection, ok := m.connections[id]
	if !ok {
		return AdapterConnection{}, ErrInvalidSettings
	}
	return cloneAdapterConnection(connection), nil
}

func (m *MemoryHomeRepository) SaveAdapterConnection(
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
