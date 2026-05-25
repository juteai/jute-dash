package widgets

import (
	"sync"
)

var (
	registryMu sync.RWMutex
	instances  = make(map[string]Widget)
)

func Register(w Widget) {
	registryMu.Lock()
	defer registryMu.Unlock()
	instances[w.Kind()] = w
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
