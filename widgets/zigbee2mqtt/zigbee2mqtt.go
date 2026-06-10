package zigbee2mqtt

import (
	"context"
	"errors"
	"log/slog"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

const (
	Kind    = "zigbee2mqtt"
	SkillID = "jute.zigbee2mqtt.control"
)

type SecretString string

func (s SecretString) LogValue() slog.Value {
	if s == "" {
		return slog.StringValue("")
	}
	return slog.StringValue("[redacted]")
}

type Settings struct {
	MQTTURL      string
	MQTTUsername string
	MQTTPassword SecretString
}

func (s Settings) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("mqtt_url", s.MQTTURL),
		slog.String("mqtt_username", s.MQTTUsername),
		slog.Any("mqtt_password", s.MQTTPassword),
	)
}

type Zigbee2MQTTWidget struct{}

func NewWidget() *Zigbee2MQTTWidget {
	return &Zigbee2MQTTWidget{}
}

func (w *Zigbee2MQTTWidget) Kind() string {
	return Kind
}

func (w *Zigbee2MQTTWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          Kind,
		Name:          "Zigbee2MQTT",
		Description:   "Monitor and control local smart devices connected via Zigbee2MQTT.",
		DefaultTitle:  "Zigbee",
		DefaultW:      6,
		DefaultH:      2,
		MinW:          4,
		MinH:          2,
		DefaultSize:   "wide",
		Overflow:      "clip",
		AllowMultiple: false,
		SettingsSchema: []widgets.SettingField{
			{
				ID:      "mqtt_url",
				Type:    widgets.SettingString,
				Label:   "MQTT URL",
				Default: "mqtt://localhost:1883",
				Help:    "Address of the MQTT Broker.",
			},
			{
				ID:    "mqtt_username",
				Type:  widgets.SettingString,
				Label: "MQTT Username",
				Help:  "Broker username credentials.",
			},
			{
				ID:    "mqtt_password",
				Type:  widgets.SettingString,
				Label: "MQTT Password",
				Help:  "Broker password credentials.",
			},
		},
	}
}

func (w *Zigbee2MQTTWidget) FetchData(_ context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // use default global logger
		"fetching zigbee2mqtt data",
	)
	s := parseSettings(rawSettings)
	if s.MQTTURL == "" {
		return map[string]any{
			"is_configured": false,
		}, nil
	}
	return map[string]any{
		"is_configured": true,
		"devices":       []any{},
	}, nil
}

func (w *Zigbee2MQTTWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          Kind,
		DisplayName:         "Zigbee2MQTT Control",
		Summary:             "Control and read status of local Zigbee devices.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "devices", Type: "array", Description: "Connected Zigbee devices list.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("status", "Get device status", "List Zigbee devices and sensor outputs."),
		},
	}
}

func (w *Zigbee2MQTTWidget) InvokeAction(
	_ context.Context,
	_ widgetskills.Snapshot,
	_ string,
	actionID string,
	_ map[string]any,
) (map[string]any, error) {
	slog.Info( //nolint:sloglint // use default global logger
		"zigbee2mqtt action invoked",
		"actionID", actionID,
	)
	return nil, errors.New("live integration not implemented")
}

func parseSettings(raw map[string]any) Settings {
	s := Settings{
		MQTTURL: "mqtt://localhost:1883",
	}
	if v, ok := raw["mqtt_url"].(string); ok && v != "" {
		s.MQTTURL = v
	}
	if v, ok := raw["mqtt_username"].(string); ok {
		s.MQTTUsername = v
	}
	if v, ok := raw["mqtt_password"].(string); ok {
		s.MQTTPassword = SecretString(v)
	}
	return s
}

func init() {
	widgets.RegisterWithSkill(&Zigbee2MQTTWidget{}, func(_ widgetskills.Snapshot, _ string) map[string]any {
		return map[string]any{
			"devices": []any{},
		}
	})
}
