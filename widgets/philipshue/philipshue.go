package philipshue

import (
	"context"
	"errors"
	"log/slog"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

const (
	Kind    = "philips-hue"
	SkillID = "jute.philipshue.control"
)

type SecretString string

func (s SecretString) LogValue() slog.Value {
	if s == "" {
		return slog.StringValue("")
	}
	return slog.StringValue("[redacted]")
}

type Settings struct {
	BridgeIP string
	APIKey   SecretString
}

func (s Settings) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("bridge_ip", s.BridgeIP),
		slog.Any("api_key", s.APIKey),
	)
}

type PhilipsHueWidget struct{}

func NewWidget() *PhilipsHueWidget {
	return &PhilipsHueWidget{}
}

func (w *PhilipsHueWidget) Kind() string {
	return Kind
}

func (w *PhilipsHueWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          Kind,
		Name:          "Philips Hue",
		Description:   "Control lights and rooms connected to a Philips Hue Bridge.",
		DefaultTitle:  "Philips Hue",
		DefaultW:      6,
		DefaultH:      2,
		MinW:          4,
		MinH:          2,
		DefaultSize:   "wide",
		Overflow:      "clip",
		AllowMultiple: false,
		SettingsSchema: []widgets.SettingField{
			{
				ID:    "bridge_ip",
				Type:  widgets.SettingString,
				Label: "Bridge IP",
				Help:  "IP address of the Philips Hue Bridge.",
			},
			{
				ID:    "api_key",
				Type:  widgets.SettingString,
				Label: "API Key",
				Help:  "Authorized API username key.",
			},
		},
	}
}

func (w *PhilipsHueWidget) FetchData(_ context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // use default global logger
		"fetching philips hue data",
	)
	s := parseSettings(rawSettings)
	if s.BridgeIP == "" || string(s.APIKey) == "" {
		return map[string]any{
			"is_configured": false,
		}, nil
	}
	return map[string]any{
		"is_configured": true,
		"devices":       []any{},
	}, nil
}

func (w *PhilipsHueWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          Kind,
		DisplayName:         "Philips Hue Control",
		Summary:             "Read light statuses and control devices connected to Philips Hue.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "devices", Type: "array", Description: "Discovered Hue devices list.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("status", "Get light status", "List light entities and states."),
		},
	}
}

func (w *PhilipsHueWidget) InvokeAction(
	_ context.Context,
	_ widgetskills.Snapshot,
	_ string,
	actionID string,
	_ map[string]any,
) (map[string]any, error) {
	slog.Info( //nolint:sloglint // use default global logger
		"philips hue action invoked",
		"actionID", actionID,
	)
	return nil, errors.New("live integration not implemented")
}

func parseSettings(raw map[string]any) Settings {
	s := Settings{}
	if v, ok := raw["bridge_ip"].(string); ok {
		s.BridgeIP = v
	}
	if v, ok := raw["api_key"].(string); ok {
		s.APIKey = SecretString(v)
	}
	return s
}

func init() {
	widgets.RegisterWithSkill(&PhilipsHueWidget{}, func(_ widgetskills.Snapshot, _ string) map[string]any {
		return map[string]any{
			"devices": []any{},
		}
	})
}
