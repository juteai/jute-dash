package datetime

import (
	"context"
	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

type DateTimeWidget struct{}

func (w *DateTimeWidget) Kind() string {
	return "date-time"
}

func (w *DateTimeWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          "date-time",
		Name:          "Date & Time",
		Description:   "Clock, date, timezone, and local display timing.",
		DefaultTitle:  "Clock",
		DefaultW:      2,
		DefaultH:      1,
		MinW:          1,
		MinH:          1,
		DefaultSize:   "wide",
		Overflow:      "clip",
		AllowMultiple: false,
	}
}

func (w *DateTimeWidget) FetchData(ctx context.Context, settings map[string]any) (any, error) {
	return map[string]any{}, nil
}

func (w *DateTimeWidget) Skill() *widgetskills.Definition {
	return nil
}

func init() {
	widgets.Register(&DateTimeWidget{})
}
