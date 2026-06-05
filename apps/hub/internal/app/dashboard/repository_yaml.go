package dashboard

import (
	"context"
	"sync"
)

// Syncer defines the interface needed for dashboard config persistence.
type Syncer interface {
	SyncDashboard(ctx context.Context, cfg DashboardConfig) error
	DashboardConfig(ctx context.Context) (DashboardConfig, error)
}

type YAMLRepository struct {
	mu      sync.RWMutex
	catalog map[string]WidgetCatalogItem
	syncer  Syncer
}

func NewYAMLRepository(syncer Syncer) *YAMLRepository {
	return &YAMLRepository{
		catalog: widgetCatalogByKind(),
		syncer:  syncer,
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

func (y *YAMLRepository) WidgetLayout(ctx context.Context, _ string) (WidgetLayout, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.syncer.DashboardConfig(ctx)
	if err != nil {
		return WidgetLayout{}, err
	}
	return WidgetLayoutFromDashboardConfig(cfg, y.catalog)
}

func (y *YAMLRepository) SaveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	_, err := y.syncer.DashboardConfig(ctx)
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
			MinW:     w.MinW,
			MinH:     w.MinH,
			Size:     w.Size,
			Visible:  w.Visible,
			Mode:     w.Mode,
			Settings: w.Settings,
		})
	}
	cfg := DashboardConfig{Widgets: widgets}
	if err := y.syncer.SyncDashboard(ctx, cfg); err != nil {
		return WidgetLayout{}, err
	}
	return normalized, nil
}

func (y *YAMLRepository) ResetWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
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
			MinW:     w.MinW,
			MinH:     w.MinH,
			Size:     w.Size,
			Visible:  w.Visible,
			Mode:     w.Mode,
			Settings: w.Settings,
		})
	}
	cfg := DashboardConfig{Widgets: widgets}
	if err := y.syncer.SyncDashboard(ctx, cfg); err != nil {
		return WidgetLayout{}, err
	}
	return layout, nil
}
