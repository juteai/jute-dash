package homestate

import (
	"context"
	"errors"
	"sync"
)

type YAMLRepository struct {
	mu         sync.RWMutex
	configPath string
	loadFn     func(path string) (HomeConfig, any, WeatherConfig, []RoomConfig, []TileConfig, error)
	saveFn     func(path string, home HomeConfig, display any, weather WeatherConfig, rooms []RoomConfig, tiles []TileConfig) error
}

func NewYAMLRepository(
	configPath string,
	loadFn func(path string) (HomeConfig, any, WeatherConfig, []RoomConfig, []TileConfig, error),
	saveFn func(path string, home HomeConfig, display any, weather WeatherConfig, rooms []RoomConfig, tiles []TileConfig) error,
) *YAMLRepository {
	return &YAMLRepository{
		configPath: configPath,
		loadFn:     loadFn,
		saveFn:     saveFn,
	}
}

func (y *YAMLRepository) load() (HomeConfig, any, WeatherConfig, []RoomConfig, []TileConfig, error) {
	if y.configPath == "" {
		return HomeConfig{}, nil, WeatherConfig{}, nil, nil, errors.New("config path is empty")
	}
	return y.loadFn(y.configPath)
}

func (y *YAMLRepository) save(
	home HomeConfig,
	display any,
	weather WeatherConfig,
	rooms []RoomConfig,
	tiles []TileConfig,
) error {
	if y.configPath == "" {
		return errors.New("cannot save: config path is empty")
	}
	return y.saveFn(y.configPath, home, display, weather, rooms, tiles)
}

func (y *YAMLRepository) SetupStatus(_ context.Context) (SetupStatus, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	home, _, weather, _, _, err := y.load()
	if err != nil {
		return SetupStatus{}, err
	}
	missing := missingSetupFields(home, weather, true)
	return SetupStatus{
		Complete: len(missing) == 0,
		Missing:  missing,
	}, nil
}

func (y *YAMLRepository) HouseholdSettings(_ context.Context) (HouseholdSettings, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	home, display, weather, _, _, err := y.load()
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
	_ context.Context,
	settings HouseholdSettings,
) (HouseholdSettings, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	_, display, _, rooms, tiles, err := y.load()
	if err != nil {
		display = settings.Display
	}
	// Note: We use the display/rooms/tiles loaded from file unless they don't exist yet,
	// and update only the household settings.
	if settings.Display != nil {
		display = settings.Display
	}
	if err := y.save(settings.Home, display, settings.Weather, rooms, tiles); err != nil {
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

func (y *YAMLRepository) Rooms(_ context.Context) ([]RoomConfig, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	_, _, _, rooms, _, err := y.load()
	if err != nil {
		return nil, err
	}
	return rooms, nil
}

func (y *YAMLRepository) SaveRooms(_ context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	home, display, weather, _, tiles, err := y.load()
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	if err := y.save(home, display, weather, normalized, tiles); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (y *YAMLRepository) Tiles(_ context.Context) ([]TileConfig, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	_, _, _, _, tiles, err := y.load()
	if err != nil {
		return nil, err
	}
	return tiles, nil
}

func (y *YAMLRepository) SaveTiles(_ context.Context, tiles []TileConfig) ([]TileConfig, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	home, display, weather, rooms, _, err := y.load()
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	if err := y.save(home, display, weather, rooms, normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}
