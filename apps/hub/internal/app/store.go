package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/filesync"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/voice"
	"jute-dash/apps/hub/internal/pkg/database"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	db            *database.Database
	riverClient   *river.Client[*sql.Tx]
	logger        *slog.Logger
	syncer        filesync.Syncer
	DashboardRepo *dashboard.Repository
	HomestateRepo *homestate.Repository
	VoiceRepo     *voice.Repository
}

func Open(dbPath string, log *slog.Logger) (*Store, error) {
	db, err := database.Open(dbPath, log)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, logger: log}
	s.DashboardRepo = dashboard.NewRepository(s.DB())
	s.DashboardRepo.SetCatalog(dashboard.RegisteredCatalog())
	s.DashboardRepo.SetOnSave(s.triggerSync)
	s.DashboardRepo.SetConfigStore(s)
	s.HomestateRepo = homestate.NewRepository(s.DB())
	s.HomestateRepo.SetOnSave(s.triggerSync)
	s.VoiceRepo = voice.NewRepository(s.DB())
	return s, nil
}

func (s *Store) SetCatalog(items []dashboard.WidgetCatalogItem) {
	if s == nil || s.DashboardRepo == nil {
		return
	}
	s.DashboardRepo.SetCatalog(items)
}

func (s *Store) triggerSync(ctx context.Context) {
	if s == nil {
		return
	}
	if s.riverClient != nil {
		_ = s.EnqueueSync(ctx)
	} else if s.syncer != nil {
		_ = s.syncer.Sync(ctx)
	}
}

func (s *Store) SetLogger(log *slog.Logger) {
	if s == nil {
		return
	}
	s.logger = log
	if s.db != nil && s.db.DB() != nil {
		gormLevel := logger.Warn
		if log.Enabled(context.Background(), slog.LevelDebug) {
			gormLevel = logger.Info
		}
		gormLogger := database.NewSlogLogger(log)
		gormLogger.LogLevel = gormLevel
		s.db.DB().Config.Logger = gormLogger
	}
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	if s.riverClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.riverClient.Stop(ctx)
	}
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Path() string {
	if s == nil || s.db == nil {
		return ""
	}
	return s.db.Path()
}

func (s *Store) DB() *gorm.DB {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.DB()
}

func (s *Store) Initialize(
	ctx context.Context,
	bootstrap config.Config,
	bootstrapProvided bool,
) (homestate.InitResult, error) {
	if err := s.Migrate(ctx); err != nil {
		return homestate.InitResult{}, err
	}

	seeded, err := s.IsSeeded(ctx)
	if err != nil {
		return homestate.InitResult{}, err
	}

	result := homestate.InitResult{}
	if !seeded {
		if err := s.seed(ctx, bootstrap, bootstrapProvided); err != nil {
			return homestate.InitResult{}, err
		}
		result.Seeded = true
	}
	if err := s.ensureDefaultVoiceSettings(ctx, config.DefaultConfig().Voice); err != nil {
		return homestate.InitResult{}, err
	}

	cfg, err := s.Config(ctx)
	if err != nil {
		return homestate.InitResult{}, err
	}
	status, err := s.SetupStatus(ctx)
	if err != nil {
		return homestate.InitResult{}, err
	}
	result.Config = cfg
	result.Setup = status
	return result, nil
}

// appMetaDB is a tiny key/value table for schema/data migration markers.
type appMetaDB struct {
	Key   string `gorm:"primaryKey;column:key"`
	Value string `gorm:"column:value"`
}

func (appMetaDB) TableName() string { return "app_meta" }

const gridBaseColumnsMetaKey = "grid_base_columns"

func (s *Store) Migrate(ctx context.Context) error {
	if err := s.db.Migrate(
		&homestate.HouseholdSettingsDB{},
		&homestate.DeviceProfileDB{},
		&homestate.LayoutProfileDB{},
		&homestate.RoomDB{},
		&homestate.TileDB{},
		&dashboard.WidgetPackDB{},
		&dashboard.WidgetInstanceDB{},
		&dashboard.WidgetPermissionDB{},
		&voice.ProviderPackDB{},
		&voice.SettingsDB{},
		&homestate.AdapterConnectionDB{},
		&homestate.SettingAuditLogDB{},
		&appMetaDB{},
	); err != nil {
		return err
	}
	if err := s.migrateRiver(ctx); err != nil {
		return err
	}
	return s.migrateGridToBaseColumns()
}

func (s *Store) migrateRiver(ctx context.Context) error {
	sqlDB, err := s.DB().DB()
	if err != nil {
		return fmt.Errorf("get sql.DB for river migrate: %w", err)
	}
	driver := riversqlite.New(sqlDB)
	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("run river migrations: %w", err)
	}
	return nil
}

// migrateGridToBaseColumns scales legacy 4-column widget coordinates onto the
// 12-column base grid. It is idempotent: a marker row records that the grid is
// at the base resolution. It runs before seeding, so fresh installs (which seed
// 12-column defaults) scale zero rows and simply record the marker.
func (s *Store) migrateGridToBaseColumns() error {
	db := s.db.DB()
	var meta appMetaDB
	err := db.Where("key = ?", gridBaseColumnsMetaKey).First(&meta).Error
	if err == nil && meta.Value == strconv.Itoa(dashboard.BaseColumns) {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check grid migration marker: %w", err)
	}

	scale := dashboard.LegacyColumnScale
	if updErr := db.Model(&dashboard.WidgetInstanceDB{}).
		Where("1 = 1").
		Updates(map[string]any{
			"x":     gorm.Expr("x * ?", scale),
			"w":     gorm.Expr("w * ?", scale),
			"min_w": gorm.Expr("min_w * ?", scale),
		}).Error; updErr != nil {
		return fmt.Errorf("scale legacy widget columns: %w", updErr)
	}

	marker := appMetaDB{
		Key:   gridBaseColumnsMetaKey,
		Value: strconv.Itoa(dashboard.BaseColumns),
	}
	if saveErr := db.Save(&marker).Error; saveErr != nil {
		return fmt.Errorf("save grid migration marker: %w", saveErr)
	}
	return nil
}

func (s *Store) IsSeeded(ctx context.Context) (bool, error) {
	if !s.db.DB().Migrator().HasTable(&homestate.HouseholdSettingsDB{}) {
		return false, nil
	}
	var count int64
	if err := s.db.DB().WithContext(ctx).Model(&homestate.HouseholdSettingsDB{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check seeded store: %w", err)
	}
	return count > 0, nil
}

func (s *Store) seed(ctx context.Context, cfg config.Config, bootstrapProvided bool) error {
	if err := config.EnsureValidConfig(&cfg); err != nil {
		return fmt.Errorf("validate seed config: %w", err)
	}

	setup := setupStatusForSeed(cfg, bootstrapProvided)
	now := nowUTC()

	return s.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hs := homestate.HouseholdSettingsDB{
			ID:                      homestate.DefaultHouseholdID,
			Name:                    cfg.Home.Name,
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

		dp := homestate.DeviceProfileDB{
			ID:              homestate.DefaultDeviceProfileID,
			Name:            "Default Display",
			InteractionMode: "touch",
			LayoutProfileID: homestate.DefaultLayoutProfileID,
			SettingsJSON:    "{}",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&dp).Error; err != nil {
			return fmt.Errorf("seed device profile: %w", err)
		}

		lp := homestate.LayoutProfileDB{
			ID:              homestate.DefaultLayoutProfileID,
			DeviceProfileID: homestate.DefaultDeviceProfileID,
			Name:            "Default Dashboard",
			SettingsJSON:    "{}",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&lp).Error; err != nil {
			return fmt.Errorf("seed layout profile: %w", err)
		}

		for i, room := range cfg.Rooms {
			rDB := homestate.RoomDB{
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
			tDB := homestate.TileDB{
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

		layout, err := dashboard.WidgetLayoutFromDashboardConfig(
			cfg.Dashboard,
			widgetCatalogForSeed(),
		)
		if err != nil {
			return fmt.Errorf("seed dashboard widgets: %w", err)
		}

		for i, widget := range layout.Widgets {
			wDB := dashboard.WidgetInstanceDB{
				ID:              widget.ID,
				Kind:            widget.Kind,
				Title:           widget.Title,
				LayoutProfileID: homestate.DefaultLayoutProfileID,
				X:               widget.X,
				Y:               widget.Y,
				W:               widget.W,
				H:               widget.H,
				MinW:            widget.MinW,
				MinH:            widget.MinH,
				Size:            widget.Size,
				Mode:            widget.Mode,
				SettingsJSON:    mustJSONString(widget.Settings),
				Visible:         boolToInt(widget.Visible),
				SortOrder:       i,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := tx.Create(&wDB).Error; err != nil {
				return fmt.Errorf("seed widget %s: %w", widget.ID, err)
			}
		}

		vDB := voice.SettingsDB{
			DeviceProfileID:         homestate.DefaultDeviceProfileID,
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

func (s *Store) ensureDefaultVoiceSettings(ctx context.Context, voiceCfg voice.Config) error {
	now := nowUTC()
	var count int64
	if err := s.db.DB().
		WithContext(ctx).
		Model(&voice.SettingsDB{}).
		Where("device_profile_id = ?", homestate.DefaultDeviceProfileID).
		Count(&count).
		Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	vDB := voice.SettingsDB{
		DeviceProfileID:         homestate.DefaultDeviceProfileID,
		Enabled:                 boolToInt(voiceCfg.Enabled),
		Muted:                   boolToInt(voiceCfg.MutedByDefault),
		WakeWordModelID:         voiceCfg.WakeWordModelID,
		STTProviderID:           voiceCfg.STTProviderID,
		TTSProviderID:           voiceCfg.TTSProviderID,
		STTModelID:              voiceCfg.STTModelID,
		TTSModelID:              voiceCfg.TTSModelID,
		TTSVoiceID:              voiceCfg.TTSVoiceID,
		PreferredAgentID:        voiceCfg.PreferredAgentID,
		CloudOptIn:              boolToInt(voiceCfg.CloudOptIn),
		CommandProvidersEnabled: boolToInt(voiceCfg.CommandProvidersEnabled),
		SensitiveOutputPolicy:   voiceCfg.SensitiveOutputPolicy,
		FollowupWindowSeconds:   voiceCfg.FollowupWindowSeconds,
		MicrophoneProfile:       voiceCfg.MicrophoneProfile,
		UpdatedAt:               now,
	}
	return s.db.DB().WithContext(ctx).Create(&vDB).Error
}

func (s *Store) Config(ctx context.Context) (config.Config, error) {
	cfg := config.DefaultConfig()

	var hs homestate.HouseholdSettingsDB
	if err := s.db.DB().WithContext(ctx).First(&hs, "id = ?", homestate.DefaultHouseholdID).Error; err != nil {
		return config.Config{}, fmt.Errorf("load household settings: %w", err)
	}
	cfg.Home.Name = hs.Name
	cfg.Display.Theme = hs.DisplayTheme
	cfg.Display.ColorMode = hs.DisplayColorMode
	cfg.Display.ThemeID = hs.DisplayThemeID
	cfg.Display.Density = hs.DisplayDensity
	cfg.Display.Motion = hs.DisplayMotion
	cfg.Display.AccentColor = hs.DisplayAccentColor
	cfg.Display.IdleMode = hs.DisplayIdleMode

	if err := decodeJSONSetting(hs.DisplayBackgroundJSON, &cfg.Display.Background); err != nil {
		return config.Config{}, fmt.Errorf("decode display background: %w", err)
	}
	if err := decodeJSONSetting(hs.DisplayWidgetChromeJSON, &cfg.Display.WidgetChrome); err != nil {
		return config.Config{}, fmt.Errorf("decode display widget chrome: %w", err)
	}

	var vs voice.SettingsDB
	if err := s.db.DB().
		WithContext(ctx).
		First(&vs, "device_profile_id = ?", homestate.DefaultDeviceProfileID).
		Error; err != nil {
		return config.Config{}, fmt.Errorf("load voice settings: %w", err)
	}
	cfg.Voice = voice.Config{
		Enabled:                 vs.Enabled == 1,
		MutedByDefault:          vs.Muted == 1,
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
	}

	var dbRooms []homestate.RoomDB
	if err := s.db.DB().WithContext(ctx).Order("sort_order").Find(&dbRooms).Error; err != nil {
		return config.Config{}, fmt.Errorf("load rooms: %w", err)
	}
	rooms := make([]homestate.RoomConfig, len(dbRooms))
	for i, r := range dbRooms {
		rooms[i] = homestate.RoomConfig{
			ID:      r.ID,
			Name:    r.Name,
			Summary: r.Summary,
			Status:  r.Status,
		}
	}

	var dbTiles []homestate.TileDB
	if err := s.db.DB().WithContext(ctx).Order("sort_order").Find(&dbTiles).Error; err != nil {
		return config.Config{}, fmt.Errorf("load tiles: %w", err)
	}
	tiles := make([]homestate.TileConfig, len(dbTiles))
	for i, t := range dbTiles {
		tiles[i] = homestate.TileConfig{
			ID:     t.ID,
			Kind:   t.Kind,
			Label:  t.Label,
			Value:  t.Value,
			Detail: t.Detail,
		}
	}

	layout, err := s.DashboardRepo.WidgetLayout(ctx, "")
	if err == nil {
		widgets := make([]dashboard.DashboardWidgetConfig, 0, len(layout.Widgets))
		for _, w := range layout.Widgets {
			widgets = append(widgets, dashboard.DashboardWidgetConfig{
				ID:             w.ID,
				Type:           w.Kind,
				Title:          w.Title,
				X:              w.X,
				Y:              w.Y,
				W:              w.W,
				H:              w.H,
				MinW:           w.MinW,
				MinH:           w.MinH,
				Size:           w.Size,
				Visible:        w.Visible,
				Mode:           w.Mode,
				Settings:       w.Settings,
				ConnectionRefs: w.ConnectionRefs,
			})
		}
		cfg.Dashboard.Widgets = widgets
	}

	cfg.Agents = nil
	cfg.Rooms = rooms
	cfg.Tiles = tiles

	if err := config.EnsureValidConfig(&cfg); err != nil {
		return config.Config{}, fmt.Errorf("validate store config: %w", err)
	}
	return cfg, nil
}

func (s *Store) SetupStatus(ctx context.Context) (homestate.SetupStatus, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return homestate.SetupStatus{}, err
	}
	var hs homestate.HouseholdSettingsDB
	if err := s.db.DB().WithContext(ctx).First(&hs, "id = ?", homestate.DefaultHouseholdID).Error; err != nil {
		return homestate.SetupStatus{}, fmt.Errorf("load setup status: %w", err)
	}

	missing := missingSetupFields(cfg, hs.SetupCompleted == 1)
	if missing == nil {
		missing = []string{}
	}
	return homestate.SetupStatus{
		Complete: hs.SetupCompleted == 1 && len(missing) == 0,
		Missing:  missing,
	}, nil
}

// Helpers

func widgetCatalogForSeed() map[string]dashboard.WidgetCatalogItem {
	items := dashboard.RegisteredCatalog()
	if len(items) == 0 {
		items = dashboard.WidgetCatalog()
	}
	catalog := make(map[string]dashboard.WidgetCatalogItem, len(items))
	for _, item := range items {
		catalog[item.Kind] = item
	}
	return catalog
}

func setupStatusForSeed(cfg config.Config, bootstrapProvided bool) homestate.SetupStatus {
	missing := missingSetupFields(cfg, bootstrapProvided)
	if missing == nil {
		missing = []string{}
	}
	return homestate.SetupStatus{
		Complete: bootstrapProvided && len(missing) == 0,
		Missing:  missing,
	}
}

func missingSetupFields(cfg config.Config, confirmed bool) []string {
	if !confirmed {
		return []string{"home.name"}
	}

	var missing []string
	if strings.TrimSpace(cfg.Home.Name) == "" {
		missing = append(missing, "home.name")
	}
	return missing
}

func decodeJSONSetting(raw string, target any) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}

func mustJSONString(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(bytes)
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

func (s *Store) StartQueue(syncer filesync.Syncer) error {
	s.syncer = syncer
	sqlDB, err := s.DB().DB()
	if err != nil {
		return err
	}

	var riverLogger *slog.Logger
	if s.logger != nil {
		riverLogger = slog.New(&filesync.RiverLevelFilterHandler{Handler: s.logger.Handler()})
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, filesync.NewConfigSyncWorker(syncer))

	client, err := river.NewClient(riversqlite.New(sqlDB), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 1},
		},
		Workers: workers,
		Logger:  riverLogger,
	})
	if err != nil {
		return err
	}

	s.riverClient = client
	return s.riverClient.Start(context.Background())
}

func (s *Store) EnqueueSync(ctx context.Context) error {
	if s == nil || s.riverClient == nil {
		return nil
	}
	_, err := s.riverClient.Insert(ctx, filesync.ConfigSyncArgs{}, nil)
	return err
}
