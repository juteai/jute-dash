package repository

import (
	"context"
	"sync"
)

type MemoryDashboardRepository struct {
	mu      sync.RWMutex
	layout  WidgetLayout
	catalog map[string]WidgetCatalogItem
}

func NewMemoryDashboardRepository() *MemoryDashboardRepository {
	return &MemoryDashboardRepository{
		layout: WidgetLayout{
			ProfileID: DefaultLayoutProfileID,
			Widgets:   []WidgetInstance{},
		},
		catalog: widgetCatalogByKind(),
	}
}

func NewMemoryDashboardRepositoryWithLayout(layout WidgetLayout) *MemoryDashboardRepository {
	return &MemoryDashboardRepository{
		layout:  layout,
		catalog: widgetCatalogByKind(),
	}
}

func (m *MemoryDashboardRepository) SetCatalog(items []WidgetCatalogItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cat := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		cat[item.Kind] = item
	}
	m.catalog = cat
}

func (m *MemoryDashboardRepository) WidgetLayout(
	_ context.Context,
	_ string,
) (WidgetLayout, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return EnsureLayoutScreens(m.layout), nil
}

func (m *MemoryDashboardRepository) SaveWidgetLayout(
	_ context.Context,
	layout WidgetLayout,
) (WidgetLayout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := NormalizeWidgetLayout(layout, m.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}
	m.layout = normalized
	return m.layout, nil
}

func (m *MemoryDashboardRepository) ResetWidgetLayout(
	_ context.Context,
	profileID string,
) (WidgetLayout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	layout := DefaultWidgetLayout()
	if profileID != "" {
		layout.ProfileID = profileID
	}
	m.layout = layout
	return m.layout, nil
}

func (m *MemoryDashboardRepository) SetActiveScreen(
	_ context.Context,
	_ string,
	screenID string,
) (WidgetLayout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	layout := EnsureLayoutScreens(m.layout)
	if !hasScreen(layout, screenID) {
		return WidgetLayout{}, ErrInvalidLayout
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
	normalized, err := NormalizeWidgetLayout(layout, m.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}
	m.layout = normalized
	return m.layout, nil
}
