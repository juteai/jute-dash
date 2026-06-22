package repository

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
	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/apps/hub/internal/pkg/database"
	"jute-dash/apps/hub/internal/pkg/filesync"

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
	DashboardRepo *DashboardRepository
	HomestateRepo *HomeRepository
	SecretVault   *Vault
	VoiceRepo     *VoiceRepository
}

func Open(dbPath string, log *slog.Logger) (*Store, error) {
	db, err := database.Open(dbPath, log)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, logger: log}
	s.DashboardRepo = NewDashboardRepository(s.DB())
	s.DashboardRepo.SetCatalog(WidgetCatalog())
	s.DashboardRepo.SetOnSave(s.triggerSync)
	s.DashboardRepo.SetConfigStore(s)
	s.HomestateRepo = NewHomeRepository(s.DB())
	s.HomestateRepo.SetOnSave(s.triggerSync)
	s.SecretVault = NewVault(s.DB(), NewKeyringMasterKeyProvider())
	s.VoiceRepo = NewVoiceRepository(s.DB())
	return s, nil
}

func (s *Store) SetCatalog(items []WidgetCatalogItem) {
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
) (InitResult, error) {
	if err := s.Migrate(ctx); err != nil {
		return InitResult{}, err
	}

	seeded, err := s.IsSeeded(ctx)
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
	if bootstrapProvided {
		if err := s.reconcileVoiceBootstrap(ctx, bootstrap.Voice, bootstrap.ProviderPacks); err != nil {
			return InitResult{}, err
		}
	} else if err := s.ensureDefaultVoiceSettings(ctx, config.DefaultConfig().Voice); err != nil {
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

// appMetaDB is a tiny key/value table for schema/data migration markers.
type appMetaDB struct {
	Key   string `gorm:"primaryKey;column:key"`
	Value string `gorm:"column:value"`
}

func (appMetaDB) TableName() string { return "app_meta" }

const gridBaseColumnsMetaKey = "grid_base_columns"

func (s *Store) Migrate(ctx context.Context) error {
	if err := s.db.Migrate(
		&HouseholdSettingsDB{},
		&DeviceProfileDB{},
		&LayoutProfileDB{},
		&RoomDB{},
		&TileDB{},
		&WidgetPackDB{},
		&WidgetInstanceDB{},
		&WidgetPermissionDB{},
		&ProviderPackDB{},
		&SettingsDB{},
		&AdapterConnectionDB{},
		&SecretDB{},
		&SettingAuditLogDB{},
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
	if err == nil && meta.Value == strconv.Itoa(BaseColumns) {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check grid migration marker: %w", err)
	}

	scale := LegacyColumnScale
	if updErr := db.Model(&WidgetInstanceDB{}).
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
		Value: strconv.Itoa(BaseColumns),
	}
	if saveErr := db.Save(&marker).Error; saveErr != nil {
		return fmt.Errorf("save grid migration marker: %w", saveErr)
	}
	return nil
}

func (s *Store) IsSeeded(ctx context.Context) (bool, error) {
	if !s.db.DB().Migrator().HasTable(&HouseholdSettingsDB{}) {
		return false, nil
	}
	var count int64
	if err := s.db.DB().WithContext(ctx).Model(&HouseholdSettingsDB{}).Count(&count).Error; err != nil {
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
		hs := HouseholdSettingsDB{
			ID:                      DefaultHouseholdID,
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

		dp := DeviceProfileDB{
			ID:              DefaultDeviceProfileID,
			Name:            "Default Display",
			InteractionMode: "touch",
			LayoutProfileID: DefaultLayoutProfileID,
			SettingsJSON:    "{}",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&dp).Error; err != nil {
			return fmt.Errorf("seed device profile: %w", err)
		}

		lp := LayoutProfileDB{
			ID:              DefaultLayoutProfileID,
			DeviceProfileID: DefaultDeviceProfileID,
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

		layout, err := WidgetLayoutFromDashboardConfig(
			cfg.Dashboard,
			WidgetCatalogForSeed(),
		)
		if err != nil {
			return fmt.Errorf("seed dashboard widgets: %w", err)
		}

		for i, widget := range layout.Widgets {
			connectionRefsJSON := mustJSONString(widget.ConnectionRefs)
			wDB := WidgetInstanceDB{
				ID:                 widget.ID,
				ScreenID:           widget.ScreenID,
				Kind:               widget.Kind,
				Title:              widget.Title,
				LayoutProfileID:    DefaultLayoutProfileID,
				X:                  widget.X,
				Y:                  widget.Y,
				W:                  widget.W,
				H:                  widget.H,
				MinW:               widget.MinW,
				MinH:               widget.MinH,
				Size:               widget.Size,
				Mode:               widget.Mode,
				SettingsJSON:       mustJSONString(widget.Settings),
				ConnectionRefsJSON: connectionRefsJSON,
				Visible:            boolToInt(widget.Visible),
				SortOrder:          i,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			if err := tx.Create(&wDB).Error; err != nil {
				return fmt.Errorf("seed widget %s: %w", widget.ID, err)
			}
		}

		vDB := SettingsDB{
			DeviceProfileID:         DefaultDeviceProfileID,
			Enabled:                 boolToInt(cfg.Voice.Enabled),
			Muted:                   boolToInt(cfg.Voice.MutedByDefault),
			WakeWordModelID:         cfg.Voice.WakeWordModelID,
			WakeWordPhrase:          cfg.Voice.WakeWordPhrase,
			WakeSensitivity:         cfg.Voice.WakeSensitivity,
			STTProviderID:           cfg.Voice.STTProviderID,
			TTSProviderID:           cfg.Voice.TTSProviderID,
			STTModelID:              cfg.Voice.STTModelID,
			TTSModelID:              cfg.Voice.TTSModelID,
			TTSVoiceID:              cfg.Voice.TTSVoiceID,
			TTSEnabled:              boolToInt(cfg.Voice.TTSEnabled),
			TTSLocale:               cfg.Voice.TTSLocale,
			TTSSpeed:                cfg.Voice.TTSSpeed,
			TTSVolume:               cfg.Voice.TTSVolume,
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
		if err := seedProviderPacks(ctx, tx, cfg.ProviderPacks, now); err != nil {
			return err
		}

		return nil
	})
}

func seedProviderPacks(
	ctx context.Context,
	tx *gorm.DB,
	packs []model.ProviderPackConfig,
	now string,
) error {
	for _, pack := range packs {
		provider, err := providerPackDBFromConfig(pack, now)
		if err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Create(&provider).Error; err != nil {
			return fmt.Errorf("seed voice provider %s: %w", pack.ID, err)
		}
	}
	return nil
}

func (s *Store) reconcileVoiceBootstrap(
	ctx context.Context,
	voiceCfg Config,
	packs []model.ProviderPackConfig,
) error {
	now := nowUTC()
	return s.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		settings := SettingsDB{
			DeviceProfileID:         DefaultDeviceProfileID,
			Enabled:                 boolToInt(voiceCfg.Enabled),
			Muted:                   boolToInt(voiceCfg.MutedByDefault),
			WakeWordModelID:         voiceCfg.WakeWordModelID,
			WakeWordPhrase:          voiceCfg.WakeWordPhrase,
			WakeSensitivity:         voiceCfg.WakeSensitivity,
			STTProviderID:           voiceCfg.STTProviderID,
			TTSProviderID:           voiceCfg.TTSProviderID,
			STTModelID:              voiceCfg.STTModelID,
			TTSModelID:              voiceCfg.TTSModelID,
			TTSVoiceID:              voiceCfg.TTSVoiceID,
			TTSEnabled:              boolToInt(voiceCfg.TTSEnabled),
			TTSLocale:               voiceCfg.TTSLocale,
			TTSSpeed:                voiceCfg.TTSSpeed,
			TTSVolume:               voiceCfg.TTSVolume,
			PreferredAgentID:        voiceCfg.PreferredAgentID,
			CloudOptIn:              boolToInt(voiceCfg.CloudOptIn),
			CommandProvidersEnabled: boolToInt(voiceCfg.CommandProvidersEnabled),
			SensitiveOutputPolicy:   voiceCfg.SensitiveOutputPolicy,
			FollowupWindowSeconds:   voiceCfg.FollowupWindowSeconds,
			MicrophoneProfile:       voiceCfg.MicrophoneProfile,
			UpdatedAt:               now,
		}
		if err := tx.Save(&settings).Error; err != nil {
			return fmt.Errorf("reconcile voice settings: %w", err)
		}
		for _, pack := range packs {
			provider, err := providerPackDBFromConfig(pack, now)
			if err != nil {
				return err
			}
			var existing ProviderPackDB
			err = tx.First(&existing, "id = ?", provider.ID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load voice provider %s: %w", provider.ID, err)
			}
			if err == nil {
				provider.InstalledAt = existing.InstalledAt
				provider.LastActivationAt = existing.LastActivationAt
			}
			if err := tx.Save(&provider).Error; err != nil {
				return fmt.Errorf("reconcile voice provider %s: %w", provider.ID, err)
			}
		}
		return nil
	})
}

func providerPackDBFromConfig(pack model.ProviderPackConfig, now string) (ProviderPackDB, error) {
	manifest := providerManifestFromConfig(pack)
	if problems := ValidateProviderManifest(manifest); len(problems) > 0 {
		return ProviderPackDB{}, fmt.Errorf(
			"invalid voice provider pack %s: %s",
			pack.ID,
			strings.Join(problems, "; "),
		)
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return ProviderPackDB{}, fmt.Errorf("marshal voice provider %s: %w", pack.ID, err)
	}
	health := strings.TrimSpace(pack.Health)
	if health == "" {
		health = "available"
	}
	transportType := strings.TrimSpace(pack.TransportKind)
	if transportType == "" {
		transportType = manifest.Transport.Type
	}
	installedAt := strings.TrimSpace(pack.InstalledAt)
	if installedAt == "" {
		installedAt = now
	}
	updatedAt := strings.TrimSpace(pack.UpdatedAt)
	if updatedAt == "" {
		updatedAt = now
	}
	return ProviderPackDB{
		ID:            manifest.ID,
		Name:          manifest.Name,
		Version:       manifest.Version,
		Kind:          manifest.Kind,
		TransportType: transportType,
		ManifestJSON:  string(manifestBytes),
		HealthStatus:  health,
		LastError:     pack.Error,
		InstalledAt:   installedAt,
		UpdatedAt:     updatedAt,
	}, nil
}

func providerManifestFromConfig(pack model.ProviderPackConfig) ProviderManifest {
	credentials := make([]CredentialManifest, 0, len(pack.Credentials))
	for _, credential := range pack.Credentials {
		credentials = append(credentials, CredentialManifest{
			ID:       credential.ID,
			Label:    credential.Label,
			Source:   credential.Source,
			Env:      credential.Env,
			Required: credential.Required,
		})
	}
	var wakeWord WakeWordManifest
	if pack.Wake != nil {
		wakeModels := make([]WakeWordModelManifest, 0, len(pack.Wake.Models))
		for _, wakeModel := range pack.Wake.Models {
			wakeModels = append(wakeModels, WakeWordModelManifest{
				ID:          wakeModel.ID,
				Path:        wakeModel.Path,
				Phrase:      wakeModel.Phrase,
				Languages:   append([]string(nil), wakeModel.Languages...),
				Sensitivity: wakeModel.Sensitivity,
			})
		}
		wakeWord = WakeWordManifest{
			DefaultModelID: pack.Wake.DefaultModelID,
			Phrase:         pack.Wake.Phrase,
			Languages:      append([]string(nil), pack.Wake.Languages...),
			Sensitivity:    pack.Wake.Sensitivity,
			Models:         wakeModels,
		}
	}
	var tts TTSManifest
	if pack.TTS != nil {
		ttsVoices := make([]TTSVoiceManifest, 0, len(pack.TTS.Voices))
		for _, voice := range pack.TTS.Voices {
			ttsVoices = append(ttsVoices, TTSVoiceManifest{
				ID:      voice.ID,
				Label:   voice.Label,
				Locale:  voice.Locale,
				ModelID: voice.ModelID,
			})
		}
		tts = TTSManifest{
			DefaultVoiceID: pack.TTS.DefaultVoiceID,
			DefaultModelID: pack.TTS.DefaultModelID,
			Voices:         ttsVoices,
		}
	}
	return ProviderManifest{
		ID:      pack.ID,
		Name:    pack.Name,
		Version: pack.Version,
		Kind:    pack.Kind,
		Transport: TransportManifest{
			Type:    pack.Transport.Type,
			Command: pack.Transport.Command,
			Args:    append([]string(nil), pack.Transport.Args...),
		},
		Capabilities: pack.Caps,
		Credentials:  credentials,
		WakeWord:     wakeWord,
		TTS:          tts,
	}
}

func (s *Store) ensureDefaultVoiceSettings(ctx context.Context, voiceCfg Config) error {
	now := nowUTC()
	var count int64
	if err := s.db.DB().
		WithContext(ctx).
		Model(&SettingsDB{}).
		Where("device_profile_id = ?", DefaultDeviceProfileID).
		Count(&count).
		Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	vDB := SettingsDB{
		DeviceProfileID:         DefaultDeviceProfileID,
		Enabled:                 boolToInt(voiceCfg.Enabled),
		Muted:                   boolToInt(voiceCfg.MutedByDefault),
		WakeWordModelID:         voiceCfg.WakeWordModelID,
		WakeWordPhrase:          voiceCfg.WakeWordPhrase,
		WakeSensitivity:         voiceCfg.WakeSensitivity,
		STTProviderID:           voiceCfg.STTProviderID,
		TTSProviderID:           voiceCfg.TTSProviderID,
		STTModelID:              voiceCfg.STTModelID,
		TTSModelID:              voiceCfg.TTSModelID,
		TTSVoiceID:              voiceCfg.TTSVoiceID,
		TTSEnabled:              boolToInt(voiceCfg.TTSEnabled),
		TTSLocale:               voiceCfg.TTSLocale,
		TTSSpeed:                voiceCfg.TTSSpeed,
		TTSVolume:               voiceCfg.TTSVolume,
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

	var hs HouseholdSettingsDB
	if err := s.db.DB().WithContext(ctx).First(&hs, "id = ?", DefaultHouseholdID).Error; err != nil {
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

	var vs SettingsDB
	if err := s.db.DB().
		WithContext(ctx).
		First(&vs, "device_profile_id = ?", DefaultDeviceProfileID).
		Error; err != nil {
		return config.Config{}, fmt.Errorf("load voice settings: %w", err)
	}
	cfg.Voice = Config{
		Enabled:                 vs.Enabled == 1,
		MutedByDefault:          vs.Muted == 1,
		WakeWordModelID:         vs.WakeWordModelID,
		WakeWordPhrase:          vs.WakeWordPhrase,
		WakeSensitivity:         vs.WakeSensitivity,
		STTProviderID:           vs.STTProviderID,
		TTSProviderID:           vs.TTSProviderID,
		STTModelID:              vs.STTModelID,
		TTSModelID:              vs.TTSModelID,
		TTSVoiceID:              vs.TTSVoiceID,
		TTSEnabled:              vs.TTSEnabled == 1,
		TTSLocale:               vs.TTSLocale,
		TTSSpeed:                vs.TTSSpeed,
		TTSVolume:               vs.TTSVolume,
		PreferredAgentID:        vs.PreferredAgentID,
		CloudOptIn:              vs.CloudOptIn == 1,
		CommandProvidersEnabled: vs.CommandProvidersEnabled == 1,
		SensitiveOutputPolicy:   vs.SensitiveOutputPolicy,
		FollowupWindowSeconds:   vs.FollowupWindowSeconds,
		MicrophoneProfile:       vs.MicrophoneProfile,
	}

	var dbRooms []RoomDB
	if err := s.db.DB().WithContext(ctx).Order("sort_order").Find(&dbRooms).Error; err != nil {
		return config.Config{}, fmt.Errorf("load rooms: %w", err)
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

	var dbTiles []TileDB
	if err := s.db.DB().WithContext(ctx).Order("sort_order").Find(&dbTiles).Error; err != nil {
		return config.Config{}, fmt.Errorf("load tiles: %w", err)
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

	layout, err := s.DashboardRepo.WidgetLayout(ctx, "")
	if err == nil {
		cfg.Dashboard.SchemaVersion = layout.SchemaVersion
		cfg.Dashboard.DefaultScreen = layout.DefaultScreen
		cfg.Dashboard.ActiveScreen = layout.ActiveScreen
		cfg.Dashboard.DefaultVariant = layout.DefaultVariant
		cfg.Dashboard.Variants = layout.Variants
		cfg.Dashboard.Widgets = dashboardWidgetConfigs(layout.Widgets)
		cfg.Dashboard.Screens = make([]DashboardScreenConfig, 0, len(layout.Screens))
		for _, screen := range layout.Screens {
			cfg.Dashboard.Screens = append(cfg.Dashboard.Screens, DashboardScreenConfig{
				ID:             screen.ID,
				Label:          screen.Label,
				DefaultVariant: screen.DefaultVariant,
				Variants:       screen.Variants,
				Widgets:        dashboardWidgetConfigs(screen.Widgets),
			})
		}
	}

	cfg.Agents = nil
	cfg.Rooms = rooms
	cfg.Tiles = tiles

	if err := config.EnsureValidConfig(&cfg); err != nil {
		return config.Config{}, fmt.Errorf("validate store config: %w", err)
	}
	return cfg, nil
}

func (s *Store) SetupStatus(ctx context.Context) (SetupStatus, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	var hs HouseholdSettingsDB
	if err := s.db.DB().WithContext(ctx).First(&hs, "id = ?", DefaultHouseholdID).Error; err != nil {
		return SetupStatus{}, fmt.Errorf("load setup status: %w", err)
	}

	missing := missingConfigSetupFields(cfg, hs.SetupCompleted == 1)
	if missing == nil {
		missing = []string{}
	}
	return SetupStatus{
		Complete: hs.SetupCompleted == 1 && len(missing) == 0,
		Missing:  missing,
	}, nil
}

// Helpers

func WidgetCatalogForSeed() map[string]WidgetCatalogItem {
	items := WidgetCatalog()
	if len(items) == 0 {
		items = WidgetCatalog()
	}
	catalog := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		catalog[item.Kind] = item
	}
	return catalog
}

func dashboardWidgetConfigs(widgets []WidgetInstance) []DashboardWidgetConfig {
	result := make([]DashboardWidgetConfig, 0, len(widgets))
	for _, w := range widgets {
		result = append(result, DashboardWidgetConfig{
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
	return result
}

func setupStatusForSeed(cfg config.Config, bootstrapProvided bool) SetupStatus {
	missing := missingConfigSetupFields(cfg, bootstrapProvided)
	if missing == nil {
		missing = []string{}
	}
	return SetupStatus{
		Complete: bootstrapProvided && len(missing) == 0,
		Missing:  missing,
	}
}

func missingConfigSetupFields(cfg config.Config, confirmed bool) []string {
	if !confirmed {
		return []string{"home.name"}
	}

	var missing []string
	if strings.TrimSpace(cfg.Home.Name) == "" {
		missing = append(missing, "home.name")
	}
	return missing
}

func mustJSONString(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(bytes)
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
