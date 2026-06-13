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
		SecretKeys:  []string{"mqtt_password"},
	}}
}

func (w *Zigbee2MQTTWidget) FetchData(_ context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // use default global logger
		"fetching zigbee2mqtt data",
	)
	s := parseSettings(rawSettings)
	if s.MQTTURL == "" {
		return widgets.Unavailable(
			"connection.missing",
			"Zigbee2MQTT broker needed",
			"Choose a Zigbee2MQTT broker connection in settings.",
		), nil
	}

	instanceID, _ := rawSettings["instanceId"].(string)
	if instanceID == "" {
		instanceID = "default-zigbee2mqtt"
	}

	mc, err := provider.GetOrCreateClient(providerSettings(s), instanceID)
	if err != nil {
		return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
			"zigbee2mqtt.unavailable",
			"Zigbee2MQTT unavailable",
			"Jute could not reach the Zigbee2MQTT broker.",
		), nil
	}

	return map[string]any{
		"devices": mc.GetDevices(),
	}, nil
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

func (w *Zigbee2MQTTWidget) InvokeAction(
	_ context.Context,
	_ widgetskills.Snapshot,
	instanceID string,
	actionID string,
	arguments map[string]any,
) (map[string]any, error) {
	slog.Info( //nolint:sloglint // use default global logger
		"zigbee2mqtt action invoked",
		"actionID", actionID,
	)

	deviceID, _ := arguments["deviceId"].(string)
	if deviceID == "" {
		deviceID, _ = arguments["device_id"].(string)
	}
	subAction, _ := arguments["action"].(string)
	if subAction == "" {
		subAction = actionID
	}

	if deviceID == "" || subAction == "" {
		return nil, errors.New("missing deviceId or action")
	}
	if err := provider.ApplyAction(instanceID, deviceID, subAction, arguments["value"]); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
}

func (w *Zigbee2MQTTWidget) InvokeActionWithConnections(
	ctx context.Context,
	input widgets.ActionInput,
) (map[string]any, error) {
	_, _ = w.FetchDataWithConnections(ctx, input.RuntimeInput)
	args := map[string]any{
		"deviceId": input.Arguments["deviceId"],
		"action":   input.ActionID,
		"value":    input.Arguments["value"],
	}
	if args["deviceId"] == nil {
		args["deviceId"] = input.Arguments["device_id"]
	}
	return w.InvokeAction(ctx, input.Snapshot, input.InstanceID, input.ActionID, args)
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
