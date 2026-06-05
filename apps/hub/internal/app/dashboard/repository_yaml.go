package dashboard

import (
	"context"
	"errors"
	"sync"
)

type YAMLRepository struct {
	mu         sync.RWMutex
	configPath string
	catalog    map[string]WidgetCatalogItem
	loadFn     func(path string) (DashboardConfig, error)
	saveFn     func(path string, cfg DashboardConfig) error
}

func NewYAMLRepository(
	configPath string,
	loadFn func(path string) (DashboardConfig, error),
	saveFn func(path string, cfg DashboardConfig) error,
) *YAMLRepository {
	return &YAMLRepository{
		configPath: configPath,
		catalog:    widgetCatalogByKind(),
		loadFn:     loadFn,
		saveFn:     saveFn,
	}
}

func (y *YAMLRepository) SetCatalog(items []WidgetCatalogItem) {
	y.mu.Lock()
	defer y.mu.Unlock()
	m := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		m[item.Kind] = item
	}
	y.catalog = m
}

func (y *YAMLRepository) load() (DashboardConfig, error) {
	if y.configPath == "" {
		return DashboardConfig{}, errors.New("config path is empty")
	}
	return y.loadFn(y.configPath)
}

func (y *YAMLRepository) save(cfg DashboardConfig) error {
	if y.configPath == "" {
		return errors.New("cannot save: config path is empty")
	}
	return y.saveFn(y.configPath, cfg)
}

func (y *YAMLRepository) WidgetLayout(_ context.Context, _ string) (WidgetLayout, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.load()
	if err != nil {
		return WidgetLayout{}, err
	}
	layout := WidgetLayout{
		ProfileID: DefaultLayoutProfileID,
		Widgets:   make([]WidgetInstance, 0, len(cfg.Widgets)),
	}
	for _, w := range cfg.Widgets {
		wi := WidgetInstance{
			ID:       w.ID,
			Kind:     w.Type,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			Mode:     normalizeMode(w.Mode),
			Settings: w.Settings,
			Visible:  w.Visible,
		}
		if wi.Settings == nil {
			wi.Settings = map[string]any{}
		}
		if item, ok := y.catalog[wi.Kind]; ok {
			wi.MinW = item.MinW
			wi.MinH = item.MinH
			wi.Size = item.DefaultSize
			wi.Overflow = item.Overflow
		}
		layout.Widgets = append(layout.Widgets, wi)
	}
	return layout, nil
}

func (y *YAMLRepository) SaveWidgetLayout(_ context.Context, layout WidgetLayout) (WidgetLayout, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	_, err := y.load()
	if err != nil {
		return WidgetLayout{}, err
	}
	normalized, err := NormalizeWidgetLayout(layout, y.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}
	widgets := make([]DashboardWidgetConfig, 0, len(normalized.Widgets))
	for _, w := range normalized.Widgets {
		widgets = append(widgets, DashboardWidgetConfig{
			ID:       w.ID,
			Type:     w.Kind,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			Visible:  w.Visible,
			Mode:     w.Mode,
			Settings: w.Settings,
		})
	}
	cfg := DashboardConfig{Widgets: widgets}
	if err := y.save(cfg); err != nil {
		return WidgetLayout{}, err
	}
	return normalized, nil
}

func (y *YAMLRepository) ResetWidgetLayout(_ context.Context, profileID string) (WidgetLayout, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	layout := DefaultWidgetLayout()
	if profileID != "" {
		layout.ProfileID = profileID
	}
	widgets := make([]DashboardWidgetConfig, 0, len(layout.Widgets))
	for _, w := range layout.Widgets {
		widgets = append(widgets, DashboardWidgetConfig{
			ID:       w.ID,
			Type:     w.Kind,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			Visible:  w.Visible,
			Mode:     w.Mode,
			Settings: w.Settings,
		})
	}
	cfg := DashboardConfig{Widgets: widgets}
	if err := y.save(cfg); err != nil {
		return WidgetLayout{}, err
	}
	return layout, nil
}
