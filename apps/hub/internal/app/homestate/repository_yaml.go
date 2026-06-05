package homestate

import (
	"context"
	"sync"
)

// Syncer defines the interface needed for homestate config persistence.
type Syncer interface {
	SyncHome(
		ctx context.Context,
		home HomeConfig,
		display any,
		weather WeatherConfig,
		rooms []RoomConfig,
		tiles []TileConfig,
	) error
	HomeConfig(ctx context.Context) (
		HomeConfig, any, WeatherConfig, []RoomConfig, []TileConfig, error,
	)
}

type YAMLRepository struct {
	mu     sync.RWMutex
	syncer Syncer
}

func NewYAMLRepository(syncer Syncer) *YAMLRepository {
	return &YAMLRepository{
		syncer: syncer,
	}
}

func (y *YAMLRepository) SetupStatus(ctx context.Context) (SetupStatus, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	home, _, weather, _, _, err := y.syncer.HomeConfig(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	missing := missingSetupFields(home, weather, true)
	return SetupStatus{
		Complete: len(missing) == 0,
		Missing:  missing,
	}, nil
}

func (y *YAMLRepository) HouseholdSettings(ctx context.Context) (HouseholdSettings, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	home, display, weather, _, _, err := y.syncer.HomeConfig(ctx)
	if err != nil {
		return HouseholdSettings{}, err
	}
	missing := missingSetupFields(home, weather, true)
	return HouseholdSettings{
		Home:    home,
		Display: display,
		Weather: weather,
		Setup: SetupStatus{
			Complete: len(missing) == 0,
			Missing:  missing,
		},
	}, nil
}

func (y *YAMLRepository) SaveHouseholdSettings(
	ctx context.Context,
	settings HouseholdSettings,
) (HouseholdSettings, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	_, display, _, rooms, tiles, err := y.syncer.HomeConfig(ctx)
	if err != nil {
		display = settings.Display
	}
	// Note: We use the display/rooms/tiles loaded from file unless they don't exist yet,
	// and update only the household settings.
	if settings.Display != nil {
		display = settings.Display
	}
	if err := y.syncer.SyncHome(ctx, settings.Home, display, settings.Weather, rooms, tiles); err != nil {
		return HouseholdSettings{}, err
	}
	missing := missingSetupFields(settings.Home, settings.Weather, true)
	return HouseholdSettings{
		Home:    settings.Home,
		Display: display,
		Weather: settings.Weather,
		Setup: SetupStatus{
			Complete: len(missing) == 0,
			Missing:  missing,
		},
	}, nil
}

func (y *YAMLRepository) Rooms(ctx context.Context) ([]RoomConfig, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	_, _, _, rooms, _, err := y.syncer.HomeConfig(ctx)
	if err != nil {
		return nil, err
	}
	return rooms, nil
}

func (y *YAMLRepository) SaveRooms(ctx context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	home, display, weather, _, tiles, err := y.syncer.HomeConfig(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	if err := y.syncer.SyncHome(ctx, home, display, weather, normalized, tiles); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (y *YAMLRepository) Tiles(ctx context.Context) ([]TileConfig, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	_, _, _, _, tiles, err := y.syncer.HomeConfig(ctx)
	if err != nil {
		return nil, err
	}
	return tiles, nil
}

func (y *YAMLRepository) SaveTiles(ctx context.Context, tiles []TileConfig) ([]TileConfig, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	home, display, weather, rooms, _, err := y.syncer.HomeConfig(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	if err := y.syncer.SyncHome(ctx, home, display, weather, rooms, normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}
