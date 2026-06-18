package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"jute-dash/apps/hub/internal/app/homestate"
)

type Repository struct {
	db          *gorm.DB
	catalog     map[string]WidgetCatalogItem
	onSave      func(ctx context.Context)
	configStore any
}

const (
	LayoutSchemaVersion    = 3
	DefaultDashboardScreen = "home"
	DefaultLayoutVariant   = "tablet-landscape"
	defaultLayoutGridGap   = 12
	minLayoutGridSize      = 1
	maxLayoutGridSize      = 24
)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:      db,
		catalog: widgetCatalogByKind(),
	}
}

func (r *Repository) SetOnSave(onSave func(ctx context.Context)) {
	r.onSave = onSave
}

func (r *Repository) SetConfigStore(cs any) {
	r.configStore = cs
}

func (r *Repository) ConfigStore() any {
	return r.configStore
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
	var profileSettings string
	var profile homestate.LayoutProfileDB
	if err := r.db.WithContext(ctx).
		Where("id = ?", profileID).
		First(&profile).
		Error; err == nil {
		profileSettings = profile.SettingsJSON
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return WidgetLayout{}, fmt.Errorf("load layout profile: %w", err)
	}
	for _, w := range wDBs {
		screenID := strings.TrimSpace(w.ScreenID)
		if screenID == "" {
			screenID = DefaultDashboardScreen
		}
		widget := WidgetInstance{
			ScreenID: screenID,
			ID:       w.ID,
			Kind:     w.Kind,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			MinW:     w.MinW,
			MinH:     w.MinH,
			Size:     w.Size,
			Mode:     normalizeMode(w.Mode),
			Visible:  w.Visible == 1,
		}
		widget.Settings = map[string]any{}
		if strings.TrimSpace(w.SettingsJSON) != "" {
			if err := json.Unmarshal([]byte(w.SettingsJSON), &widget.Settings); err != nil {
				return WidgetLayout{}, fmt.Errorf("decode widget settings for %s: %w", widget.ID, err)
			}
		}
		widget.ConnectionRefs = map[string]string{}
		if strings.TrimSpace(w.ConnectionRefsJSON) != "" {
			if err := json.Unmarshal([]byte(w.ConnectionRefsJSON), &widget.ConnectionRefs); err != nil {
				return WidgetLayout{}, fmt.Errorf("decode widget connection refs for %s: %w", widget.ID, err)
			}
		}
		if item, ok := r.catalog[widget.Kind]; ok {
			widget.Overflow = item.Overflow
		}
		layout.Widgets = append(layout.Widgets, widget)
	}
	applyLayoutProfileSettings(&layout, profileSettings)
	layout = EnsureLayoutScreens(layout)
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

		settingsJSON, err := layoutProfileSettingsJSON(normalized)
		if err != nil {
			return err
		}
		if err := tx.Model(&homestate.LayoutProfileDB{}).
			Where("id = ?", normalized.ProfileID).
			Updates(map[string]any{
				"settings_json": settingsJSON,
				"updated_at":    now,
			}).
			Error; err != nil {
			return fmt.Errorf("save layout profile settings: %w", err)
		}

		sortOrder := 0
		for _, screen := range normalized.Screens {
			for _, widget := range screen.Widgets {
				settingsJSON, err := jsonString(widget.Settings)
				if err != nil {
					return fmt.Errorf("encode widget settings for %s: %w", widget.ID, err)
				}
				connectionRefsJSON, err := jsonString(widget.ConnectionRefs)
				if err != nil {
					return fmt.Errorf("encode widget connection refs for %s: %w", widget.ID, err)
				}
				wDB := WidgetInstanceDB{
					ID:                 widget.ID,
					ScreenID:           screen.ID,
					Kind:               widget.Kind,
					Title:              widget.Title,
					LayoutProfileID:    normalized.ProfileID,
					X:                  widget.X,
					Y:                  widget.Y,
					W:                  widget.W,
					H:                  widget.H,
					MinW:               widget.MinW,
					MinH:               widget.MinH,
					Size:               widget.Size,
					Mode:               normalizeMode(widget.Mode),
					SettingsJSON:       settingsJSON,
					ConnectionRefsJSON: connectionRefsJSON,
					Visible:            boolToInt(widget.Visible),
					SortOrder:          sortOrder,
					CreatedAt:          now,
					UpdatedAt:          now,
				}
				sortOrder++
				if err := tx.Create(&wDB).Error; err != nil {
					return fmt.Errorf("save widget %s: %w", widget.ID, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return WidgetLayout{}, err
	}
	if r.onSave != nil {
		r.onSave(ctx)
	}

	return normalized, nil
}

func (r *Repository) SetActiveScreen(ctx context.Context, profileID string, screenID string) (WidgetLayout, error) {
	layout, err := r.WidgetLayout(ctx, profileID)
	if err != nil {
		return WidgetLayout{}, err
	}
	screenID = strings.TrimSpace(screenID)
	if !hasScreen(layout, screenID) {
		return WidgetLayout{}, fmt.Errorf("%w: active screen %q is missing", ErrInvalidLayout, screenID)
	}
	layout.ActiveScreen = screenID
	for _, screen := range layout.Screens {
		if screen.ID == screenID {
			layout.Widgets = screen.Widgets
			layout.DefaultVariant = screen.DefaultVariant
			layout.Variants = screen.Variants
			break
		}
	}
	return r.SaveWidgetLayout(ctx, layout)
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
	layout = applyCompatibilityFieldsToScreens(layout)
	layout = EnsureLayoutScreens(layout)
	seenIDs := map[string]bool{}
	seenSingleInstanceKinds := map[string]string{}

	for screenIndex := range layout.Screens {
		screen := &layout.Screens[screenIndex]
		for i := range screen.Widgets {
			widget := &screen.Widgets[i]
			widget.ScreenID = screen.ID
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
				if !item.AllowMultiple {
					if firstScreen, exists := seenSingleInstanceKinds[widget.Kind]; exists {
						return WidgetLayout{}, fmt.Errorf(
							"%w: duplicate widget kind %q across screens %q and %q",
							ErrInvalidLayout,
							widget.Kind,
							firstScreen,
							screen.ID,
						)
					}
					seenSingleInstanceKinds[widget.Kind] = screen.ID
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
			if err := validateWidgetInstance(*widget); err != nil {
				return WidgetLayout{}, err
			}
			if widget.Settings == nil {
				widget.Settings = map[string]any{}
			}
			if widget.ConnectionRefs == nil {
				widget.ConnectionRefs = map[string]string{}
			}
			if _, err := json.Marshal(widget.Settings); err != nil {
				return WidgetLayout{}, fmt.Errorf(
					"%w: widget %s settings are not JSON serializable",
					ErrInvalidLayout,
					widget.ID,
				)
			}
			if _, err := json.Marshal(widget.ConnectionRefs); err != nil {
				return WidgetLayout{}, fmt.Errorf(
					"%w: widget %s connection refs are not JSON serializable",
					ErrInvalidLayout,
					widget.ID,
				)
			}
		}
	}
	layout = syncFlattenedWidgets(layout)
	if err := validateLayoutVariants(layout); err != nil {
		return WidgetLayout{}, err
	}
	return layout, nil
}

func applyCompatibilityFieldsToScreens(layout WidgetLayout) WidgetLayout {
	if len(layout.Screens) == 0 {
		return layout
	}
	activeID := strings.TrimSpace(layout.ActiveScreen)
	if activeID == "" {
		activeID = strings.TrimSpace(layout.DefaultScreen)
	}
	if activeID == "" {
		activeID = layout.Screens[0].ID
	}
	for i := range layout.Screens {
		if layout.Screens[i].ID != activeID {
			continue
		}
		appliedWidgets := false
		if len(layout.Widgets) > 0 {
			screenWidgets := make([]WidgetInstance, 0, len(layout.Widgets))
			for _, widget := range layout.Widgets {
				if strings.TrimSpace(widget.ScreenID) == "" || widget.ScreenID == activeID {
					widget.ScreenID = activeID
					screenWidgets = append(screenWidgets, widget)
				}
			}
			if len(screenWidgets) > 0 {
				layout.Screens[i].Widgets = screenWidgets
				appliedWidgets = true
			}
		}
		if appliedWidgets && strings.TrimSpace(layout.DefaultVariant) != "" {
			layout.Screens[i].DefaultVariant = layout.DefaultVariant
		}
		if appliedWidgets && len(layout.Variants) > 0 {
			layout.Screens[i].Variants = layout.Variants
		}
		break
	}
	return layout
}

func EnsureLayoutScreens(layout WidgetLayout) WidgetLayout {
	if len(layout.Screens) == 0 {
		widgets := layout.Widgets
		if widgets == nil {
			widgets = []WidgetInstance{}
		}
		screen := DashboardScreen{
			ID:             DefaultDashboardScreen,
			Label:          "Home",
			DefaultVariant: layout.DefaultVariant,
			Variants:       layout.Variants,
			Widgets:        widgets,
		}
		layout.Screens = []DashboardScreen{screen}
	}
	if strings.TrimSpace(layout.DefaultScreen) == "" {
		layout.DefaultScreen = layout.Screens[0].ID
	}
	if strings.TrimSpace(layout.ActiveScreen) == "" {
		layout.ActiveScreen = layout.DefaultScreen
	}
	if layout.SchemaVersion < LayoutSchemaVersion {
		layout.SchemaVersion = LayoutSchemaVersion
	}
	seen := map[string]bool{}
	for i := range layout.Screens {
		screen := &layout.Screens[i]
		screen.ID = strings.TrimSpace(screen.ID)
		if screen.ID == "" {
			screen.ID = DefaultDashboardScreen
		}
		screen.Label = strings.TrimSpace(screen.Label)
		if screen.Label == "" {
			screen.Label = screen.ID
		}
		for widgetIndex := range screen.Widgets {
			screen.Widgets[widgetIndex].ScreenID = screen.ID
		}
		screenLayout := WidgetLayout{
			ProfileID:      layout.ProfileID,
			SchemaVersion:  layout.SchemaVersion,
			DefaultVariant: screen.DefaultVariant,
			Variants:       screen.Variants,
			Widgets:        screen.Widgets,
		}
		screenLayout = EnsureLayoutVariants(screenLayout)
		screen.DefaultVariant = screenLayout.DefaultVariant
		screen.Variants = screenLayout.Variants
		if !seen[screen.ID] {
			seen[screen.ID] = true
		}
	}
	if !seen[layout.DefaultScreen] {
		layout.DefaultScreen = layout.Screens[0].ID
	}
	if !seen[layout.ActiveScreen] {
		layout.ActiveScreen = layout.DefaultScreen
	}
	active := activeDashboardScreen(layout)
	layout.DefaultVariant = active.DefaultVariant
	layout.Variants = active.Variants
	return syncFlattenedWidgets(layout)
}

func syncFlattenedWidgets(layout WidgetLayout) WidgetLayout {
	widgets := []WidgetInstance{}
	for i := range layout.Screens {
		screenID := layout.Screens[i].ID
		for j := range layout.Screens[i].Widgets {
			layout.Screens[i].Widgets[j].ScreenID = screenID
			widgets = append(widgets, layout.Screens[i].Widgets[j])
		}
	}
	layout.Widgets = widgets
	return layout
}

func activeDashboardScreen(layout WidgetLayout) DashboardScreen {
	for _, screen := range layout.Screens {
		if screen.ID == layout.ActiveScreen {
			return screen
		}
	}
	if len(layout.Screens) > 0 {
		return layout.Screens[0]
	}
	return DashboardScreen{
		ID:             DefaultDashboardScreen,
		Label:          "Home",
		DefaultVariant: DefaultLayoutVariant,
		Widgets:        []WidgetInstance{},
	}
}

func hasScreen(layout WidgetLayout, screenID string) bool {
	screenID = strings.TrimSpace(screenID)
	for _, screen := range layout.Screens {
		if screen.ID == screenID {
			return true
		}
	}
	return false
}

func EnsureLayoutVariants(layout WidgetLayout) WidgetLayout {
	if layout.SchemaVersion == 0 {
		layout.SchemaVersion = 2
	}
	if strings.TrimSpace(layout.DefaultVariant) == "" {
		layout.DefaultVariant = DefaultLayoutVariant
	}
	existing := map[string]LayoutVariant{}
	for _, variant := range layout.Variants {
		existing[strings.TrimSpace(variant.ID)] = variant
	}
	presets := []LayoutVariant{
		{
			ID:          "phone",
			Label:       "Phone",
			MinWidth:    0,
			Orientation: "any",
			Columns:     1,
			Rows:        8,
			Gap:         defaultLayoutGridGap,
		},
		{
			ID:          "tablet-portrait",
			Label:       "Tablet",
			MinWidth:    641,
			Orientation: "portrait",
			Columns:     6,
			Rows:        10,
			Gap:         defaultLayoutGridGap,
		},
		{
			ID:          "tablet-landscape",
			Label:       "Tablet wide",
			MinWidth:    768,
			Orientation: "landscape",
			Columns:     10,
			Rows:        6,
			Gap:         defaultLayoutGridGap,
		},
		{
			ID:          "desktop",
			Label:       "Desktop",
			MinWidth:    1024,
			Orientation: "any",
			Columns:     12,
			Rows:        8,
			Gap:         defaultLayoutGridGap,
		},
		{
			ID:          "wall",
			Label:       "Wall",
			MinWidth:    1600,
			Orientation: "landscape",
			Columns:     16,
			Rows:        9,
			Gap:         defaultLayoutGridGap,
		},
	}

	var variants []LayoutVariant
	seen := map[string]bool{}
	for _, preset := range presets {
		variant := preset
		if candidate, ok := existing[preset.ID]; ok {
			variant = mergeVariantPreset(preset, candidate)
		}
		variants = append(variants, normalizeLayoutVariant(variant, layout.Widgets))
		seen[preset.ID] = true
	}
	for _, variant := range layout.Variants {
		id := strings.TrimSpace(variant.ID)
		if id == "" || seen[id] {
			continue
		}
		variants = append(variants, normalizeLayoutVariant(variant, layout.Widgets))
		seen[id] = true
	}
	layout.Variants = variants
	if _, ok := seen[layout.DefaultVariant]; !ok && len(layout.Variants) > 0 {
		layout.DefaultVariant = layout.Variants[0].ID
	}
	return layout
}

func mergeVariantPreset(preset, variant LayoutVariant) LayoutVariant {
	if strings.TrimSpace(variant.Label) == "" {
		variant.Label = preset.Label
	}
	if variant.Columns == 0 {
		variant.Columns = preset.Columns
	}
	if variant.Rows == 0 {
		variant.Rows = preset.Rows
	}
	if variant.Gap == 0 {
		variant.Gap = preset.Gap
	}
	if strings.TrimSpace(variant.Orientation) == "" {
		variant.Orientation = preset.Orientation
	}
	if variant.MinWidth == 0 {
		variant.MinWidth = preset.MinWidth
	}
	return variant
}

func normalizeLayoutVariant(variant LayoutVariant, widgets []WidgetInstance) LayoutVariant {
	variant.ID = strings.TrimSpace(variant.ID)
	variant.Label = strings.TrimSpace(variant.Label)
	if variant.Label == "" {
		variant.Label = variant.ID
	}
	variant.Orientation = strings.TrimSpace(variant.Orientation)
	if variant.Orientation == "" {
		variant.Orientation = "any"
	}
	variant.MinWidth = maxInt(0, variant.MinWidth)
	variant.MinHeight = maxInt(0, variant.MinHeight)
	variant.Columns = clampInt(variant.Columns, minLayoutGridSize, maxLayoutGridSize)
	variant.Rows = clampInt(variant.Rows, minLayoutGridSize, maxLayoutGridSize)
	if variant.Gap == 0 {
		variant.Gap = defaultLayoutGridGap
	}
	if variant.Placements == nil {
		variant.Placements = map[string]WidgetPlacement{}
	}
	for _, widget := range widgets {
		placement, ok := variant.Placements[widget.ID]
		if !ok {
			placement = WidgetPlacement{X: widget.X, Y: widget.Y, W: widget.W, H: widget.H}
		}
		variant.Placements[widget.ID] = clampWidgetPlacement(placement, widget, variant.Columns, variant.Rows)
	}
	return variant
}

func clampWidgetPlacement(
	placement WidgetPlacement,
	widget WidgetInstance,
	columns int,
	rows int,
) WidgetPlacement {
	minW := clampInt(widget.MinW, 1, columns)
	minH := clampInt(widget.MinH, 1, rows)
	width := clampInt(placement.W, minW, columns)
	height := clampInt(placement.H, minH, rows)
	return WidgetPlacement{
		X:      clampInt(placement.X, 0, columns-width),
		Y:      clampInt(placement.Y, 0, rows-height),
		W:      width,
		H:      height,
		Hidden: placement.Hidden,
	}
}

func validateLayoutVariants(layout WidgetLayout) error {
	if layout.SchemaVersion < LayoutSchemaVersion {
		return fmt.Errorf("%w: unsupported layout schema version", ErrInvalidLayout)
	}
	if len(layout.Screens) == 0 {
		return fmt.Errorf("%w: at least one screen is required", ErrInvalidLayout)
	}
	seenScreens := map[string]bool{}
	for _, screen := range layout.Screens {
		if screen.ID == "" {
			return fmt.Errorf("%w: screen id is required", ErrInvalidLayout)
		}
		if seenScreens[screen.ID] {
			return fmt.Errorf("%w: duplicate screen %q", ErrInvalidLayout, screen.ID)
		}
		seenScreens[screen.ID] = true
		if err := validateScreenVariants(screen); err != nil {
			return err
		}
	}
	if !seenScreens[layout.DefaultScreen] {
		return fmt.Errorf("%w: default screen is missing", ErrInvalidLayout)
	}
	if !seenScreens[layout.ActiveScreen] {
		return fmt.Errorf("%w: active screen is missing", ErrInvalidLayout)
	}
	return nil
}

func validateScreenVariants(screen DashboardScreen) error {
	seen := map[string]bool{}
	widgetsByID := map[string]WidgetInstance{}
	for _, widget := range screen.Widgets {
		widgetsByID[widget.ID] = widget
	}
	for _, variant := range screen.Variants {
		if variant.ID == "" {
			return fmt.Errorf("%w: layout variant id is required", ErrInvalidLayout)
		}
		if seen[variant.ID] {
			return fmt.Errorf("%w: duplicate layout variant %q on screen %q", ErrInvalidLayout, variant.ID, screen.ID)
		}
		seen[variant.ID] = true
		if variant.Columns < minLayoutGridSize || variant.Columns > maxLayoutGridSize ||
			variant.Rows < minLayoutGridSize || variant.Rows > maxLayoutGridSize {
			return fmt.Errorf("%w: layout variant %q grid size is invalid", ErrInvalidLayout, variant.ID)
		}
		switch variant.Orientation {
		case "", "any", "portrait", "landscape":
		default:
			return fmt.Errorf("%w: layout variant %q orientation is invalid", ErrInvalidLayout, variant.ID)
		}
		for widgetID, placement := range variant.Placements {
			widget, ok := widgetsByID[widgetID]
			if !ok {
				return fmt.Errorf(
					"%w: layout variant %q references unknown widget %q",
					ErrInvalidLayout,
					variant.ID,
					widgetID,
				)
			}
			minW := clampInt(widget.MinW, 1, variant.Columns)
			minH := clampInt(widget.MinH, 1, variant.Rows)
			if placement.W < minW || placement.H < minH ||
				placement.X < 0 || placement.Y < 0 ||
				placement.X+placement.W > variant.Columns ||
				placement.Y+placement.H > variant.Rows {
				return fmt.Errorf(
					"%w: layout variant %q placement for %q is out of bounds",
					ErrInvalidLayout,
					variant.ID,
					widgetID,
				)
			}
		}
	}
	if !seen[screen.DefaultVariant] {
		return fmt.Errorf("%w: default layout variant is missing", ErrInvalidLayout)
	}
	return nil
}

type layoutProfileSettings struct {
	SchemaVersion  int              `json:"schemaVersion,omitempty"`
	DefaultScreen  string           `json:"defaultScreenId,omitempty"`
	ActiveScreen   string           `json:"activeScreenId,omitempty"`
	Screens        []screenSettings `json:"screens,omitempty"`
	DefaultVariant string           `json:"defaultVariant,omitempty"`
	Variants       []LayoutVariant  `json:"variants,omitempty"`
}

type screenSettings struct {
	ID             string          `json:"id"`
	Label          string          `json:"label"`
	DefaultVariant string          `json:"defaultVariant,omitempty"`
	Variants       []LayoutVariant `json:"variants,omitempty"`
}

func applyLayoutProfileSettings(layout *WidgetLayout, raw string) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	var settings layoutProfileSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return
	}
	layout.SchemaVersion = settings.SchemaVersion
	layout.DefaultScreen = settings.DefaultScreen
	layout.ActiveScreen = settings.ActiveScreen
	if len(settings.Screens) > 0 {
		widgetsByScreen := map[string][]WidgetInstance{}
		for _, widget := range layout.Widgets {
			screenID := strings.TrimSpace(widget.ScreenID)
			if screenID == "" {
				screenID = DefaultDashboardScreen
			}
			widgetsByScreen[screenID] = append(widgetsByScreen[screenID], widget)
		}
		layout.Screens = make([]DashboardScreen, 0, len(settings.Screens))
		for _, screen := range settings.Screens {
			layout.Screens = append(layout.Screens, DashboardScreen{
				ID:             screen.ID,
				Label:          screen.Label,
				DefaultVariant: screen.DefaultVariant,
				Variants:       screen.Variants,
				Widgets:        widgetsByScreen[screen.ID],
			})
		}
	}
	layout.DefaultVariant = settings.DefaultVariant
	layout.Variants = settings.Variants
}

func layoutProfileSettingsJSON(layout WidgetLayout) (string, error) {
	screens := make([]screenSettings, 0, len(layout.Screens))
	for _, screen := range layout.Screens {
		screens = append(screens, screenSettings{
			ID:             screen.ID,
			Label:          screen.Label,
			DefaultVariant: screen.DefaultVariant,
			Variants:       screen.Variants,
		})
	}
	settings := layoutProfileSettings{
		SchemaVersion:  layout.SchemaVersion,
		DefaultScreen:  layout.DefaultScreen,
		ActiveScreen:   layout.ActiveScreen,
		Screens:        screens,
		DefaultVariant: layout.DefaultVariant,
		Variants:       layout.Variants,
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("encode layout profile settings: %w", err)
	}
	return string(raw), nil
}

func clampInt(value, lower, upper int) int {
	if upper < lower {
		return lower
	}
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func WidgetLayoutFromDashboardConfig(
	cfg DashboardConfig,
	catalog map[string]WidgetCatalogItem,
) (WidgetLayout, error) {
	layout := WidgetLayout{
		ProfileID:     DefaultLayoutProfileID,
		SchemaVersion: cfg.SchemaVersion,
		DefaultScreen: cfg.DefaultScreen,
		ActiveScreen:  cfg.ActiveScreen,
	}
	if len(cfg.Screens) > 0 {
		layout.Screens = make([]DashboardScreen, 0, len(cfg.Screens))
		for _, screenCfg := range cfg.Screens {
			screen := DashboardScreen{
				ID:             screenCfg.ID,
				Label:          screenCfg.Label,
				DefaultVariant: screenCfg.DefaultVariant,
				Variants:       screenCfg.Variants,
				Widgets:        dashboardWidgetsFromConfig(screenCfg.Widgets, catalog),
			}
			for i := range screen.Widgets {
				screen.Widgets[i].ScreenID = screen.ID
			}
			layout.Screens = append(layout.Screens, screen)
		}
		return NormalizeWidgetLayout(layout, catalog)
	}

	layout.DefaultVariant = cfg.DefaultVariant
	layout.Variants = cfg.Variants
	layout.Widgets = dashboardWidgetsFromConfig(cfg.Widgets, catalog)
	return NormalizeWidgetLayout(layout, catalog)
}

func dashboardWidgetsFromConfig(
	widgets []DashboardWidgetConfig,
	catalog map[string]WidgetCatalogItem,
) []WidgetInstance {
	result := make([]WidgetInstance, 0, len(widgets))
	legacyColumns := usesLegacyWidgetColumns(widgets, catalog)
	for _, w := range widgets {
		x := w.X
		width := w.W
		minW := w.MinW
		if legacyColumns {
			x *= LegacyColumnScale
			width *= LegacyColumnScale
			if minW > 0 {
				minW *= LegacyColumnScale
			}
		}
		if item, ok := catalog[w.Type]; ok {
			if width == 0 {
				width = item.DefaultW
			}
			if w.H == 0 {
				w.H = item.DefaultH
			}
		}
		result = append(result, WidgetInstance{
			ID:             w.ID,
			Kind:           w.Type,
			Title:          w.Title,
			X:              x,
			Y:              w.Y,
			W:              width,
			H:              w.H,
			MinW:           minW,
			MinH:           w.MinH,
			Size:           w.Size,
			Mode:           normalizeMode(w.Mode),
			Settings:       w.Settings,
			ConnectionRefs: w.ConnectionRefs,
			Visible:        w.Visible,
		})
	}
	return result
}

func usesLegacyWidgetColumns(widgets []DashboardWidgetConfig, catalog map[string]WidgetCatalogItem) bool {
	maxRight := 0
	hasTile := false
	for _, widget := range widgets {
		if normalizeMode(widget.Mode) == WidgetModeHeadless || !widget.Visible {
			continue
		}
		hasTile = true
		if right := widget.X + widget.W; right > maxRight {
			maxRight = right
		}
		if widget.MinW > 0 || widget.MinH > 0 || strings.TrimSpace(widget.Size) != "" {
			return false
		}
		if item, ok := catalog[widget.Type]; ok && widget.W > item.DefaultW {
			return false
		}
	}
	return hasTile && maxRight > 0 && maxRight <= 4
}

func DefaultWidgetLayout() WidgetLayout {
	widgets := defaultWidgetInstances()
	screenWidgets := make([]WidgetInstance, 0, len(widgets))
	layout := WidgetLayout{
		ProfileID:     DefaultLayoutProfileID,
		SchemaVersion: LayoutSchemaVersion,
		DefaultScreen: DefaultDashboardScreen,
		ActiveScreen:  DefaultDashboardScreen,
	}
	for _, widget := range widgets {
		screenWidgets = append(screenWidgets, WidgetInstance{
			ScreenID:       DefaultDashboardScreen,
			ID:             widget.id,
			Kind:           widget.kind,
			Title:          widget.title,
			X:              widget.x,
			Y:              widget.y,
			W:              widget.w,
			H:              widget.h,
			MinW:           widget.minW,
			MinH:           widget.minH,
			Size:           widget.size,
			Overflow:       widget.overflow,
			Mode:           WidgetModeUI,
			Settings:       map[string]any{},
			ConnectionRefs: map[string]string{},
			Visible:        widget.visible,
		})
	}
	layout.Screens = []DashboardScreen{{
		ID:      DefaultDashboardScreen,
		Label:   "Home",
		Widgets: screenWidgets,
	}}
	return EnsureLayoutScreens(layout)
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
		{
			id:       "spotify-widget-1",
			kind:     "spotify",
			title:    "Spotify",
			x:        6,
			y:        1,
			w:        6,
			h:        2,
			minW:     4,
			minH:     2,
			size:     "wide",
			overflow: "clip",
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
