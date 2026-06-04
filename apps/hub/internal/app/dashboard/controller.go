package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"jute-dash/widgets"
)

type LayoutStore interface {
	WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error)
	SaveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error)
	ResetWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error)
}

type Controller struct {
	layoutStore LayoutStore
	onUpdate    func(WidgetLayout)
}

func NewController(store LayoutStore, onUpdate func(WidgetLayout)) *Controller {
	return &Controller{
		layoutStore: store,
		onUpdate:    onUpdate,
	}
}

func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/widgets/catalog", c.handleWidgetCatalog)
	mux.HandleFunc("/api/v1/widgets/layout", c.handleWidgetLayout)
	mux.HandleFunc("/api/v1/widgets/layout/reset", c.handleWidgetLayoutReset)
}

func (c *Controller) handleWidgetCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		c.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	items := widgets.List()
	catalog := make([]widgets.WidgetCatalogItem, 0, len(items))
	for _, it := range items {
		catalog = append(catalog, it.CatalogInfo())
	}
	c.writeJSON(w, http.StatusOK, map[string]any{
		"widgets": catalog,
	})
}

func (c *Controller) handleWidgetLayout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profileID := r.URL.Query().Get("profileId")
		layout, err := c.layoutStore.WidgetLayout(r.Context(), profileID)
		if err != nil {
			c.writeError(w, http.StatusInternalServerError, "widget layout is unavailable")
			return
		}
		hydrated := HydrateWidgetLayout(r.Context(), layout)
		c.writeJSON(w, http.StatusOK, hydrated)
	case http.MethodPut:
		var layout WidgetLayout
		if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
			c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		saved, err := c.layoutStore.SaveWidgetLayout(r.Context(), layout)
		if err != nil {
			if errors.Is(err, ErrInvalidLayout) {
				c.writeError(w, http.StatusBadRequest, "invalid widget layout")
				return
			}
			c.writeError(w, http.StatusInternalServerError, "widget layout could not be saved")
			return
		}
		if c.onUpdate != nil {
			c.onUpdate(saved)
		}
		c.writeJSON(w, http.StatusOK, saved)
	default:
		c.writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (c *Controller) handleWidgetLayoutReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		c.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	profileID := strings.TrimSpace(r.URL.Query().Get("profileId"))
	if profileID == "" {
		profileID = DefaultLayoutProfileID
	}

	saved, err := c.layoutStore.ResetWidgetLayout(r.Context(), profileID)
	if err != nil {
		if errors.Is(err, ErrInvalidLayout) {
			c.writeError(w, http.StatusBadRequest, "invalid widget layout")
			return
		}
		c.writeError(w, http.StatusInternalServerError, "widget layout could not be reset")
		return
	}
	if c.onUpdate != nil {
		c.onUpdate(saved)
	}
	c.writeJSON(w, http.StatusOK, saved)
}

// HydrateWidgetLayout fills in widget data and overflow properties dynamically.
func HydrateWidgetLayout(ctx context.Context, layout WidgetLayout) WidgetLayout {
	for i := range layout.Widgets {
		widget := &layout.Widgets[i]
		if !widget.Visible {
			continue
		}
		provider, ok := widgets.Get(widget.Kind)
		if !ok {
			continue
		}
		if widget.Overflow == "" {
			widget.Overflow = provider.CatalogInfo().Overflow
		}
		data, err := provider.FetchData(ctx, widget.Settings)
		if err == nil {
			widget.Data = data
		}
	}
	return layout
}

// Helpers

func (c *Controller) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (c *Controller) writeError(w http.ResponseWriter, status int, message string) {
	c.writeJSON(w, status, map[string]string{"error": message})
}

func (c *Controller) writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	c.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
