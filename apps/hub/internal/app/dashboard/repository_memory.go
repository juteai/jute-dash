package dashboard

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	layout  WidgetLayout
	catalog map[string]WidgetCatalogItem
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		layout: WidgetLayout{
			ProfileID: DefaultLayoutProfileID,
			Widgets:   []WidgetInstance{},
		},
		catalog: widgetCatalogByKind(),
	}
}

func NewMemoryRepositoryWithLayout(layout WidgetLayout) *MemoryRepository {
	return &MemoryRepository{
		layout:  layout,
		catalog: widgetCatalogByKind(),
	}
}

func (m *MemoryRepository) SetCatalog(items []WidgetCatalogItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cat := make(map[string]WidgetCatalogItem, len(items))
	for _, item := range items {
		cat[item.Kind] = item
	}
	m.catalog = cat
}

func (m *MemoryRepository) WidgetLayout(_ context.Context, _ string) (WidgetLayout, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.layout, nil
}

func (m *MemoryRepository) SaveWidgetLayout(_ context.Context, layout WidgetLayout) (WidgetLayout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := NormalizeWidgetLayout(layout, m.catalog)
	if err != nil {
		return WidgetLayout{}, err
	}
	m.layout = normalized
	return m.layout, nil
}

func (m *MemoryRepository) ResetWidgetLayout(_ context.Context, profileID string) (WidgetLayout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	layout := DefaultWidgetLayout()
	if profileID != "" {
		layout.ProfileID = profileID
	}
	m.layout = layout
	return m.layout, nil
}
