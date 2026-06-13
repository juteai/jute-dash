package zigbee2mqtt

import (
	"context"
	"errors"
	"log/slog"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"jute-dash/widgets/zigbee2mqtt/hub/internal/provider"
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

type Device = provider.Device

type Zigbee2MQTTWidget struct{}

func NewWidget() *Zigbee2MQTTWidget {
	return &Zigbee2MQTTWidget{}
}

func (w *Zigbee2MQTTWidget) Kind() string {
	return Kind
}

func (w *Zigbee2MQTTWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:                   Kind,
		Name:                   "Zigbee2MQTT",
		Description:            "Monitor and control local smart devices connected via Zigbee2MQTT.",
		DefaultTitle:           "Zigbee",
		DefaultW:               6,
		DefaultH:               2,
		MinW:                   4,
		MinH:                   2,
		DefaultSize:            "wide",
		Overflow:               "clip",
		AllowMultiple:          false,
		ConnectionRequirements: w.RequiredConnections(),
	}
}

func (w *Zigbee2MQTTWidget) RequiredConnections() []widgets.ConnectionRequirement {
	return []widgets.ConnectionRequirement{{
		Slot:        "broker",
		Kind:        "zigbee2mqtt",
		DisplayName: "Zigbee2MQTT Broker",
		Description: "MQTT broker used by Zigbee2MQTT.",
		Required:    true,
		Fields: []widgets.ConnectionField{
			{
				ID:       "mqtt_url",
				Type:     widgets.ConnectionFieldString,
				Label:    "MQTT broker URL",
				Required: true,
				Default:  "mqtt://localhost:1883",
			},
			{
				ID:    "mqtt_username",
				Type:  widgets.ConnectionFieldString,
				Label: "MQTT username",
			},
			{
				ID:     "mqtt_password",
				Type:   widgets.ConnectionFieldString,
				Label:  "MQTT password reference",
				Secret: true,
				Help:   "Optional secret reference such as env:MQTT_PASSWORD.",
			},
		},
	}}
}

func (w *Zigbee2MQTTWidget) FetchData(_ context.Context, _ map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // use default global logger
		"fetching zigbee2mqtt data",
	)
	return widgets.Unavailable(
		"connection.missing",
		"Zigbee2MQTT broker needed",
		"Choose a Zigbee2MQTT broker connection in settings.",
	), nil
}

func (w *Zigbee2MQTTWidget) FetchDataWithConnections(
	_ context.Context,
	input widgets.RuntimeInput,
) (widgets.RuntimePayload, error) {
	settings := zigbeeSettingsFromConnection(input.Connections["broker"])
	if settings.MQTTURL == "" {
		return widgets.Unavailable(
			"connection.missing",
			"MQTT broker needed",
			"Choose a Zigbee2MQTT Broker connection in settings.",
		), nil
	}
	mc, err := provider.GetOrCreateClient(providerSettings(settings), input.InstanceID)
	if err != nil {
		return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
			"zigbee2mqtt.broker_unavailable",
			"MQTT broker unavailable",
			"Jute cannot reach the Zigbee2MQTT broker.",
		), nil
	}
	return widgets.OK(map[string]any{"devices": mc.GetDevices()}), nil
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
			zigbeeAction("toggle", "Toggle device", "Toggle a Zigbee switch or light."),
			zigbeeAction("turn_on", "Turn device on", "Turn a Zigbee switch or light on."),
			zigbeeAction("turn_off", "Turn device off", "Turn a Zigbee switch or light off."),
			zigbeeAction("set_brightness", "Set brightness", "Set Zigbee light brightness."),
		},
	}
}

func zigbeeAction(id, title, description string) widgetskills.Action {
	return widgetskills.Action{
		ID:                   id,
		Title:                title,
		Description:          description,
		SideEffect:           "home_action",
		RequiresConfirmation: false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deviceId": map[string]any{"type": "string"},
				"value":    map[string]any{},
			},
			"required":             []string{"deviceId"},
			"additionalProperties": true,
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
	}
}

func (w *Zigbee2MQTTWidget) InvokeActionWithConnections(
	ctx context.Context,
	input widgets.ActionInput,
) (map[string]any, error) {
	_, _ = w.FetchDataWithConnections(ctx, input.RuntimeInput)
	slog.Info( //nolint:sloglint // use default global logger
		"zigbee2mqtt action invoked",
		"actionID", input.ActionID,
	)
	deviceID, _ := input.Arguments["deviceId"].(string)
	if deviceID == "" {
		deviceID, _ = input.Arguments["device_id"].(string)
	}
	if deviceID == "" {
		return nil, errors.New("missing deviceId")
	}
	if err := provider.ApplyAction(input.InstanceID, deviceID, input.ActionID, input.Arguments["value"]); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
}

func zigbeeSettingsFromConnection(connection widgets.ResolvedConnection) Settings {
	settings := Settings{MQTTURL: "mqtt://localhost:1883"}
	if v, ok := connection.Settings["mqtt_url"].(string); ok && v != "" {
		settings.MQTTURL = v
	}
	if v, ok := connection.Settings["mqtt_username"].(string); ok {
		settings.MQTTUsername = v
	}
	settings.MQTTPassword = SecretString(connection.Secrets["mqtt_password"])
	return settings
}

func providerSettings(settings Settings) provider.Settings {
	return provider.Settings{
		MQTTURL:      settings.MQTTURL,
		MQTTUsername: settings.MQTTUsername,
		MQTTPassword: string(settings.MQTTPassword),
	}
}

func init() {
	widgets.RegisterWithSkill(&Zigbee2MQTTWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
		for _, widget := range snapshot.Layout.Widgets {
			if widget.ID == instID {
				if data, ok := widgets.PayloadData(widget.Data).(map[string]any); ok {
					return data
				}
			}
		}
		return map[string]any{"devices": []any{}}
	})
}
