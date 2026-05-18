package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jute-dash/internal/config"

	_ "modernc.org/sqlite"
)

const (
	defaultHouseholdID     = "default"
	defaultDeviceProfileID = "default-display"
	defaultLayoutProfileID = "default-dashboard"
)

type Store struct {
	db   *sql.DB
	path string
}

type InitResult struct {
	Config config.Config
	Setup  SetupStatus
	Seeded bool
}

type SetupStatus struct {
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing"`
}

type WidgetLayout struct {
	ProfileID string           `json:"profileId"`
	Widgets   []WidgetInstance `json:"widgets"`
}

type WidgetInstance struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	X        int            `json:"x"`
	Y        int            `json:"y"`
	W        int            `json:"w"`
	H        int            `json:"h"`
	MinW     int            `json:"minW"`
	MinH     int            `json:"minH"`
	Size     string         `json:"size"`
	Settings map[string]any `json:"settings"`
	Visible  bool           `json:"visible"`
}

func Open(dbPath string) (*Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &Store{db: db, path: dbPath}
	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Initialize(ctx context.Context, bootstrap config.Config, bootstrapProvided bool) (InitResult, error) {
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
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL
);`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	migrations := []migration{
		{version: 1, name: "initial runtime settings", sql: initialMigrationSQL},
	}
	for _, item := range migrations {
		applied, err := s.migrationApplied(ctx, item.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		if _, err := tx.ExecContext(ctx, item.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("run migration %d: %w", item.version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, item.version, item.name, nowUTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}
	return nil
}

func (s *Store) Config(ctx context.Context) (config.Config, error) {
	cfg := config.Default()

	var setupCompleted int
	if err := s.db.QueryRowContext(ctx, `
SELECT name, timezone, locale, display_theme, display_accent_color, display_idle_mode, setup_completed
FROM household_settings
WHERE id = ?`, defaultHouseholdID).Scan(
		&cfg.Home.Name,
		&cfg.Home.Timezone,
		&cfg.Home.Locale,
		&cfg.Display.Theme,
		&cfg.Display.AccentColor,
		&cfg.Display.IdleMode,
		&setupCompleted,
	); err != nil {
		return config.Config{}, fmt.Errorf("load household settings: %w", err)
	}

	if err := s.db.QueryRowContext(ctx, `
SELECT enabled, provider, location_name, latitude, longitude, temperature_unit, wind_speed_unit
FROM weather_settings
WHERE id = ?`, defaultHouseholdID).Scan(
		&cfg.Weather.Enabled,
		&cfg.Weather.Provider,
		&cfg.Weather.LocationName,
		&cfg.Weather.Latitude,
		&cfg.Weather.Longitude,
		&cfg.Weather.TemperatureUnit,
		&cfg.Weather.WindSpeedUnit,
	); err != nil {
		return config.Config{}, fmt.Errorf("load weather settings: %w", err)
	}

	agents, err := s.loadAgents(ctx)
	if err != nil {
		return config.Config{}, err
	}
	rooms, err := s.loadRooms(ctx)
	if err != nil {
		return config.Config{}, err
	}
	tiles, err := s.loadTiles(ctx)
	if err != nil {
		return config.Config{}, err
	}

	cfg.Agents = agents
	cfg.Rooms = rooms
	cfg.Tiles = tiles
	if err := config.Validate(cfg); err != nil {
		return config.Config{}, fmt.Errorf("validate store config: %w", err)
	}
	return cfg, nil
}

func (s *Store) SetupStatus(ctx context.Context) (SetupStatus, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	var setupCompleted int
	if err := s.db.QueryRowContext(ctx, `SELECT setup_completed FROM household_settings WHERE id = ?`, defaultHouseholdID).Scan(&setupCompleted); err != nil {
		return SetupStatus{}, fmt.Errorf("load setup status: %w", err)
	}
	missing := missingSetupFields(cfg, setupCompleted == 1)
	if missing == nil {
		missing = []string{}
	}
	return SetupStatus{
		Complete: setupCompleted == 1 && len(missing) == 0,
		Missing:  missing,
	}, nil
}

func (s *Store) WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = defaultLayoutProfileID
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, kind, title, x, y, w, h, min_w, min_h, size, settings_json, visible
FROM widget_instances
WHERE layout_profile_id = ?
ORDER BY sort_order, id`, profileID)
	if err != nil {
		return WidgetLayout{}, fmt.Errorf("load widget layout: %w", err)
	}
	defer rows.Close()

	layout := WidgetLayout{ProfileID: profileID, Widgets: []WidgetInstance{}}
	for rows.Next() {
		var widget WidgetInstance
		var settingsJSON string
		var visible int
		if err := rows.Scan(
			&widget.ID,
			&widget.Kind,
			&widget.Title,
			&widget.X,
			&widget.Y,
			&widget.W,
			&widget.H,
			&widget.MinW,
			&widget.MinH,
			&widget.Size,
			&settingsJSON,
			&visible,
		); err != nil {
			return WidgetLayout{}, fmt.Errorf("scan widget layout: %w", err)
		}
		widget.Visible = visible == 1
		widget.Settings = map[string]any{}
		if strings.TrimSpace(settingsJSON) != "" {
			if err := json.Unmarshal([]byte(settingsJSON), &widget.Settings); err != nil {
				return WidgetLayout{}, fmt.Errorf("decode widget settings for %s: %w", widget.ID, err)
			}
		}
		layout.Widgets = append(layout.Widgets, widget)
	}
	if err := rows.Err(); err != nil {
		return WidgetLayout{}, fmt.Errorf("iterate widget layout: %w", err)
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
			Settings: map[string]any{},
			Visible:  widget.visible,
		})
	}
	return layout
}

func (s *Store) configure(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("set sqlite WAL mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	return nil
}

func (s *Store) migrationApplied(ctx context.Context, version int) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %d: %w", version, err)
	}
	return count > 0, nil
}

func (s *Store) isSeeded(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM household_settings`).Scan(&count); err != nil {
		return false, fmt.Errorf("check seeded store: %w", err)
	}
	return count > 0, nil
}

func (s *Store) seed(ctx context.Context, cfg config.Config, bootstrapProvided bool) error {
	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("validate seed config: %w", err)
	}

	setup := setupStatusForSeed(cfg, bootstrapProvided)
	now := nowUTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO household_settings (
  id, name, timezone, locale, display_theme, display_accent_color, display_idle_mode,
  setup_completed, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		defaultHouseholdID,
		cfg.Home.Name,
		cfg.Home.Timezone,
		cfg.Home.Locale,
		cfg.Display.Theme,
		cfg.Display.AccentColor,
		cfg.Display.IdleMode,
		boolToInt(setup.Complete),
		now,
		now,
	); err != nil {
		return fmt.Errorf("seed household settings: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO weather_settings (
  id, enabled, provider, location_name, latitude, longitude, temperature_unit, wind_speed_unit, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		defaultHouseholdID,
		boolToInt(cfg.Weather.Enabled),
		cfg.Weather.Provider,
		cfg.Weather.LocationName,
		cfg.Weather.Latitude,
		cfg.Weather.Longitude,
		cfg.Weather.TemperatureUnit,
		cfg.Weather.WindSpeedUnit,
		now,
	); err != nil {
		return fmt.Errorf("seed weather settings: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO device_profiles (id, name, interaction_mode, layout_profile_id, settings_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		defaultDeviceProfileID,
		"Default Display",
		"touch",
		defaultLayoutProfileID,
		"{}",
		now,
		now,
	); err != nil {
		return fmt.Errorf("seed device profile: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO layout_profiles (id, device_profile_id, name, settings_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		defaultLayoutProfileID,
		defaultDeviceProfileID,
		"Default Dashboard",
		"{}",
		now,
		now,
	); err != nil {
		return fmt.Errorf("seed layout profile: %w", err)
	}

	if err = seedAgents(ctx, tx, cfg.Agents, now); err != nil {
		return err
	}
	if err = seedRooms(ctx, tx, cfg.Rooms, now); err != nil {
		return err
	}
	if err = seedTiles(ctx, tx, cfg.Tiles, now); err != nil {
		return err
	}
	if err = seedDefaultWidgets(ctx, tx, now); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit seed: %w", err)
	}
	return nil
}

func seedAgents(ctx context.Context, tx *sql.Tx, agents []config.AgentConfig, now string) error {
	for _, agent := range agents {
		capabilities, err := jsonString(agent.Capabilities)
		if err != nil {
			return fmt.Errorf("encode agent capabilities: %w", err)
		}
		authType := ""
		authEnvToken := ""
		if agent.Auth != nil {
			authType = agent.Auth.Type
			authEnvToken = agent.Auth.EnvToken
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO agents (
  id, name, description, card_url, endpoint_url, protocol_binding, enabled,
  capabilities_json, auth_type, auth_env_token, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			agent.ID,
			agent.Name,
			agent.Description,
			agent.CardURL,
			agent.EndpointURL,
			agent.ProtocolBinding,
			boolToInt(agent.Enabled),
			capabilities,
			authType,
			authEnvToken,
			now,
			now,
		); err != nil {
			return fmt.Errorf("seed agent %s: %w", agent.ID, err)
		}
	}
	return nil
}

func seedRooms(ctx context.Context, tx *sql.Tx, rooms []config.RoomConfig, now string) error {
	for i, room := range rooms {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO rooms (id, name, summary, status, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			room.ID,
			room.Name,
			room.Summary,
			room.Status,
			i,
			now,
			now,
		); err != nil {
			return fmt.Errorf("seed room %s: %w", room.ID, err)
		}
	}
	return nil
}

func seedTiles(ctx context.Context, tx *sql.Tx, tiles []config.TileConfig, now string) error {
	for i, tile := range tiles {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO tiles (id, kind, label, value, detail, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			tile.ID,
			tile.Kind,
			tile.Label,
			tile.Value,
			tile.Detail,
			i,
			now,
			now,
		); err != nil {
			return fmt.Errorf("seed tile %s: %w", tile.ID, err)
		}
	}
	return nil
}

func seedDefaultWidgets(ctx context.Context, tx *sql.Tx, now string) error {
	for i, widget := range defaultWidgetInstances() {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO widget_instances (
  id, kind, title, layout_profile_id, x, y, w, h, min_w, min_h, size, settings_json,
  visible, sort_order, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			widget.id,
			widget.kind,
			widget.title,
			defaultLayoutProfileID,
			widget.x,
			widget.y,
			widget.w,
			widget.h,
			widget.minW,
			widget.minH,
			widget.size,
			"{}",
			boolToInt(widget.visible),
			i,
			now,
			now,
		); err != nil {
			return fmt.Errorf("seed widget %s: %w", widget.id, err)
		}
	}
	return nil
}

type defaultWidgetInstance struct {
	id      string
	kind    string
	title   string
	x       int
	y       int
	w       int
	h       int
	minW    int
	minH    int
	size    string
	visible bool
}

func defaultWidgetInstances() []defaultWidgetInstance {
	return []defaultWidgetInstance{
		{id: "date-time", kind: "date-time", title: "Date & Time", x: 0, y: 0, w: 2, h: 1, minW: 1, minH: 1, size: "wide", visible: true},
		{id: "weather", kind: "weather", title: "Weather", x: 2, y: 0, w: 2, h: 1, minW: 1, minH: 1, size: "wide", visible: true},
		{id: "chat-history", kind: "chat-history", title: "Chat History", x: 0, y: 1, w: 2, h: 2, minW: 1, minH: 1, size: "medium", visible: true},
	}
}

func (s *Store) loadAgents(ctx context.Context) ([]config.AgentConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, description, card_url, endpoint_url, protocol_binding, enabled,
       capabilities_json, auth_type, auth_env_token
FROM agents
ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}
	defer rows.Close()

	var agents []config.AgentConfig
	for rows.Next() {
		var agent config.AgentConfig
		var enabled int
		var capabilitiesJSON string
		var authType string
		var authEnvToken string
		if err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&agent.Description,
			&agent.CardURL,
			&agent.EndpointURL,
			&agent.ProtocolBinding,
			&enabled,
			&capabilitiesJSON,
			&authType,
			&authEnvToken,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if strings.TrimSpace(capabilitiesJSON) != "" {
			if err := json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities); err != nil {
				return nil, fmt.Errorf("decode agent capabilities for %s: %w", agent.ID, err)
			}
		}
		agent.Enabled = enabled == 1
		if strings.TrimSpace(authType) != "" || strings.TrimSpace(authEnvToken) != "" {
			agent.Auth = &config.AuthConfig{Type: authType, EnvToken: authEnvToken}
		}
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}
	return agents, nil
}

func (s *Store) loadRooms(ctx context.Context) ([]config.RoomConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, summary, status FROM rooms ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("load rooms: %w", err)
	}
	defer rows.Close()

	var rooms []config.RoomConfig
	for rows.Next() {
		var room config.RoomConfig
		if err := rows.Scan(&room.ID, &room.Name, &room.Summary, &room.Status); err != nil {
			return nil, fmt.Errorf("scan room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rooms: %w", err)
	}
	return rooms, nil
}

func (s *Store) loadTiles(ctx context.Context) ([]config.TileConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, label, value, detail FROM tiles ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("load tiles: %w", err)
	}
	defer rows.Close()

	var tiles []config.TileConfig
	for rows.Next() {
		var tile config.TileConfig
		if err := rows.Scan(&tile.ID, &tile.Kind, &tile.Label, &tile.Value, &tile.Detail); err != nil {
			return nil, fmt.Errorf("scan tile: %w", err)
		}
		tiles = append(tiles, tile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tiles: %w", err)
	}
	return tiles, nil
}

func setupStatusForSeed(cfg config.Config, bootstrapProvided bool) SetupStatus {
	missing := missingSetupFields(cfg, bootstrapProvided)
	if missing == nil {
		missing = []string{}
	}
	return SetupStatus{
		Complete: bootstrapProvided && len(missing) == 0,
		Missing:  missing,
	}
}

func missingSetupFields(cfg config.Config, confirmed bool) []string {
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

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

type migration struct {
	version int
	name    string
	sql     string
}

const initialMigrationSQL = `
CREATE TABLE IF NOT EXISTS household_settings (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  timezone TEXT NOT NULL,
  locale TEXT NOT NULL,
  display_theme TEXT NOT NULL,
  display_accent_color TEXT NOT NULL,
  display_idle_mode TEXT NOT NULL,
  setup_completed INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS weather_settings (
  id TEXT PRIMARY KEY,
  enabled INTEGER NOT NULL,
  provider TEXT NOT NULL,
  location_name TEXT NOT NULL,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  temperature_unit TEXT NOT NULL,
  wind_speed_unit TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS device_profiles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  interaction_mode TEXT NOT NULL,
  layout_profile_id TEXT NOT NULL,
  settings_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS layout_profiles (
  id TEXT PRIMARY KEY,
  device_profile_id TEXT NOT NULL,
  name TEXT NOT NULL,
  settings_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (device_profile_id) REFERENCES device_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rooms (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  summary TEXT NOT NULL,
  status TEXT NOT NULL,
  sort_order INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tiles (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  label TEXT NOT NULL,
  value TEXT NOT NULL,
  detail TEXT NOT NULL,
  sort_order INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  card_url TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  protocol_binding TEXT NOT NULL,
  enabled INTEGER NOT NULL,
  capabilities_json TEXT NOT NULL,
  auth_type TEXT NOT NULL DEFAULT '',
  auth_env_token TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_card_cache (
  agent_id TEXT PRIMARY KEY,
  card_json TEXT NOT NULL,
  etag TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS widget_packs (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  manifest_json TEXT NOT NULL,
  installed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS widget_instances (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  layout_profile_id TEXT NOT NULL,
  x INTEGER NOT NULL,
  y INTEGER NOT NULL,
  w INTEGER NOT NULL,
  h INTEGER NOT NULL,
  min_w INTEGER NOT NULL,
  min_h INTEGER NOT NULL,
  size TEXT NOT NULL,
  settings_json TEXT NOT NULL,
  visible INTEGER NOT NULL,
  sort_order INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (layout_profile_id) REFERENCES layout_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS widget_permissions (
  widget_instance_id TEXT NOT NULL,
  permission TEXT NOT NULL,
  granted INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (widget_instance_id, permission),
  FOREIGN KEY (widget_instance_id) REFERENCES widget_instances(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS voice_provider_packs (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  kind TEXT NOT NULL,
  transport_type TEXT NOT NULL,
  manifest_json TEXT NOT NULL,
  health_status TEXT NOT NULL,
  installed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS voice_settings (
  device_profile_id TEXT PRIMARY KEY,
  wake_word_model_id TEXT NOT NULL DEFAULT '',
  stt_provider_id TEXT NOT NULL DEFAULT '',
  tts_provider_id TEXT NOT NULL DEFAULT '',
  stt_model_id TEXT NOT NULL DEFAULT '',
  tts_model_id TEXT NOT NULL DEFAULT '',
  tts_voice_id TEXT NOT NULL DEFAULT '',
  cloud_opt_in INTEGER NOT NULL DEFAULT 0,
  command_providers_enabled INTEGER NOT NULL DEFAULT 0,
  sensitive_output_policy TEXT NOT NULL DEFAULT 'visual_only_sensitive',
  followup_window_seconds INTEGER NOT NULL DEFAULT 8,
  microphone_profile TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  FOREIGN KEY (device_profile_id) REFERENCES device_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS adapter_connections (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  settings_json TEXT NOT NULL,
  secret_ref_json TEXT NOT NULL,
  enabled INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS setting_audit_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  target TEXT NOT NULL,
  metadata_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
`
