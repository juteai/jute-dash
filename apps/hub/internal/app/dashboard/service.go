package dashboard

import (
	"context"
	"strings"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
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
	Stale            bool
}

type WidgetInfo struct {
	ID            string
	Kind          string
	Title         string
	Size          string
	X, Y, W, H    int
	Permissions   []string
	PublicContext map[string]any
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

	wsWidgets := make([]widgetskills.WidgetInstance, len(layout.Widgets))
	for i, w := range layout.Widgets {
		wsWidgets[i] = widgetskills.WidgetInstance{
			ID:       w.ID,
			Kind:     w.Kind,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			Visible:  w.Visible,
			Mode:     w.Mode,
			Size:     w.Size,
			Settings: w.Settings,
			Data:     w.Data,
		}
	}
	wsLayout := widgetskills.WidgetLayout{
		ProfileID: layout.ProfileID,
		Widgets:   wsWidgets,
	}

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
			ID:    widget.ID,
			Kind:  widget.Kind,
			Title: widget.Title,
			Size:  widget.Size,
			X:     widget.X,
			Y:     widget.Y,
			W:     widget.W,
			H:     widget.H,
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
			Stale:            false,
		},
		Widgets: widgets,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
