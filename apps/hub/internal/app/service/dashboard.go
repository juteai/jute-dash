package service

import (
	"context"
	"strings"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

type DashboardSnapshot struct {
	GeneratedAt time.Time
	Display     DisplayInfo
	Dashboard   DashboardInfo
	Widgets     []WidgetInfo
}

type DisplayInfo struct {
	DeviceID        string
	Profile         string
	Locale          string
	Timezone        string
	InteractionMode string
}

type DashboardInfo struct {
	VisibleWidgetIDs []string
	FocusedWidgetID  string
	ActiveScreenID   string
	Screens          []ScreenInfo
	Stale            bool
}

type ScreenInfo struct {
	ID               string
	Label            string
	VisibleWidgetIDs []string
	Active           bool
}

type WidgetInfo struct {
	ScreenID      string
	ID            string
	Kind          string
	Title         string
	Size          string
	X, Y, W, H    int
	Permissions   []string
	PublicContext map[string]any
}

func RegisteredCatalog() []WidgetCatalogItem {
	items := widgets.List()
	catalog := make([]WidgetCatalogItem, 0, len(items))
	for _, it := range items {
		catalog = append(catalog, it.CatalogInfo())
	}
	return catalog
}

func HydrateWidgetLayout(ctx context.Context, layout WidgetLayout) WidgetLayout {
	return NewDashboardHydrator(nil).HydrateWidgetLayout(ctx, layout)
}

type DashboardHydrator struct {
	resolver widgets.ConnectionResolver
}

func NewDashboardHydrator(resolver widgets.ConnectionResolver) *DashboardHydrator {
	return &DashboardHydrator{resolver: resolver}
}

func (h *DashboardHydrator) HydrateWidgetLayout(ctx context.Context, layout WidgetLayout) WidgetLayout {
	for i := range layout.Widgets {
		h.hydrateWidget(ctx, &layout.Widgets[i])
	}
	for screenIndex := range layout.Screens {
		for widgetIndex := range layout.Screens[screenIndex].Widgets {
			h.hydrateWidget(ctx, &layout.Screens[screenIndex].Widgets[widgetIndex])
		}
	}
	return layout
}

func (h *DashboardHydrator) hydrateWidget(ctx context.Context, widget *WidgetInstance) {
	if !widget.Visible {
		return
	}
	provider, ok := widgets.Get(widget.Kind)
	if !ok {
		return
	}
	if widget.Overflow == "" {
		widget.Overflow = provider.CatalogInfo().Overflow
	}

	settings := cloneSettings(widget.Settings)
	if connectionWidget, ok := provider.(widgets.ConnectionAwareWidget); ok {
		resolution := widgets.ConnectionResolution{Connections: map[string]widgets.ResolvedConnection{}}
		if h.resolver != nil {
			resolution = h.resolver.ResolveWidgetConnections(
				ctx,
				connectionWidget.RequiredConnections(),
				widget.ConnectionRefs,
			)
		}
		if resolution.Issue != nil {
			widget.Data = *resolution.Issue
			return
		}
		payload, err := connectionWidget.FetchDataWithConnections(ctx, widgets.RuntimeInput{
			InstanceID:     widget.ID,
			Settings:       settings,
			ConnectionRefs: cloneConnectionRefs(widget.ConnectionRefs),
			Connections:    resolution.Connections,
		})
		widget.Data = widgets.NormalizePayload(payload, err)
		return
	}
	settings["instanceId"] = widget.ID
	data, err := provider.FetchData(ctx, settings)
	widget.Data = widgets.NormalizePayload(data, err)
}

// Project compiles layout state and household locale/timezone into a dashboard snapshot.
func Project(_ context.Context, layout WidgetLayout) DashboardSnapshot {
	timezone := "UTC"
	locale := "en"
	for _, w := range layout.Widgets {
		if w.Kind == "date-time" {
			if tzVal, ok := w.Settings["timezone"].(string); ok && tzVal != "" {
				timezone = tzVal
			}
			if locVal, ok := w.Settings["locale"].(string); ok && locVal != "" {
				locale = locVal
			}
			break
		}
	}

	wsCfg := widgetskills.Config{}
	wsCfg.Home.Locale = locale
	wsCfg.Home.Timezone = timezone

	wsLayout := WidgetSkillsLayout(layout)

	snap := widgetskills.Snapshot{
		Config:      wsCfg,
		Layout:      wsLayout,
		GeneratedAt: time.Now().UTC(),
	}

	skills := widgetskills.Available(snap)
	skillsByWidget := make(map[string]widgetskills.Skill, len(skills))
	for _, skill := range skills {
		skillsByWidget[skill.WidgetInstanceID] = skill
	}

	visibleWidgetIDs := make([]string, 0, len(layout.Widgets))
	widgets := make([]WidgetInfo, 0, len(layout.Widgets))

	for _, widget := range layout.Widgets {
		if !widget.Visible {
			// Removed from the profile: no fetch, no context.
			continue
		}
		// Headless widgets feed agent context but are not on screen, so they
		// contribute a WidgetInfo but are excluded from the visible widget IDs.
		if widget.Mode != WidgetModeHeadless {
			visibleWidgetIDs = append(visibleWidgetIDs, widget.ID)
		}

		info := WidgetInfo{
			ScreenID: widget.ScreenID,
			ID:       widget.ID,
			Kind:     widget.Kind,
			Title:    widget.Title,
			Size:     widget.Size,
			X:        widget.X,
			Y:        widget.Y,
			W:        widget.W,
			H:        widget.H,
		}

		if skill, ok := skillsByWidget[widget.ID]; ok {
			info.Permissions = append([]string(nil), skill.RequiredPermissions...)
			info.PublicContext = widgetskills.ContextForSkill(snap, skill)
		} else {
			info.PublicContext = map[string]any{}
			if widget.Data != nil {
				info.PublicContext["data"] = widget.Data
			}
		}

		widgets = append(widgets, info)
	}

	return DashboardSnapshot{
		GeneratedAt: snap.GeneratedAt,
		Display: DisplayInfo{
			DeviceID:        "default-display",
			Profile:         firstNonEmpty(layout.ProfileID, "default-dashboard"),
			Locale:          locale,
			Timezone:        timezone,
			InteractionMode: "touch",
		},
		Dashboard: DashboardInfo{
			VisibleWidgetIDs: visibleWidgetIDs,
			FocusedWidgetID:  "",
			ActiveScreenID:   layout.ActiveScreen,
			Screens:          screenInfos(layout),
			Stale:            false,
		},
		Widgets: widgets,
	}
}

func WidgetSkillsLayout(layout WidgetLayout) widgetskills.WidgetLayout {
	wsWidgets := make([]widgetskills.WidgetInstance, len(layout.Widgets))
	for i, w := range layout.Widgets {
		wsWidgets[i] = widgetskills.WidgetInstance{
			ScreenID:       w.ScreenID,
			ID:             w.ID,
			Kind:           w.Kind,
			Title:          w.Title,
			X:              w.X,
			Y:              w.Y,
			W:              w.W,
			H:              w.H,
			Visible:        w.Visible,
			Mode:           w.Mode,
			Size:           w.Size,
			Settings:       w.Settings,
			ConnectionRefs: w.ConnectionRefs,
			Data:           w.Data,
		}
	}
	return widgetskills.WidgetLayout{
		ProfileID:       layout.ProfileID,
		DefaultScreenID: layout.DefaultScreen,
		ActiveScreenID:  layout.ActiveScreen,
		Screens:         widgetSkillScreens(layout),
		Widgets:         wsWidgets,
	}
}

func widgetSkillScreens(layout WidgetLayout) []widgetskills.WidgetScreen {
	screens := make([]widgetskills.WidgetScreen, 0, len(layout.Screens))
	for _, screen := range layout.Screens {
		ids := make([]string, 0, len(screen.Widgets))
		for _, widget := range screen.Widgets {
			ids = append(ids, widget.ID)
		}
		screens = append(screens, widgetskills.WidgetScreen{
			ID:      screen.ID,
			Label:   screen.Label,
			Widgets: ids,
		})
	}
	return screens
}

func screenInfos(layout WidgetLayout) []ScreenInfo {
	screens := make([]ScreenInfo, 0, len(layout.Screens))
	for _, screen := range layout.Screens {
		ids := []string{}
		for _, widget := range screen.Widgets {
			if widget.Visible && widget.Mode != WidgetModeHeadless {
				ids = append(ids, widget.ID)
			}
		}
		screens = append(screens, ScreenInfo{
			ID:               screen.ID,
			Label:            screen.Label,
			VisibleWidgetIDs: ids,
			Active:           screen.ID == layout.ActiveScreen,
		})
	}
	return screens
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
