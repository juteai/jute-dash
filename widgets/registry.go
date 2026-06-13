package widgets

import (
	"context"
	"sync"

	"jute-dash/apps/hub/pkg/widgetskills"
)

type ActionWidget interface {
	Widget
	InvokeAction(
		ctx context.Context,
		snapshot widgetskills.Snapshot,
		instanceID string,
		actionID string,
		arguments map[string]any,
	) (map[string]any, error)
}

var (
	registryMu sync.RWMutex
	instances  = make(map[string]Widget)
)

func Register(w Widget) {
	registryMu.Lock()
	defer registryMu.Unlock()
	instances[w.Kind()] = w
}

// RegisterWithSkill registers a widget and its agent-facing skill in one call.
// Use this in widget init() functions instead of calling Register and
// widgetskills.Register separately.
func RegisterWithSkill(w Widget, contextFn widgetskills.ContextFunc) {
	Register(w)
	if skill := w.Skill(); skill != nil {
		widgetskills.Register(*skill, contextFn)
		if aw, ok := w.(ActionWidget); ok {
			widgetskills.RegisterAction(skill.SkillID, aw.InvokeAction)
		}
	}
}

// Catalog returns catalog metadata for all registered widgets.
func Catalog() []WidgetCatalogItem {
	registryMu.RLock()
	defer registryMu.RUnlock()
	items := make([]WidgetCatalogItem, 0, len(instances))
	for _, w := range instances {
		items = append(items, w.CatalogInfo())
	}
	return items
}

func Get(kind string) (Widget, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	w, exists := instances[kind]
	return w, exists
}

func List() []Widget {
	registryMu.RLock()
	defer registryMu.RUnlock()
	list := make([]Widget, 0, len(instances))
	for _, w := range instances {
		list = append(list, w)
	}
	return list
}
