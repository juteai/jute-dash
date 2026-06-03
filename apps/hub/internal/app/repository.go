package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type WidgetLayoutStore interface {
	WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error)
	SaveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error)
	ResetWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error)
}

type VoiceSettingsStore interface {
	VoiceSettings(ctx context.Context, deviceProfileID string) (VoiceSettings, error)
	SetVoiceMuted(ctx context.Context, deviceProfileID string, muted bool) (VoiceSettings, error)
	CancelVoice(ctx context.Context, deviceProfileIDID string) (VoiceSettings, error)
	VoiceProviders(ctx context.Context) ([]VoiceProviderPack, error)
}

type HouseholdSettingsStore interface {
	HouseholdSettings(ctx context.Context) (HouseholdSettings, error)
	SaveHouseholdSettings(ctx context.Context, settings HouseholdSettings) (HouseholdSettings, error)
	Rooms(ctx context.Context) ([]RoomConfig, error)
	SaveRooms(ctx context.Context, rooms []RoomConfig) ([]RoomConfig, error)
	Tiles(ctx context.Context) ([]TileConfig, error)
	SaveTiles(ctx context.Context, tiles []TileConfig) ([]TileConfig, error)
}

type Store struct {
	db      *gorm.DB
	path    string
	catalog map[string]WidgetCatalogItem
}

func Open(dbPath string) (*Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite via gorm: %w", err)
	}

	store := &Store{db: db, path: dbPath, catalog: widgetCatalogByKind()}
	if err := store.configure(context.Background()); err != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	return store, nil
}

func (s *Store) SetCatalog(items []WidgetCatalogItem) {
	m := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		m[item.Kind] = item
	}
	s.catalog = m
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Initialize(ctx context.Context, bootstrap Config, bootstrapProvided bool) (InitResult, error) {
	if err := s.Migrate(ctx); err != nil {
		return InitResult{}, err
	}

	seeded, err := s.isSeeded(ctx)
	if err != nil {
		return InitResult{}, err
	}

	result := InitResult{}
	if !seeded {
		if err := s.seed(ctx, bootstrap, bootstrapProvided); err != nil {
			return InitResult{}, err
		}
		result.Seeded = true
	}
	if err := s.ensureDefaultVoiceSettings(ctx, DefaultConfig().Voice); err != nil {
		return InitResult{}, err
	}

	cfg, err := s.Config(ctx)
	if err != nil {
		return InitResult{}, err
	}
	status, err := s.SetupStatus(ctx)
	if err != nil {
		return InitResult{}, err
	}
	result.Config = cfg
	result.Setup = status
	return result, nil
}

func (s *Store) NeedsSeed(ctx context.Context) (bool, error) {
	if err := s.Migrate(ctx); err != nil {
		return false, err
	}
	seeded, err := s.isSeeded(ctx)
	if err != nil {
		return false, err
	}
	return !seeded, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(
		&HouseholdSettingsDB{},
		&WeatherSettingsDB{},
		&DeviceProfileDB{},
		&LayoutProfileDB{},
		&RoomDB{},
		&TileDB{},
		&WidgetPackDB{},
		&WidgetInstanceDB{},
		&WidgetPermissionDB{},
		&VoiceProviderPackDB{},
		&VoiceSettingsDB{},
		&AdapterConnectionDB{},
		&SettingAuditLogDB{},
	)
}

func (s *Store) Config(ctx context.Context) (Config, error) {
	cfg := DefaultConfig()

	var hs HouseholdSettingsDB
	if err := s.db.WithContext(ctx).First(&hs, "id = ?", defaultHouseholdID).Error; err != nil {
		return Config{}, fmt.Errorf("load household settings: %w", err)
	}
	cfg.Home.Name = hs.Name
	cfg.Home.Timezone = hs.Timezone
	cfg.Home.Locale = hs.Locale
	cfg.Display.Theme = hs.DisplayTheme
	cfg.Display.ColorMode = hs.DisplayColorMode
	cfg.Display.ThemeID = hs.DisplayThemeID
	cfg.Display.Density = hs.DisplayDensity
	cfg.Display.Motion = hs.DisplayMotion
	cfg.Display.AccentColor = hs.DisplayAccentColor
	cfg.Display.IdleMode = hs.DisplayIdleMode

	if err := decodeJSONSetting(hs.DisplayBackgroundJSON, &cfg.Display.Background); err != nil {
		return Config{}, fmt.Errorf("decode display background: %w", err)
	}
	if err := decodeJSONSetting(hs.DisplayWidgetChromeJSON, &cfg.Display.WidgetChrome); err != nil {
		return Config{}, fmt.Errorf("decode display widget chrome: %w", err)
	}

	var ws WeatherSettingsDB
	if err := s.db.WithContext(ctx).First(&ws, "id = ?", defaultHouseholdID).Error; err != nil {
		return Config{}, fmt.Errorf("load weather settings: %w", err)
	}
	cfg.Weather.Enabled = ws.Enabled == 1
	cfg.Weather.Provider = ws.Provider
	cfg.Weather.LocationName = ws.LocationName
	cfg.Weather.Latitude = ws.Latitude
	cfg.Weather.Longitude = ws.Longitude
	cfg.Weather.TemperatureUnit = ws.TemperatureUnit
	cfg.Weather.WindSpeedUnit = ws.WindSpeedUnit

	voiceSettings, err := s.VoiceSettings(ctx, defaultDeviceProfileID)
	if err != nil {
		return Config{}, err
	}
	cfg.Voice = VoiceConfig{
		Enabled:                 voiceSettings.Enabled,
		MutedByDefault:          voiceSettings.Muted,
		WakeWordModelID:         voiceSettings.WakeWordModelID,
		STTProviderID:           voiceSettings.STTProviderID,
		TTSProviderID:           voiceSettings.TTSProviderID,
		STTModelID:              voiceSettings.STTModelID,
		TTSModelID:              voiceSettings.TTSModelID,
		TTSVoiceID:              voiceSettings.TTSVoiceID,
		PreferredAgentID:        voiceSettings.PreferredAgentID,
		CloudOptIn:              voiceSettings.CloudOptIn,
		CommandProvidersEnabled: voiceSettings.CommandProvidersEnabled,
		SensitiveOutputPolicy:   voiceSettings.SensitiveOutputPolicy,
		FollowupWindowSeconds:   voiceSettings.FollowupWindowSeconds,
		MicrophoneProfile:       voiceSettings.MicrophoneProfile,
	}

	rooms, err := s.loadRooms(ctx)
	if err != nil {
		return Config{}, err
	}
	tiles, err := s.loadTiles(ctx)
	if err != nil {
		return Config{}, err
	}

	cfg.Agents = nil
	cfg.Rooms = rooms
	cfg.Tiles = tiles
	if err := EnsureValidConfig(&cfg); err != nil {
		return Config{}, fmt.Errorf("validate store config: %w", err)
	}
	return cfg, nil
}

func (s *Store) SetupStatus(ctx context.Context) (SetupStatus, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	var hs HouseholdSettingsDB
	if err := s.db.WithContext(ctx).First(&hs, "id = ?", defaultHouseholdID).Error; err != nil {
		return SetupStatus{}, fmt.Errorf("load setup status: %w", err)
	}
	missing := missingSetupFields(cfg, hs.SetupCompleted == 1)
	if missing == nil {
		missing = []string{}
	}
	return SetupStatus{
		Complete: hs.SetupCompleted == 1 && len(missing) == 0,
		Missing:  missing,
	}, nil
}

func (s *Store) HouseholdSettings(ctx context.Context) (HouseholdSettings, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return HouseholdSettings{}, err
	}
	setup, err := s.SetupStatus(ctx)
	if err != nil {
		return HouseholdSettings{}, err
	}
	return HouseholdSettings{
		Home:    cfg.Home,
		Display: cfg.Display,
		Weather: cfg.Weather,
		Setup:   setup,
	}, nil
}

func (s *Store) Rooms(ctx context.Context) ([]RoomConfig, error) {
	return s.loadRooms(ctx)
}

func (s *Store) SaveRooms(ctx context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	now := nowUTC()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM rooms").Error; err != nil {
			return fmt.Errorf("clear rooms: %w", err)
		}
		for i, room := range normalized {
			rDB := RoomDB{
				ID:        room.ID,
				Name:      room.Name,
				Summary:   room.Summary,
				Status:    room.Status,
				SortOrder: i,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&rDB).Error; err != nil {
				return fmt.Errorf("seed room %s: %w", room.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.loadRooms(ctx)
}

func (s *Store) Tiles(ctx context.Context) ([]TileConfig, error) {
	return s.loadTiles(ctx)
}

func (s *Store) SaveTiles(ctx context.Context, tiles []TileConfig) ([]TileConfig, error) {
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	now := nowUTC()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM tiles").Error; err != nil {
			return fmt.Errorf("clear tiles: %w", err)
		}
		for i, tile := range normalized {
			tDB := TileDB{
				ID:        tile.ID,
				Kind:      tile.Kind,
				Label:     tile.Label,
				Value:     tile.Value,
				Detail:    tile.Detail,
				SortOrder: i,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&tDB).Error; err != nil {
				return fmt.Errorf("seed tile %s: %w", tile.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.loadTiles(ctx)
}

func (s *Store) SaveHouseholdSettings(ctx context.Context, settings HouseholdSettings) (HouseholdSettings, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return HouseholdSettings{}, err
	}
	cfg.Home = settings.Home
	cfg.Display = settings.Display
	cfg.Weather = settings.Weather
	if err := EnsureValidConfig(&cfg); err != nil {
		return HouseholdSettings{}, err
	}
	backgroundJSON, err := jsonString(cfg.Display.Background)
	if err != nil {
		return HouseholdSettings{}, fmt.Errorf("encode display background: %w", err)
	}
	widgetChromeJSON, err := jsonString(cfg.Display.WidgetChrome)
	if err != nil {
		return HouseholdSettings{}, fmt.Errorf("encode display widget chrome: %w", err)
	}

	now := nowUTC()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var hs HouseholdSettingsDB
		if err := tx.First(&hs, "id = ?", defaultHouseholdID).Error; err != nil {
			return err
		}
		hs.Name = cfg.Home.Name
		hs.Timezone = cfg.Home.Timezone
		hs.Locale = cfg.Home.Locale
		hs.DisplayTheme = cfg.Display.Theme
		hs.DisplayColorMode = cfg.Display.ColorMode
		hs.DisplayThemeID = cfg.Display.ThemeID
		hs.DisplayDensity = cfg.Display.Density
		hs.DisplayMotion = cfg.Display.Motion
		hs.DisplayBackgroundJSON = backgroundJSON
		hs.DisplayWidgetChromeJSON = widgetChromeJSON
		hs.DisplayAccentColor = cfg.Display.AccentColor
		hs.DisplayIdleMode = cfg.Display.IdleMode
		hs.SetupCompleted = 1
		hs.UpdatedAt = now

		if err := tx.Save(&hs).Error; err != nil {
			return err
		}

		var ws WeatherSettingsDB
		if err := tx.First(&ws, "id = ?", defaultHouseholdID).Error; err != nil {
			return err
		}
		ws.Enabled = boolToInt(cfg.Weather.Enabled)
		ws.Provider = cfg.Weather.Provider
		ws.LocationName = cfg.Weather.LocationName
		ws.Latitude = cfg.Weather.Latitude
		ws.Longitude = cfg.Weather.Longitude
		ws.TemperatureUnit = cfg.Weather.TemperatureUnit
		ws.WindSpeedUnit = cfg.Weather.WindSpeedUnit
		ws.UpdatedAt = now

		if err := tx.Save(&ws).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return HouseholdSettings{}, fmt.Errorf("save settings transaction: %w", err)
	}

	return s.HouseholdSettings(ctx)
}

func (s *Store) loadRooms(ctx context.Context) ([]RoomConfig, error) {
	var dbRooms []RoomDB
	if err := s.db.WithContext(ctx).Order("sort_order").Find(&dbRooms).Error; err != nil {
		return nil, fmt.Errorf("load rooms: %w", err)
	}
	rooms := make([]RoomConfig, len(dbRooms))
	for i, r := range dbRooms {
		rooms[i] = RoomConfig{
			ID:      r.ID,
			Name:    r.Name,
			Summary: r.Summary,
			Status:  r.Status,
		}
	}
	return rooms, nil
}

func (s *Store) loadTiles(ctx context.Context) ([]TileConfig, error) {
	var dbTiles []TileDB
	if err := s.db.WithContext(ctx).Order("sort_order").Find(&dbTiles).Error; err != nil {
		return nil, fmt.Errorf("load tiles: %w", err)
	}
	tiles := make([]TileConfig, len(dbTiles))
	for i, t := range dbTiles {
		tiles[i] = TileConfig{
			ID:     t.ID,
			Kind:   t.Kind,
			Label:  t.Label,
			Value:  t.Value,
			Detail: t.Detail,
		}
	}
	return tiles, nil
}


func (s *Store) WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = defaultLayoutProfileID
	}

	var wDBs []WidgetInstanceDB
	if err := s.db.WithContext(ctx).Where("layout_profile_id = ?", profileID).Order("sort_order, id").Find(&wDBs).Error; err != nil {
		return WidgetLayout{}, fmt.Errorf("load widget layout: %w", err)
	}

	layout := WidgetLayout{ProfileID: profileID, Widgets: []WidgetInstance{}}
	for _, w := range wDBs {
		widget := WidgetInstance{
			ID:      w.ID,
			Kind:    w.Kind,
			Title:   w.Title,
			X:       w.X,
			Y:       w.Y,
			W:       w.W,
			H:       w.H,
			MinW:    w.MinW,
			MinH:    w.MinH,
			Size:    w.Size,
			Visible: w.Visible == 1,
		}
		widget.Settings = map[string]any{}
		if strings.TrimSpace(w.SettingsJSON) != "" {
			if err := json.Unmarshal([]byte(w.SettingsJSON), &widget.Settings); err != nil {
				return WidgetLayout{}, fmt.Errorf("decode widget settings for %s: %w", widget.ID, err)
			}
		}
		if item, ok := s.catalog[widget.Kind]; ok {
			widget.Overflow = item.Overflow
		}
		layout.Widgets = append(layout.Widgets, widget)
	}
	return layout, nil
}

func (s *Store) VoiceSettings(ctx context.Context, deviceProfileID string) (VoiceSettings, error) {
	deviceProfileID = strings.TrimSpace(deviceProfileID)
	if deviceProfileID == "" {
		deviceProfileID = defaultDeviceProfileID
	}

	var vs VoiceSettingsDB
	if err := s.db.WithContext(ctx).First(&vs, "device_profile_id = ?", deviceProfileID).Error; err != nil {
		return VoiceSettings{}, fmt.Errorf("load voice settings: %w", err)
	}

	return VoiceSettings{
		DeviceProfileID:         vs.DeviceProfileID,
		Enabled:                 vs.Enabled == 1,
		Muted:                   vs.Muted == 1,
		WakeWordModelID:         vs.WakeWordModelID,
		STTProviderID:           vs.STTProviderID,
		TTSProviderID:           vs.TTSProviderID,
		STTModelID:              vs.STTModelID,
		TTSModelID:              vs.TTSModelID,
		TTSVoiceID:              vs.TTSVoiceID,
		PreferredAgentID:        vs.PreferredAgentID,
		CloudOptIn:              vs.CloudOptIn == 1,
		CommandProvidersEnabled: vs.CommandProvidersEnabled == 1,
		SensitiveOutputPolicy:   vs.SensitiveOutputPolicy,
		FollowupWindowSeconds:   vs.FollowupWindowSeconds,
		MicrophoneProfile:       vs.MicrophoneProfile,
		UpdatedAt:               vs.UpdatedAt,
	}, nil
}

func (s *Store) SetVoiceMuted(ctx context.Context, deviceProfileID string, muted bool) (VoiceSettings, error) {
	deviceProfileID = strings.TrimSpace(deviceProfileID)
	if deviceProfileID == "" {
		deviceProfileID = defaultDeviceProfileID
	}
	now := nowUTC()

	err := s.db.WithContext(ctx).Model(&VoiceSettingsDB{}).Where("device_profile_id = ?", deviceProfileID).Updates(map[string]any{
		"muted":                 boolToInt(muted),
		"last_state_updated_at": now,
		"updated_at":            now,
	}).Error
	if err != nil {
		return VoiceSettings{}, fmt.Errorf("update voice mute state: %w", err)
	}
	return s.VoiceSettings(ctx, deviceProfileID)
}

func (s *Store) CancelVoice(ctx context.Context, deviceProfileID string) (VoiceSettings, error) {
	deviceProfileID = strings.TrimSpace(deviceProfileID)
	if deviceProfileID == "" {
		deviceProfileID = defaultDeviceProfileID
	}
	now := nowUTC()

	err := s.db.WithContext(ctx).Model(&VoiceSettingsDB{}).Where("device_profile_id = ?", deviceProfileID).Updates(map[string]any{
		"last_state_updated_at": now,
		"updated_at":            now,
	}).Error
	if err != nil {
		return VoiceSettings{}, fmt.Errorf("cancel voice state: %w", err)
	}
	return s.VoiceSettings(ctx, deviceProfileID)
}

func (s *Store) VoiceProviders(ctx context.Context) ([]VoiceProviderPack, error) {
	var vpps []VoiceProviderPackDB
	if err := s.db.WithContext(ctx).Order("name, id").Find(&vpps).Error; err != nil {
		return nil, fmt.Errorf("load voice providers: %w", err)
	}

	providers := make([]VoiceProviderPack, len(vpps))
	for i, v := range vpps {
		providers[i] = VoiceProviderPack{
			ID:            v.ID,
			Name:          v.Name,
			Version:       v.Version,
			Kind:          v.Kind,
			TransportType: v.TransportType,
			HealthStatus:  v.HealthStatus,
			UpdatedAt:     v.UpdatedAt,
		}
	}
	return providers, nil
}

func (s *Store) SaveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error) {
	normalized, err := NormalizeWidgetLayout(layout, s.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&LayoutProfileDB{}).Where("id = ?", normalized.ProfileID).Count(&count).Error; err != nil {
		return WidgetLayout{}, fmt.Errorf("check layout profile: %w", err)
	}
	if count == 0 {
		return WidgetLayout{}, fmt.Errorf("%w: layout profile not found", ErrInvalidLayout)
	}

	now := nowUTC()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("layout_profile_id = ?", normalized.ProfileID).Delete(&WidgetInstanceDB{}).Error; err != nil {
			return fmt.Errorf("clear widget layout: %w", err)
		}

		for i, widget := range normalized.Widgets {
			settingsJSON, err := jsonString(widget.Settings)
			if err != nil {
				return fmt.Errorf("encode widget settings for %s: %w", widget.ID, err)
			}
			wDB := WidgetInstanceDB{
				ID:              widget.ID,
				Kind:            widget.Kind,
				Title:           widget.Title,
				LayoutProfileID: normalized.ProfileID,
				X:               widget.X,
				Y:               widget.Y,
				W:               widget.W,
				H:               widget.H,
				MinW:            widget.MinW,
				MinH:            widget.MinH,
				Size:            widget.Size,
				SettingsJSON:    settingsJSON,
				Visible:         boolToInt(widget.Visible),
				SortOrder:       i,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := tx.Create(&wDB).Error; err != nil {
				return fmt.Errorf("save widget %s: %w", widget.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return WidgetLayout{}, err
	}

	return normalized, nil
}

func (s *Store) ResetWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	layout := DefaultWidgetLayout()
	if strings.TrimSpace(profileID) != "" {
		layout.ProfileID = strings.TrimSpace(profileID)
	}
	return s.SaveWidgetLayout(ctx, layout)
}

func (s *Store) configure(ctx context.Context) error {
	db, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("set sqlite WAL mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	return nil
}

func (s *Store) isSeeded(ctx context.Context) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&HouseholdSettingsDB{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check seeded store: %w", err)
	}
	return count > 0, nil
}

func (s *Store) seed(ctx context.Context, cfg Config, bootstrapProvided bool) error {
	if err := EnsureValidConfig(&cfg); err != nil {
		return fmt.Errorf("validate seed config: %w", err)
	}

	setup := setupStatusForSeed(cfg, bootstrapProvided)
	now := nowUTC()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hs := HouseholdSettingsDB{
			ID:                      defaultHouseholdID,
			Name:                    cfg.Home.Name,
			Timezone:                cfg.Home.Timezone,
			Locale:                  cfg.Home.Locale,
			DisplayTheme:            cfg.Display.Theme,
			DisplayAccentColor:      cfg.Display.AccentColor,
			DisplayIdleMode:         cfg.Display.IdleMode,
			SetupCompleted:          boolToInt(setup.Complete),
			CreatedAt:               now,
			UpdatedAt:               now,
			DisplayColorMode:        cfg.Display.ColorMode,
			DisplayThemeID:          cfg.Display.ThemeID,
			DisplayDensity:          cfg.Display.Density,
			DisplayMotion:           cfg.Display.Motion,
			DisplayBackgroundJSON:   mustJSONString(cfg.Display.Background),
			DisplayWidgetChromeJSON: mustJSONString(cfg.Display.WidgetChrome),
		}
		if err := tx.Create(&hs).Error; err != nil {
			return fmt.Errorf("seed household settings: %w", err)
		}

		ws := WeatherSettingsDB{
			ID:              defaultHouseholdID,
			Enabled:         boolToInt(cfg.Weather.Enabled),
			Provider:        cfg.Weather.Provider,
			LocationName:    cfg.Weather.LocationName,
			Latitude:        cfg.Weather.Latitude,
			Longitude:       cfg.Weather.Longitude,
			TemperatureUnit: cfg.Weather.TemperatureUnit,
			WindSpeedUnit:   cfg.Weather.WindSpeedUnit,
			UpdatedAt:       now,
		}
		if err := tx.Create(&ws).Error; err != nil {
			return fmt.Errorf("seed weather settings: %w", err)
		}

		dp := DeviceProfileDB{
			ID:              defaultDeviceProfileID,
			Name:            "Default Display",
			InteractionMode: "touch",
			LayoutProfileID: defaultLayoutProfileID,
			SettingsJSON:    "{}",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&dp).Error; err != nil {
			return fmt.Errorf("seed device profile: %w", err)
		}

		lp := LayoutProfileDB{
			ID:              defaultLayoutProfileID,
			DeviceProfileID: defaultDeviceProfileID,
			Name:            "Default Dashboard",
			SettingsJSON:    "{}",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&lp).Error; err != nil {
			return fmt.Errorf("seed layout profile: %w", err)
		}

		for i, room := range cfg.Rooms {
			rDB := RoomDB{
				ID:        room.ID,
				Name:      room.Name,
				Summary:   room.Summary,
				Status:    room.Status,
				SortOrder: i,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&rDB).Error; err != nil {
				return fmt.Errorf("seed room %s: %w", room.ID, err)
			}
		}

		for i, tile := range cfg.Tiles {
			tDB := TileDB{
				ID:        tile.ID,
				Kind:      tile.Kind,
				Label:     tile.Label,
				Value:     tile.Value,
				Detail:    tile.Detail,
				SortOrder: i,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&tDB).Error; err != nil {
				return fmt.Errorf("seed tile %s: %w", tile.ID, err)
			}
		}

		for i, widget := range defaultWidgetInstances() {
			wDB := WidgetInstanceDB{
				ID:              widget.id,
				Kind:            widget.kind,
				Title:           widget.title,
				LayoutProfileID: defaultLayoutProfileID,
				X:               widget.x,
				Y:               widget.y,
				W:               widget.w,
				H:               widget.h,
				MinW:            widget.minW,
				MinH:            widget.minH,
				Size:            widget.size,
				SettingsJSON:    "{}",
				Visible:         boolToInt(widget.visible),
				SortOrder:       i,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := tx.Create(&wDB).Error; err != nil {
				return fmt.Errorf("seed widget %s: %w", widget.id, err)
			}
		}

		vDB := VoiceSettingsDB{
			DeviceProfileID:         defaultDeviceProfileID,
			Enabled:                 boolToInt(cfg.Voice.Enabled),
			Muted:                   boolToInt(cfg.Voice.MutedByDefault),
			WakeWordModelID:         cfg.Voice.WakeWordModelID,
			STTProviderID:           cfg.Voice.STTProviderID,
			TTSProviderID:           cfg.Voice.TTSProviderID,
			STTModelID:              cfg.Voice.STTModelID,
			TTSModelID:              cfg.Voice.TTSModelID,
			TTSVoiceID:              cfg.Voice.TTSVoiceID,
			PreferredAgentID:        cfg.Voice.PreferredAgentID,
			CloudOptIn:              boolToInt(cfg.Voice.CloudOptIn),
			CommandProvidersEnabled: boolToInt(cfg.Voice.CommandProvidersEnabled),
			SensitiveOutputPolicy:   cfg.Voice.SensitiveOutputPolicy,
			FollowupWindowSeconds:   cfg.Voice.FollowupWindowSeconds,
			MicrophoneProfile:       cfg.Voice.MicrophoneProfile,
			UpdatedAt:               now,
		}
		if err := tx.Create(&vDB).Error; err != nil {
			return fmt.Errorf("seed voice settings: %w", err)
		}

		return nil
	})
}

func (s *Store) ensureDefaultVoiceSettings(ctx context.Context, voice VoiceConfig) error {
	now := nowUTC()
	var count int64
	if err := s.db.WithContext(ctx).Model(&VoiceSettingsDB{}).Where("device_profile_id = ?", defaultDeviceProfileID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	vDB := VoiceSettingsDB{
		DeviceProfileID:         defaultDeviceProfileID,
		Enabled:                 boolToInt(voice.Enabled),
		Muted:                   boolToInt(voice.MutedByDefault),
		WakeWordModelID:         voice.WakeWordModelID,
		STTProviderID:           voice.STTProviderID,
		TTSProviderID:           voice.TTSProviderID,
		STTModelID:              voice.STTModelID,
		TTSModelID:              voice.TTSModelID,
		TTSVoiceID:              voice.TTSVoiceID,
		PreferredAgentID:        voice.PreferredAgentID,
		CloudOptIn:              boolToInt(voice.CloudOptIn),
		CommandProvidersEnabled: boolToInt(voice.CommandProvidersEnabled),
		SensitiveOutputPolicy:   voice.SensitiveOutputPolicy,
		FollowupWindowSeconds:   voice.FollowupWindowSeconds,
		MicrophoneProfile:       voice.MicrophoneProfile,
		UpdatedAt:               now,
	}
	return s.db.WithContext(ctx).Create(&vDB).Error
}

func NormalizeWidgetLayout(layout WidgetLayout, catalog map[string]WidgetCatalogItem) (WidgetLayout, error) {
	layout.ProfileID = strings.TrimSpace(layout.ProfileID)
	if layout.ProfileID == "" {
		return WidgetLayout{}, fmt.Errorf("%w: profileId is required", ErrInvalidLayout)
	}
	if layout.Widgets == nil {
		layout.Widgets = []WidgetInstance{}
	}

	seenIDs := map[string]bool{}
	seenKinds := map[string]bool{}

	for i := range layout.Widgets {
		widget := &layout.Widgets[i]
		widget.ID = strings.TrimSpace(widget.ID)
		widget.Kind = strings.TrimSpace(widget.Kind)
		widget.Title = strings.TrimSpace(widget.Title)
		widget.Size = strings.TrimSpace(widget.Size)

		item, ok := catalog[widget.Kind]
		if catalog != nil && !ok {
			return WidgetLayout{}, fmt.Errorf("%w: unknown widget kind %q", ErrInvalidLayout, widget.Kind)
		}
		if widget.ID == "" {
			return WidgetLayout{}, fmt.Errorf("%w: widget id is required", ErrInvalidLayout)
		}
		if seenIDs[widget.ID] {
			return WidgetLayout{}, fmt.Errorf("%w: duplicate widget id %q", ErrInvalidLayout, widget.ID)
		}
		seenIDs[widget.ID] = true
		if ok {
			if !item.AllowMultiple && seenKinds[widget.Kind] {
				return WidgetLayout{}, fmt.Errorf("%w: duplicate widget kind %q", ErrInvalidLayout, widget.Kind)
			}
			if widget.Title == "" {
				widget.Title = item.DefaultTitle
			}
			if widget.Size == "" {
				widget.Size = item.DefaultSize
			}
			if widget.Overflow == "" {
				widget.Overflow = item.Overflow
			}
			if widget.MinW < item.MinW {
				widget.MinW = item.MinW
			}
			if widget.MinH < item.MinH {
				widget.MinH = item.MinH
			}
		}
		seenKinds[widget.Kind] = true
		if err := validateWidgetInstance(*widget); err != nil {
			return WidgetLayout{}, err
		}
		if widget.Settings == nil {
			widget.Settings = map[string]any{}
		}
		if _, err := json.Marshal(widget.Settings); err != nil {
			return WidgetLayout{}, fmt.Errorf(
				"%w: widget %s settings are not JSON serializable",
				ErrInvalidLayout,
				widget.ID,
			)
		}
	}
	return layout, nil
}

func DefaultWidgetLayout() WidgetLayout {
	widgets := defaultWidgetInstances()
	layout := WidgetLayout{
		ProfileID: defaultLayoutProfileID,
		Widgets:   make([]WidgetInstance, 0, len(widgets)),
	}
	for _, widget := range widgets {
		layout.Widgets = append(layout.Widgets, WidgetInstance{
			ID:       widget.id,
			Kind:     widget.kind,
			Title:    widget.title,
			X:        widget.x,
			Y:        widget.y,
			W:        widget.w,
			H:        widget.h,
			MinW:     widget.minW,
			MinH:     widget.minH,
			Size:     widget.size,
			Overflow: widget.overflow,
			Settings: map[string]any{},
			Visible:  widget.visible,
		})
	}
	return layout
}

func WidgetCatalog() []WidgetCatalogItem {
	return []WidgetCatalogItem{
		{
			Kind:          "date-time",
			Name:          "Date & Time",
			Description:   "Clock, date, timezone, and local display timing.",
			DefaultTitle:  "Date & Time",
			DefaultW:      2,
			DefaultH:      1,
			MinW:          1,
			MinH:          1,
			DefaultSize:   "wide",
			Overflow:      "clip",
			AllowMultiple: false,
		},
		{
			Kind:          "weather",
			Name:          "Weather",
			Description:   "Current weather from the configured hub weather provider.",
			DefaultTitle:  "Weather",
			DefaultW:      2,
			DefaultH:      1,
			MinW:          1,
			MinH:          1,
			DefaultSize:   "wide",
			Overflow:      "clip",
			AllowMultiple: false,
		},
		{
			Kind:          "chat-history",
			Name:          "Chat History",
			Description:   "Recent in-memory chat turns and active agent status.",
			DefaultTitle:  "Chat History",
			DefaultW:      2,
			DefaultH:      2,
			MinW:          1,
			MinH:          1,
			DefaultSize:   "medium",
			Overflow:      "scroll",
			AllowMultiple: false,
		},
	}
}

func widgetCatalogByKind() map[string]WidgetCatalogItem {
	items := WidgetCatalog()
	byKind := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		byKind[item.Kind] = item
	}
	return byKind
}

func validateWidgetInstance(widget WidgetInstance) error {
	if widget.X < 0 || widget.Y < 0 {
		return fmt.Errorf("%w: widget %s position must be non-negative", ErrInvalidLayout, widget.ID)
	}
	if widget.W < 1 || widget.H < 1 || widget.MinW < 1 || widget.MinH < 1 {
		return fmt.Errorf("%w: widget %s dimensions must be positive", ErrInvalidLayout, widget.ID)
	}
	if widget.W < widget.MinW || widget.H < widget.MinH {
		return fmt.Errorf("%w: widget %s is smaller than its minimum size", ErrInvalidLayout, widget.ID)
	}
	if widget.W > 4 || widget.MinW > 4 || widget.X+widget.W > 4 {
		return fmt.Errorf("%w: widget %s exceeds dashboard column bounds", ErrInvalidLayout, widget.ID)
	}
	if widget.H > 6 || widget.MinH > 6 || widget.Y > 99 {
		return fmt.Errorf("%w: widget %s exceeds dashboard row bounds", ErrInvalidLayout, widget.ID)
	}
	switch widget.Size {
	case "small", "medium", "wide", "large":
		return nil
	default:
		return fmt.Errorf("%w: widget %s has unsupported size %q", ErrInvalidLayout, widget.ID, widget.Size)
	}
}

func normalizeRooms(rooms []RoomConfig) ([]RoomConfig, error) {
	normalized := make([]RoomConfig, 0, len(rooms))
	seen := map[string]struct{}{}
	for _, room := range rooms {
		room.ID = normalizeID(room.ID)
		room.Name = strings.TrimSpace(room.Name)
		room.Summary = strings.TrimSpace(room.Summary)
		room.Status = strings.TrimSpace(room.Status)
		if room.ID == "" {
			return nil, fmt.Errorf("%w: room id is required", ErrInvalidSettings)
		}
		if _, ok := seen[room.ID]; ok {
			return nil, fmt.Errorf("%w: duplicate room id", ErrInvalidSettings)
		}
		if room.Name == "" {
			return nil, fmt.Errorf("%w: room name is required", ErrInvalidSettings)
		}
		seen[room.ID] = struct{}{}
		normalized = append(normalized, room)
	}
	return normalized, nil
}

func NormalizeRooms(rooms []RoomConfig) ([]RoomConfig, error) {
	return normalizeRooms(rooms)
}

func normalizeTiles(tiles []TileConfig) ([]TileConfig, error) {
	normalized := make([]TileConfig, 0, len(tiles))
	seen := map[string]struct{}{}
	for _, tile := range tiles {
		tile.ID = normalizeID(tile.ID)
		tile.Kind = normalizeID(tile.Kind)
		tile.Label = strings.TrimSpace(tile.Label)
		tile.Value = strings.TrimSpace(tile.Value)
		tile.Detail = strings.TrimSpace(tile.Detail)
		if tile.ID == "" {
			return nil, fmt.Errorf("%w: tile id is required", ErrInvalidSettings)
		}
		if _, ok := seen[tile.ID]; ok {
			return nil, fmt.Errorf("%w: duplicate tile id", ErrInvalidSettings)
		}
		if tile.Kind == "" {
			return nil, fmt.Errorf("%w: tile kind is required", ErrInvalidSettings)
		}
		if tile.Label == "" {
			return nil, fmt.Errorf("%w: tile label is required", ErrInvalidSettings)
		}
		if tile.Value == "" {
			return nil, fmt.Errorf("%w: tile value is required", ErrInvalidSettings)
		}
		seen[tile.ID] = struct{}{}
		normalized = append(normalized, tile)
	}
	return normalized, nil
}

func NormalizeTiles(tiles []TileConfig) ([]TileConfig, error) {
	return normalizeTiles(tiles)
}

func normalizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func setupStatusForSeed(cfg Config, bootstrapProvided bool) SetupStatus {
	missing := missingSetupFields(cfg, bootstrapProvided)
	if missing == nil {
		missing = []string{}
	}
	return SetupStatus{
		Complete: bootstrapProvided && len(missing) == 0,
		Missing:  missing,
	}
}

func missingSetupFields(cfg Config, confirmed bool) []string {
	if !confirmed {
		missing := []string{"home.name", "home.timezone", "home.locale"}
		if cfg.Weather.Enabled {
			missing = append(missing, "weather.location")
		}
		return missing
	}

	var missing []string
	if strings.TrimSpace(cfg.Home.Name) == "" {
		missing = append(missing, "home.name")
	}
	if strings.TrimSpace(cfg.Home.Timezone) == "" {
		missing = append(missing, "home.timezone")
	}
	if strings.TrimSpace(cfg.Home.Locale) == "" {
		missing = append(missing, "home.locale")
	}
	if cfg.Weather.Enabled && strings.TrimSpace(cfg.Weather.LocationName) == "" {
		missing = append(missing, "weather.location")
	}
	return missing
}

func jsonString(value any) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func mustJSONString(value any) string {
	result, err := jsonString(value)
	if err != nil {
		panic(err)
	}
	return result
}

func decodeJSONSetting(raw string, target any) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

type defaultWidgetInstance struct {
	id       string
	kind     string
	title    string
	x        int
	y        int
	w        int
	h        int
	minW     int
	minH     int
	size     string
	overflow string
	visible  bool
}

func defaultWidgetInstances() []defaultWidgetInstance {
	return []defaultWidgetInstance{
		{
			id:       "date-time",
			kind:     "date-time",
			title:    "Date & Time",
			x:        0,
			y:        0,
			w:        2,
			h:        1,
			minW:     1,
			minH:     1,
			size:     "wide",
			overflow: "clip",
			visible:  true,
		},
		{
			id:       "weather",
			kind:     "weather",
			title:    "Weather",
			x:        2,
			y:        0,
			w:        2,
			h:        1,
			minW:     1,
			minH:     1,
			size:     "wide",
			overflow: "clip",
			visible:  true,
		},
		{
			id:       "chat-history",
			kind:     "chat-history",
			title:    "Chat History",
			x:        0,
			y:        1,
			w:        2,
			h:        2,
			minW:     1,
			minH:     1,
			size:     "medium",
			overflow: "scroll",
			visible:  true,
		},
	}
}

const (
	envJuteHome = "JUTE_HOME"
	dbFileName  = "jute.db"
)

func ResolveDataDir(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return absoluteClean(explicit)
	}
	if home := strings.TrimSpace(os.Getenv(envJuteHome)); home != "" {
		return absoluteClean(home)
	}
	return defaultDataDir()
}

func DatabasePath(dataDir string) string {
	return filepath.Join(dataDir, dbFileName)
}

func defaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(home) == "" {
			return "", errors.New("home directory is empty")
		}
		return filepath.Join(home, "Library", "Application Support", "Jute Dash"), nil
	case "windows":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(configDir) == "" {
			return "", errors.New("user config directory is empty")
		}
		return filepath.Join(configDir, "Jute Dash"), nil
	default:
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			return absoluteClean(filepath.Join(xdg, "jute-dash"))
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(home) == "" {
			return "", errors.New("home directory is empty")
		}
		return filepath.Join(home, ".local", "share", "jute-dash"), nil
	}
}

func absoluteClean(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// ==========================================
// Fallback Memory Store (from memory_store)
// ==========================================

type MemorySettingsStore struct {
	mu        sync.RWMutex
	household HouseholdSettings
	rooms     []RoomConfig
	tiles     []TileConfig
	layout    WidgetLayout
	voice     VoiceSettings
	catalog   map[string]WidgetCatalogItem
}

func NewMemorySettingsStore() *MemorySettingsStore {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return &MemorySettingsStore{
		catalog: widgetCatalogByKind(),
		household: HouseholdSettings{
			Setup: SetupStatus{Complete: true},
		},
		layout: WidgetLayout{
			ProfileID: "default",
			Widgets:   []WidgetInstance{},
		},
		voice: VoiceSettings{
			DeviceProfileID: "default-display",
			UpdatedAt:       now,
		},
	}
}

func NewMemorySettingsStoreWithConfig(cfg Config, layout WidgetLayout) *MemorySettingsStore {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return &MemorySettingsStore{
		catalog: widgetCatalogByKind(),
		household: HouseholdSettings{
			Home:    cfg.Home,
			Display: cfg.Display,
			Weather: cfg.Weather,
			Setup: SetupStatus{
				Complete: true,
			},
		},
		layout: layout,
		voice: VoiceSettings{
			DeviceProfileID:         "default-display",
			Enabled:                 cfg.Voice.Enabled,
			Muted:                   cfg.Voice.MutedByDefault,
			WakeWordModelID:         cfg.Voice.WakeWordModelID,
			STTProviderID:           cfg.Voice.STTProviderID,
			TTSProviderID:           cfg.Voice.TTSProviderID,
			STTModelID:              cfg.Voice.STTModelID,
			TTSModelID:              cfg.Voice.TTSModelID,
			TTSVoiceID:              cfg.Voice.TTSVoiceID,
			PreferredAgentID:        cfg.Voice.PreferredAgentID,
			CloudOptIn:              cfg.Voice.CloudOptIn,
			CommandProvidersEnabled: cfg.Voice.CommandProvidersEnabled,
			SensitiveOutputPolicy:   cfg.Voice.SensitiveOutputPolicy,
			FollowupWindowSeconds:   cfg.Voice.FollowupWindowSeconds,
			MicrophoneProfile:       cfg.Voice.MicrophoneProfile,
			UpdatedAt:               now,
		},
	}
}

func (m *MemorySettingsStore) SetCatalog(items []WidgetCatalogItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cat := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		cat[item.Kind] = item
	}
	m.catalog = cat
}

func (m *MemorySettingsStore) HouseholdSettings(_ context.Context) (HouseholdSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.household, nil
}

func (m *MemorySettingsStore) SaveHouseholdSettings(
	_ context.Context,
	settings HouseholdSettings,
) (HouseholdSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.household = settings
	m.household.Setup = SetupStatus{Complete: true}
	return m.household, nil
}

func (m *MemorySettingsStore) Rooms(_ context.Context) ([]RoomConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RoomConfig(nil), m.rooms...), nil
}

func (m *MemorySettingsStore) SaveRooms(_ context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	m.rooms = normalized
	return append([]RoomConfig(nil), m.rooms...), nil
}

func (m *MemorySettingsStore) Tiles(_ context.Context) ([]TileConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]TileConfig(nil), m.tiles...), nil
}

func (m *MemorySettingsStore) SaveTiles(_ context.Context, tiles []TileConfig) ([]TileConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	m.tiles = normalized
	return append([]TileConfig(nil), m.tiles...), nil
}

func (m *MemorySettingsStore) WidgetLayout(_ context.Context, profileID string) (WidgetLayout, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.layout, nil
}

func (m *MemorySettingsStore) SaveWidgetLayout(_ context.Context, layout WidgetLayout) (WidgetLayout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := NormalizeWidgetLayout(layout, m.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}
	m.layout = normalized
	return m.layout, nil
}

func (m *MemorySettingsStore) ResetWidgetLayout(_ context.Context, profileID string) (WidgetLayout, error) {
	defaultLayout := DefaultWidgetLayout()
	if strings.TrimSpace(profileID) != "" {
		defaultLayout.ProfileID = strings.TrimSpace(profileID)
	}
	return m.SaveWidgetLayout(context.TODO(), defaultLayout)
}

func (m *MemorySettingsStore) VoiceSettings(_ context.Context, deviceProfileID string) (VoiceSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.voice, nil
}

func (m *MemorySettingsStore) SetVoiceMuted(
	_ context.Context,
	deviceProfileID string,
	muted bool,
) (VoiceSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.voice.Muted = muted
	m.voice.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.voice, nil
}

func (m *MemorySettingsStore) CancelVoice(_ context.Context, deviceProfileID string) (VoiceSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.voice.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.voice, nil
}

func (m *MemorySettingsStore) VoiceProviders(_ context.Context) ([]VoiceProviderPack, error) {
	return []VoiceProviderPack{}, nil
}

// ==========================================
// Fallback YAML Store (from yaml_store)
// ==========================================

type YAMLSettingsStore struct {
	mu         sync.RWMutex
	configPath string
	catalog    map[string]WidgetCatalogItem
}

func NewYAMLSettingsStore(configPath string) *YAMLSettingsStore {
	return &YAMLSettingsStore{
		configPath: configPath,
		catalog:    widgetCatalogByKind(),
	}
}

func (y *YAMLSettingsStore) SetCatalog(items []WidgetCatalogItem) {
	y.mu.Lock()
	defer y.mu.Unlock()
	m := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		m[item.Kind] = item
	}
	y.catalog = m
}

func (y *YAMLSettingsStore) load() (Config, error) {
	if y.configPath == "" {
		return DefaultConfig(), nil
	}
	return LoadConfig(y.configPath)
}

func (y *YAMLSettingsStore) save(cfg Config) error {
	if y.configPath == "" {
		return errors.New("cannot save: config path is empty")
	}
	return SaveYAML(y.configPath, cfg)
}

func (y *YAMLSettingsStore) HouseholdSettings(_ context.Context) (HouseholdSettings, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.load()
	if err != nil {
		return HouseholdSettings{}, err
	}
	missing := missingSetupFields(cfg, true)
	return HouseholdSettings{
		Home:    cfg.Home,
		Display: cfg.Display,
		Weather: cfg.Weather,
		Setup: SetupStatus{
			Complete: len(missing) == 0,
			Missing:  missing,
		},
	}, nil
}

func (y *YAMLSettingsStore) SaveHouseholdSettings(
	_ context.Context,
	settings HouseholdSettings,
) (HouseholdSettings, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	cfg, err := y.load()
	if err != nil {
		return HouseholdSettings{}, err
	}
	cfg.Home = settings.Home
	cfg.Display = settings.Display
	cfg.Weather = settings.Weather
	if err := y.save(cfg); err != nil {
		return HouseholdSettings{}, err
	}
	missing := missingSetupFields(cfg, true)
	return HouseholdSettings{
		Home:    cfg.Home,
		Display: cfg.Display,
		Weather: cfg.Weather,
		Setup: SetupStatus{
			Complete: len(missing) == 0,
			Missing:  missing,
		},
	}, nil
}

func (y *YAMLSettingsStore) Rooms(_ context.Context) ([]RoomConfig, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.load()
	if err != nil {
		return nil, err
	}
	return cfg.Rooms, nil
}

func (y *YAMLSettingsStore) SaveRooms(_ context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	cfg, err := y.load()
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	cfg.Rooms = normalized
	if err := y.save(cfg); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (y *YAMLSettingsStore) Tiles(_ context.Context) ([]TileConfig, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.load()
	if err != nil {
		return nil, err
	}
	return cfg.Tiles, nil
}

func (y *YAMLSettingsStore) SaveTiles(_ context.Context, tiles []TileConfig) ([]TileConfig, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	cfg, err := y.load()
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	cfg.Tiles = normalized
	if err := y.save(cfg); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (y *YAMLSettingsStore) WidgetLayout(_ context.Context, profileID string) (WidgetLayout, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.load()
	if err != nil {
		return WidgetLayout{}, err
	}
	widgetInstances := make([]WidgetInstance, 0, len(cfg.Dashboard.Widgets))
	for _, w := range cfg.Dashboard.Widgets {
		instance := WidgetInstance{
			ID:       w.ID,
			Kind:     w.Type,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			MinW:     1,
			MinH:     1,
			Size:     "medium",
			Visible:  w.Visible,
			Settings: w.Settings,
		}
		if provider, ok := y.catalog[w.Type]; ok {
			instance.MinW = provider.MinW
			instance.MinH = provider.MinH
			instance.Size = provider.DefaultSize
			instance.Overflow = provider.Overflow
		}
		if instance.Settings == nil {
			instance.Settings = map[string]any{}
		}
		widgetInstances = append(widgetInstances, instance)
	}
	layout := WidgetLayout{
		ProfileID: "default",
		Widgets:   widgetInstances,
	}
	normalized, err := NormalizeWidgetLayout(layout, y.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}
	return normalized, nil
}

func (y *YAMLSettingsStore) SaveWidgetLayout(_ context.Context, layout WidgetLayout) (WidgetLayout, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	cfg, err := y.load()
	if err != nil {
		return WidgetLayout{}, err
	}
	normalized, err := NormalizeWidgetLayout(layout, y.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}
	newWidgets := make([]DashboardWidgetConfig, 0, len(normalized.Widgets))
	for _, w := range normalized.Widgets {
		newWidgets = append(newWidgets, DashboardWidgetConfig{
			ID:       w.ID,
			Type:     w.Kind,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			Visible:  w.Visible,
			Settings: w.Settings,
		})
	}
	cfg.Dashboard.Widgets = newWidgets
	if err := y.save(cfg); err != nil {
		return WidgetLayout{}, err
	}
	return normalized, nil
}

func (y *YAMLSettingsStore) ResetWidgetLayout(_ context.Context, profileID string) (WidgetLayout, error) {
	defaultLayout := DefaultWidgetLayout()
	if strings.TrimSpace(profileID) != "" {
		defaultLayout.ProfileID = strings.TrimSpace(profileID)
	}
	return y.SaveWidgetLayout(context.TODO(), defaultLayout)
}

func (y *YAMLSettingsStore) VoiceSettings(_ context.Context, deviceProfileID string) (VoiceSettings, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.load()
	if err != nil {
		return VoiceSettings{}, err
	}
	return VoiceSettings{
		DeviceProfileID:         "default-display",
		Enabled:                 cfg.Voice.Enabled,
		Muted:                   cfg.Voice.MutedByDefault,
		WakeWordModelID:         cfg.Voice.WakeWordModelID,
		STTProviderID:           cfg.Voice.STTProviderID,
		TTSProviderID:           cfg.Voice.TTSProviderID,
		STTModelID:              cfg.Voice.STTModelID,
		TTSModelID:              cfg.Voice.TTSModelID,
		TTSVoiceID:              cfg.Voice.TTSVoiceID,
		PreferredAgentID:        cfg.Voice.PreferredAgentID,
		CloudOptIn:              cfg.Voice.CloudOptIn,
		CommandProvidersEnabled: cfg.Voice.CommandProvidersEnabled,
		SensitiveOutputPolicy:   cfg.Voice.SensitiveOutputPolicy,
		FollowupWindowSeconds:   cfg.Voice.FollowupWindowSeconds,
		MicrophoneProfile:       cfg.Voice.MicrophoneProfile,
		UpdatedAt:               time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (y *YAMLSettingsStore) SetVoiceMuted(
	_ context.Context,
	deviceProfileID string,
	muted bool,
) (VoiceSettings, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	cfg, err := y.load()
	if err != nil {
		return VoiceSettings{}, err
	}
	cfg.Voice.MutedByDefault = muted
	if err := y.save(cfg); err != nil {
		return VoiceSettings{}, err
	}
	return VoiceSettings{
		DeviceProfileID:         "default-display",
		Enabled:                 cfg.Voice.Enabled,
		Muted:                   cfg.Voice.MutedByDefault,
		WakeWordModelID:         cfg.Voice.WakeWordModelID,
		STTProviderID:           cfg.Voice.STTProviderID,
		TTSProviderID:           cfg.Voice.TTSProviderID,
		STTModelID:              cfg.Voice.STTModelID,
		TTSModelID:              cfg.Voice.TTSModelID,
		TTSVoiceID:              cfg.Voice.TTSVoiceID,
		PreferredAgentID:        cfg.Voice.PreferredAgentID,
		CloudOptIn:              cfg.Voice.CloudOptIn,
		CommandProvidersEnabled: cfg.Voice.CommandProvidersEnabled,
		SensitiveOutputPolicy:   cfg.Voice.SensitiveOutputPolicy,
		FollowupWindowSeconds:   cfg.Voice.FollowupWindowSeconds,
		MicrophoneProfile:       cfg.Voice.MicrophoneProfile,
		UpdatedAt:               time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (y *YAMLSettingsStore) CancelVoice(_ context.Context, deviceProfileID string) (VoiceSettings, error) {
	return y.VoiceSettings(context.TODO(), deviceProfileID)
}

func (y *YAMLSettingsStore) VoiceProviders(_ context.Context) ([]VoiceProviderPack, error) {
	return []VoiceProviderPack{}, nil
}

