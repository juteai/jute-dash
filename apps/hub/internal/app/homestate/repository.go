package homestate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db     *gorm.DB
	onSave func(ctx context.Context)
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SetOnSave(onSave func(ctx context.Context)) {
	r.onSave = onSave
}

func (r *Repository) SetupStatus(ctx context.Context) (SetupStatus, error) {
	var hs HouseholdSettingsDB
	if err := r.db.WithContext(ctx).First(&hs, "id = ?", DefaultHouseholdID).Error; err != nil {
		return SetupStatus{}, fmt.Errorf("load setup status: %w", err)
	}

	home := HomeConfig{
		Name: hs.Name,
	}

	missing := missingSetupFields(home, hs.SetupCompleted == 1)
	if missing == nil {
		missing = []string{}
	}
	return SetupStatus{
		Complete: hs.SetupCompleted == 1 && len(missing) == 0,
		Missing:  missing,
	}, nil
}

func (r *Repository) HouseholdSettings(ctx context.Context) (HouseholdSettings, error) {
	var hs HouseholdSettingsDB
	if err := r.db.WithContext(ctx).First(&hs, "id = ?", DefaultHouseholdID).Error; err != nil {
		return HouseholdSettings{}, fmt.Errorf("load household settings: %w", err)
	}

	var bg map[string]any
	if err := decodeJSONSetting(hs.DisplayBackgroundJSON, &bg); err != nil {
		return HouseholdSettings{}, fmt.Errorf("decode display background: %w", err)
	}
	var chrome map[string]any
	if err := decodeJSONSetting(hs.DisplayWidgetChromeJSON, &chrome); err != nil {
		return HouseholdSettings{}, fmt.Errorf("decode display widget chrome: %w", err)
	}

	display := DisplaySettings{
		Theme:        hs.DisplayTheme,
		ColorMode:    hs.DisplayColorMode,
		ThemeID:      hs.DisplayThemeID,
		Density:      hs.DisplayDensity,
		Motion:       hs.DisplayMotion,
		Background:   bg,
		WidgetChrome: chrome,
		AccentColor:  hs.DisplayAccentColor,
		IdleMode:     hs.DisplayIdleMode,
	}

	home := HomeConfig{
		Name: hs.Name,
	}

	setup, err := r.SetupStatus(ctx)
	if err != nil {
		return HouseholdSettings{}, err
	}

	return HouseholdSettings{
		Home:    home,
		Display: display,
		Setup:   setup,
	}, nil
}

func (r *Repository) Rooms(ctx context.Context) ([]RoomConfig, error) {
	return r.loadRooms(ctx)
}

func (r *Repository) SaveRooms(ctx context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
	normalized, err := normalizeRooms(rooms)
	if err != nil {
		return nil, err
	}
	now := nowUTC()

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	if r.onSave != nil {
		r.onSave(ctx)
	}
	return r.loadRooms(ctx)
}

func (r *Repository) Tiles(ctx context.Context) ([]TileConfig, error) {
	return r.loadTiles(ctx)
}

func (r *Repository) SaveTiles(ctx context.Context, tiles []TileConfig) ([]TileConfig, error) {
	normalized, err := normalizeTiles(tiles)
	if err != nil {
		return nil, err
	}
	now := nowUTC()

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	if r.onSave != nil {
		r.onSave(ctx)
	}
	return r.loadTiles(ctx)
}

func (r *Repository) SaveHouseholdSettings(ctx context.Context, settings HouseholdSettings) (HouseholdSettings, error) {
	var display DisplaySettings
	displayBytes, err := json.Marshal(settings.Display)
	if err == nil {
		_ = json.Unmarshal(displayBytes, &display)
	}

	backgroundJSON, err := jsonString(display.Background)
	if err != nil {
		return HouseholdSettings{}, fmt.Errorf("encode display background: %w", err)
	}
	widgetChromeJSON, err := jsonString(display.WidgetChrome)
	if err != nil {
		return HouseholdSettings{}, fmt.Errorf("encode display widget chrome: %w", err)
	}

	now := nowUTC()

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var hs HouseholdSettingsDB
		if err := tx.First(&hs, "id = ?", DefaultHouseholdID).Error; err != nil {
			return err
		}
		hs.Name = settings.Home.Name
		hs.DisplayTheme = display.Theme
		hs.DisplayColorMode = display.ColorMode
		hs.DisplayThemeID = display.ThemeID
		hs.DisplayDensity = display.Density
		hs.DisplayMotion = display.Motion
		hs.DisplayBackgroundJSON = backgroundJSON
		hs.DisplayWidgetChromeJSON = widgetChromeJSON
		hs.DisplayAccentColor = display.AccentColor
		hs.DisplayIdleMode = display.IdleMode
		hs.SetupCompleted = 1
		hs.UpdatedAt = now

		if err := tx.Save(&hs).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return HouseholdSettings{}, fmt.Errorf("save settings transaction: %w", err)
	}
	if r.onSave != nil {
		r.onSave(ctx)
	}
	return r.HouseholdSettings(ctx)
}

func (r *Repository) loadRooms(ctx context.Context) ([]RoomConfig, error) {
	var dbRooms []RoomDB
	if err := r.db.WithContext(ctx).Order("sort_order").Find(&dbRooms).Error; err != nil {
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

func (r *Repository) loadTiles(ctx context.Context) ([]TileConfig, error) {
	var dbTiles []TileDB
	if err := r.db.WithContext(ctx).Order("sort_order").Find(&dbTiles).Error; err != nil {
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

func (r *Repository) IsSeeded(ctx context.Context) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&HouseholdSettingsDB{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check seeded store: %w", err)
	}
	return count > 0, nil
}

// Helpers

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

func jsonString(value any) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func decodeJSONSetting(raw string, target any) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func missingSetupFields(home HomeConfig, confirmed bool) []string {
	if !confirmed {
		return []string{"home.name"}
	}

	var missing []string
	if strings.TrimSpace(home.Name) == "" {
		missing = append(missing, "home.name")
	}
	return missing
}
