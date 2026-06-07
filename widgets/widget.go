package widgets

import (
	"context"
	"jute-dash/apps/hub/pkg/widgetskills"
)

type WidgetCatalogItem struct {
	Kind           string         `json:"kind"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	DefaultTitle   string         `json:"defaultTitle"`
	DefaultW       int            `json:"defaultW"`
	DefaultH       int            `json:"defaultH"`
	MinW           int            `json:"minW"`
	MinH           int            `json:"minH"`
	DefaultSize    string         `json:"defaultSize"`
	Overflow       string         `json:"overflow"`
	AllowMultiple  bool           `json:"allowMultiple"`
	SettingsSchema []SettingField `json:"settingsSchema,omitempty"`
}

// SettingFieldType enumerates the settings field types the display can render.
type SettingFieldType string

const (
	SettingString     SettingFieldType = "string"
	SettingNumber     SettingFieldType = "number"
	SettingBoolean    SettingFieldType = "boolean"
	SettingEnum       SettingFieldType = "enum"
	SettingStringList SettingFieldType = "string-list"
	SettingObjectList SettingFieldType = "object-list"
)

// SettingField is one typed, UI-renderable widget setting. A single generic
// form renderer in the display builds the settings sheet from these.
type SettingField struct {
	ID      string           `json:"id"`
	Type    SettingFieldType `json:"type"`
	Label   string           `json:"label"`
	Help    string           `json:"help,omitempty"`
	Default any              `json:"default,omitempty"`
	Options []string         `json:"options,omitempty"` // for enum
	Fields  []SettingField   `json:"fields,omitempty"`  // for object-list item shape
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
