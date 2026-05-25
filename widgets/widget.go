package widgets

import (
	"context"
	"jute-dash/internal/widgetskills"
)

type WidgetCatalogItem struct {
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	DefaultTitle  string   `json:"defaultTitle"`
	DefaultW      int      `json:"defaultW"`
	DefaultH      int      `json:"defaultH"`
	MinW          int      `json:"minW"`
	MinH          int      `json:"minH"`
	DefaultSize   string   `json:"defaultSize"`
	Overflow      string   `json:"overflow"`
	AllowMultiple bool     `json:"allowMultiple"`
}

type Widget interface {
	// Kind returns the unique string identifier for the widget (e.g. "weather", "rss").
	Kind() string

	// CatalogInfo returns the static registration metadata.
	CatalogInfo() WidgetCatalogItem

	// FetchData gathers and aggregates the latest state/payload for this widget.
	// It is passed the widget's custom settings from the YAML file.
	FetchData(ctx context.Context, settings map[string]any) (any, error)

	// Skill returns the optional agent-facing skill metadata. Returns nil if visual-only.
	Skill() *widgetskills.Definition
}
