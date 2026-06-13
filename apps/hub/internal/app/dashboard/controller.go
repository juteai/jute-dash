package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"jute-dash/apps/hub/internal/pkg/httphelper"
	"jute-dash/widgets"
)

type LayoutStore interface {
	WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error)
	SaveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error)
	ResetWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error)
}

type Controller struct {
	layoutStore LayoutStore
	hydrator    *Hydrator
	onUpdate    func(WidgetLayout)
}

func NewController(store LayoutStore, onUpdate func(WidgetLayout)) *Controller {
	return NewControllerWithHydrator(store, NewHydrator(nil), onUpdate)
}

func NewControllerWithHydrator(
	store LayoutStore,
	hydrator *Hydrator,
	onUpdate func(WidgetLayout),
) *Controller {
	if hydrator == nil {
		hydrator = NewHydrator(nil)
	}
	return &Controller{
		layoutStore: store,
		hydrator:    hydrator,
		onUpdate:    onUpdate,
	}
}

func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/widgets/catalog", c.handleWidgetCatalog)
	mux.HandleFunc("/api/v1/widgets/layout", c.handleWidgetLayout)
	mux.HandleFunc("/api/v1/widgets/layout/reset", c.handleWidgetLayoutReset)
}

// RegisteredCatalog returns catalog metadata for every registered widget,
// as the single source the layout store uses for normalization.
func RegisteredCatalog() []WidgetCatalogItem {
	items := widgets.List()
	catalog := make([]WidgetCatalogItem, 0, len(items))
	for _, it := range items {
		catalog = append(catalog, it.CatalogInfo())
	}
	return catalog
}

func (c *Controller) handleWidgetCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httphelper.WriteMethodNotAllowed(w, http.MethodGet)
		return
	}
	items := widgets.List()
	catalog := make([]widgets.WidgetCatalogItem, 0, len(items))
	for _, it := range items {
		catalog = append(catalog, it.CatalogInfo())
	}
	httphelper.WriteJSON(w, http.StatusOK, map[string]any{
		"widgets": catalog,
	})
}

func (c *Controller) handleWidgetLayout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profileID := r.URL.Query().Get("profileId")
		layout, err := c.layoutStore.WidgetLayout(r.Context(), profileID)
		if err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, "widget layout is unavailable")
			return
		}
		hydrated := c.hydrator.HydrateWidgetLayout(r.Context(), layout)
		httphelper.WriteJSON(w, http.StatusOK, hydrated)
	case http.MethodPut:
		var layout WidgetLayout
		if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		saved, err := c.layoutStore.SaveWidgetLayout(r.Context(), layout)
		if err != nil {
			if errors.Is(err, ErrInvalidLayout) {
				httphelper.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			httphelper.WriteError(w, http.StatusInternalServerError, "widget layout could not be saved")
			return
		}
		if c.onUpdate != nil {
			c.onUpdate(saved)
		}
		httphelper.WriteJSON(w, http.StatusOK, saved)
	default:
		httphelper.WriteMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (c *Controller) handleWidgetLayoutReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httphelper.WriteMethodNotAllowed(w, http.MethodPost)
		return
	}
	profileID := strings.TrimSpace(r.URL.Query().Get("profileId"))
	if profileID == "" {
		profileID = DefaultLayoutProfileID
	}

	saved, err := c.layoutStore.ResetWidgetLayout(r.Context(), profileID)
	if err != nil {
		if errors.Is(err, ErrInvalidLayout) {
			httphelper.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httphelper.WriteError(w, http.StatusInternalServerError, "widget layout could not be reset")
		return
	}
	if c.onUpdate != nil {
		c.onUpdate(saved)
	}
	httphelper.WriteJSON(w, http.StatusOK, saved)
}

// HydrateWidgetLayout fills in widget data and overflow properties dynamically.
func HydrateWidgetLayout(ctx context.Context, layout WidgetLayout) WidgetLayout {
	return NewHydrator(nil).HydrateWidgetLayout(ctx, layout)
}

type Hydrator struct {
	resolver widgets.ConnectionResolver
}

func NewHydrator(resolver widgets.ConnectionResolver) *Hydrator {
	return &Hydrator{resolver: resolver}
}

func (h *Hydrator) HydrateWidgetLayout(ctx context.Context, layout WidgetLayout) WidgetLayout {
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

		settings := cloneSettings(widget.Settings)
		if connectionWidget, ok := provider.(widgets.ConnectionAwareWidget); ok {
			resolution := widgets.ConnectionResolution{
				Connections: map[string]widgets.ResolvedConnection{},
			}
			if h.resolver != nil {
				resolution = h.resolver.ResolveWidgetConnections(
					ctx,
					connectionWidget.RequiredConnections(),
					widget.ConnectionRefs,
				)
			}
			if resolution.Issue != nil {
				widget.Data = *resolution.Issue
				continue
			}
			payload, err := connectionWidget.FetchDataWithConnections(ctx, widgets.RuntimeInput{
				InstanceID:     widget.ID,
				Settings:       settings,
				ConnectionRefs: cloneConnectionRefs(widget.ConnectionRefs),
				Connections:    resolution.Connections,
			})
			widget.Data = widgets.NormalizePayload(payload, err)
			continue
		}
		settings["instanceId"] = widget.ID
		data, err := provider.FetchData(ctx, settings)
		widget.Data = widgets.NormalizePayload(data, err)
	}
	return layout
}

func cloneSettings(in map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneConnectionRefs(in map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		out[k] = v
	}
	return out
}
