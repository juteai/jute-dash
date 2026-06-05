package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"jute-dash/apps/hub/internal/app/homestate"
)

type Repository struct {
	db      *gorm.DB
	catalog map[string]WidgetCatalogItem
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:      db,
		catalog: widgetCatalogByKind(),
	}
}

func (r *Repository) SetCatalog(items []WidgetCatalogItem) {
	m := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		m[item.Kind] = item
	}
	r.catalog = m
}

func (r *Repository) CatalogByKind() map[string]WidgetCatalogItem {
	return r.catalog
}

func (r *Repository) WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = DefaultLayoutProfileID
	}

	var wDBs []WidgetInstanceDB
	if err := r.db.WithContext(ctx).
		Where("layout_profile_id = ?", profileID).
		Order("sort_order, id").
		Find(&wDBs).
		Error; err != nil {
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
			Mode:    normalizeMode(w.Mode),
			Visible: w.Visible == 1,
		}
		widget.Settings = map[string]any{}
		if strings.TrimSpace(w.SettingsJSON) != "" {
			if err := json.Unmarshal([]byte(w.SettingsJSON), &widget.Settings); err != nil {
				return WidgetLayout{}, fmt.Errorf("decode widget settings for %s: %w", widget.ID, err)
			}
		}
		if item, ok := r.catalog[widget.Kind]; ok {
			widget.Overflow = item.Overflow
		}
		layout.Widgets = append(layout.Widgets, widget)
	}
	return layout, nil
}

func (r *Repository) SaveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error) {
	normalized, err := NormalizeWidgetLayout(layout, r.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&homestate.LayoutProfileDB{}).
		Where("id = ?", normalized.ProfileID).
		Count(&count).
		Error; err != nil {
		return WidgetLayout{}, fmt.Errorf("check layout profile: %w", err)
	}
	if count == 0 {
		return WidgetLayout{}, fmt.Errorf("%w: layout profile not found", ErrInvalidLayout)
	}

	now := nowUTC()

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("layout_profile_id = ?", normalized.ProfileID).
			Delete(&WidgetInstanceDB{}).
			Error; err != nil {
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
				Mode:            normalizeMode(widget.Mode),
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

func (r *Repository) ResetWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	layout := DefaultWidgetLayout()
	if strings.TrimSpace(profileID) != "" {
		layout.ProfileID = strings.TrimSpace(profileID)
	}
	return r.SaveWidgetLayout(ctx, layout)
}

// Normalization & Defaults

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
		widget.Mode = strings.TrimSpace(widget.Mode)
		if widget.Mode == "" {
			widget.Mode = WidgetModeUI
		}

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
		ProfileID: DefaultLayoutProfileID,
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
			Mode:     WidgetModeUI,
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
			DefaultW:      6,
			DefaultH:      1,
			MinW:          3,
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
			DefaultW:      6,
			DefaultH:      1,
			MinW:          3,
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
			DefaultW:      6,
			DefaultH:      2,
			MinW:          3,
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
	if widget.W > BaseColumns || widget.MinW > BaseColumns || widget.X+widget.W > BaseColumns {
		return fmt.Errorf("%w: widget %s exceeds dashboard column bounds", ErrInvalidLayout, widget.ID)
	}
	if widget.H > 12 || widget.MinH > 12 || widget.Y > 99 {
		return fmt.Errorf("%w: widget %s exceeds dashboard row bounds", ErrInvalidLayout, widget.ID)
	}
	switch widget.Mode {
	case "", WidgetModeUI, WidgetModeHeadless:
	default:
		return fmt.Errorf("%w: widget %s has unsupported mode %q", ErrInvalidLayout, widget.ID, widget.Mode)
	}
	switch widget.Size {
	case "small", "medium", "wide", "large":
		return nil
	default:
		return fmt.Errorf("%w: widget %s has unsupported size %q", ErrInvalidLayout, widget.ID, widget.Size)
	}
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
			w:        6,
			h:        1,
			minW:     3,
			minH:     1,
			size:     "wide",
			overflow: "clip",
			visible:  true,
		},
		{
			id:       "weather",
			kind:     "weather",
			title:    "Weather",
			x:        6,
			y:        0,
			w:        6,
			h:        1,
			minW:     3,
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
			w:        6,
			h:        2,
			minW:     3,
			minH:     1,
			size:     "medium",
			overflow: "scroll",
			visible:  true,
		},
	}
}

// Global defaults

const DefaultLayoutProfileID = "default-dashboard"

// Shared helper replacements

func jsonString(value any) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func normalizeMode(mode string) string {
	if strings.TrimSpace(mode) == WidgetModeHeadless {
		return WidgetModeHeadless
	}
	return WidgetModeUI
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
