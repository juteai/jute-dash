package widgets

import (
	"context"
	"jute-dash/apps/hub/pkg/widgetskills"
	"time"
)

type WidgetCatalogItem struct {
	Kind                   string                  `json:"kind"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	DefaultTitle           string                  `json:"defaultTitle"`
	DefaultW               int                     `json:"defaultW"`
	DefaultH               int                     `json:"defaultH"`
	MinW                   int                     `json:"minW"`
	MinH                   int                     `json:"minH"`
	DefaultSize            string                  `json:"defaultSize"`
	Overflow               string                  `json:"overflow"`
	AllowMultiple          bool                    `json:"allowMultiple"`
	SettingsSchema         []SettingField          `json:"settingsSchema,omitempty"`
	ConnectionRequirements []ConnectionRequirement `json:"connectionRequirements,omitempty"`
}

type ConnectionFieldType string

const (
	ConnectionFieldString  ConnectionFieldType = "string"
	ConnectionFieldNumber  ConnectionFieldType = "number"
	ConnectionFieldBoolean ConnectionFieldType = "boolean"
	ConnectionFieldEnum    ConnectionFieldType = "enum"
)

type ConnectionField struct {
	ID       string              `json:"id"`
	Type     ConnectionFieldType `json:"type"`
	Label    string              `json:"label"`
	Help     string              `json:"help,omitempty"`
	Required bool                `json:"required"`
	Secret   bool                `json:"secret"`
	Default  any                 `json:"default,omitempty"`
	Options  []string            `json:"options,omitempty"`
}

type AdapterConnectionKind struct {
	Kind        string            `json:"kind"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description,omitempty"`
	Fields      []ConnectionField `json:"fields"`
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

type UserFacingIssue struct {
	Code     string       `json:"code"`
	Severity string       `json:"severity"`
	Title    string       `json:"title"`
	Message  string       `json:"message"`
	Action   *IssueAction `json:"action,omitempty"`
}

type IssueAction struct {
	Label  string `json:"label"`
	Target string `json:"target"`
}

type RuntimePayload struct {
	Status    string           `json:"status"`
	Issue     *UserFacingIssue `json:"issue,omitempty"`
	UpdatedAt string           `json:"updatedAt,omitempty"`
	Data      any              `json:"data,omitempty"`
}

const (
	StatusOK                 = "ok"
	StatusLoading            = "loading"
	StatusEmpty              = "empty"
	StatusUnavailable        = "unavailable"
	StatusError              = "error"
	StatusPermissionRequired = "permission_required"
	StatusStale              = "stale"
)

func OK(data any) RuntimePayload {
	return RuntimePayload{
		Status:    StatusOK,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Data:      data,
	}
}

func Empty(title, message string) RuntimePayload {
	return RuntimePayload{
		Status: StatusEmpty,
		Issue: &UserFacingIssue{
			Code:     "widget.empty",
			Severity: "info",
			Title:    title,
			Message:  message,
		},
	}
}

func Unavailable(code, title, message string) RuntimePayload {
	return RuntimePayload{
		Status: StatusUnavailable,
		Issue: &UserFacingIssue{
			Code:     code,
			Severity: "warning",
			Title:    title,
			Message:  message,
			Action:   &IssueAction{Label: "Open settings", Target: "settings"},
		},
	}
}

func ErrorPayload(code, title, message string) RuntimePayload {
	return RuntimePayload{
		Status: StatusError,
		Issue: &UserFacingIssue{
			Code:     code,
			Severity: "error",
			Title:    title,
			Message:  message,
		},
	}
}

func NormalizePayload(data any, err error) RuntimePayload {
	if err != nil {
		return ErrorPayload("widget.fetch_failed", "Widget unavailable", "Jute could not load this widget.")
	}
	if payload, ok := data.(RuntimePayload); ok {
		return payload
	}
	if payload, ok := data.(*RuntimePayload); ok && payload != nil {
		return *payload
	}
	return OK(data)
}

func PayloadData(data any) any {
	switch payload := data.(type) {
	case RuntimePayload:
		return payload.Data
	case *RuntimePayload:
		if payload == nil {
			return nil
		}
		return payload.Data
	case map[string]any:
		if _, hasStatus := payload["status"]; hasStatus {
			if inner, ok := payload["data"]; ok {
				return inner
			}
		}
	}
	return data
}

type ConnectionRequirement struct {
	Slot        string            `json:"slot"`
	Kind        string            `json:"kind"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description,omitempty"`
	Required    bool              `json:"required"`
	SecretKeys  []string          `json:"secretKeys,omitempty"`
	Fields      []ConnectionField `json:"fields,omitempty"`
}

type ResolvedConnection struct {
	ID       string
	Kind     string
	Name     string
	Settings map[string]any
	Secrets  map[string]string
	Enabled  bool
}

type ConnectionResolver interface {
	ResolveWidgetConnections(
		ctx context.Context,
		requirements []ConnectionRequirement,
		refs map[string]string,
	) ConnectionResolution
}

type ConnectionResolution struct {
	Connections map[string]ResolvedConnection
	Issue       *RuntimePayload
}

type RuntimeInput struct {
	InstanceID     string
	Settings       map[string]any
	ConnectionRefs map[string]string
	Connections    map[string]ResolvedConnection
}

type ActionInput struct {
	RuntimeInput

	Snapshot  widgetskills.Snapshot
	ActionID  string
	Arguments map[string]any
	Actor     string
}

type ConnectionAwareWidget interface {
	Widget
	RequiredConnections() []ConnectionRequirement
	FetchDataWithConnections(ctx context.Context, input RuntimeInput) (RuntimePayload, error)
}

type ConnectionAwareActionWidget interface {
	ConnectionAwareWidget
	InvokeActionWithConnections(ctx context.Context, input ActionInput) (map[string]any, error)
}
