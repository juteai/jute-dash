package zigbee2mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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

type RawDevice struct {
	FriendlyName string `json:"friendly_name"`
	IEEEAddress  string `json:"ieee_address"`
	Type         string `json:"type"`
	Definition   *struct {
		Description string `json:"description"`
		Model       string `json:"model"`
		Exposes     []struct {
			Type     string `json:"type"`
			Features []struct {
				Name     string `json:"name"`
				Property string `json:"property"`
			} `json:"features"`
		} `json:"exposes"`
	} `json:"definition"`
}

type Device struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"` // "light", "switch", "sensor"
	State bool   `json:"state"`
	Value string `json:"value"`
}

type mqttClient struct {
	client  mqtt.Client
	devices []RawDevice
	states  map[string]map[string]any
	mu      sync.RWMutex
}

var (
	clientsMu sync.Mutex
	clients   = make(map[string]*mqttClient)
)

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

	mc, err := getOrCreateClient(s, instanceID)
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
	mc, err := getOrCreateClient(settings, input.InstanceID)
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
	subAction, _ := arguments["action"].(string)
	val := arguments["value"]

	if deviceID == "" || subAction == "" {
		return nil, errors.New("missing deviceId or action")
	}

	clientsMu.Lock()
	mc, ok := clients[instanceID]
	clientsMu.Unlock()
	if !ok || mc == nil || !mc.client.IsConnected() {
		return nil, errors.New("MQTT broker not connected")
	}

	payload := make(map[string]any)
	switch subAction {
	case "turn_on":
		payload["state"] = "ON"
	case "turn_off":
		payload["state"] = "OFF"
	case "toggle":
		mc.mu.RLock()
		currentState := "OFF"
		if st, ok := mc.states[deviceID]; ok {
			if s, ok := st["state"].(string); ok {
				currentState = s
			}
		}
		mc.mu.RUnlock()
		if currentState == "ON" {
			payload["state"] = "OFF"
		} else {
			payload["state"] = "ON"
		}
	case "brightness", "set_brightness":
		if b, ok := val.(float64); ok {
			payload["brightness"] = int(b)
		} else if b, ok := val.(int); ok {
			payload["brightness"] = b
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	topic := fmt.Sprintf("zigbee2mqtt/%s/set", deviceID)
	token := mc.client.Publish(topic, 0, false, body)
	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	mc.mu.Lock()
	if mc.states == nil {
		mc.states = make(map[string]map[string]any)
	}
	if mc.states[deviceID] == nil {
		mc.states[deviceID] = make(map[string]any)
	}
	if stateStr, ok := payload["state"].(string); ok {
		mc.states[deviceID]["state"] = stateStr
	}
	if briVal, ok := payload["brightness"].(int); ok {
		mc.states[deviceID]["brightness"] = briVal
	}
	mc.mu.Unlock()

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

func getOrCreateClient(settings Settings, instanceID string) (*mqttClient, error) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	key := instanceID
	if c, ok := clients[key]; ok {
		if c.client != nil && c.client.IsConnected() {
			return c, nil
		}
		if c.client != nil {
			c.client.Disconnect(250)
		}
		delete(clients, key)
	}

	if strings.HasPrefix(settings.MQTTURL, "mock://") || settings.MQTTURL == "test" {
		mc := &mqttClient{
			devices: []RawDevice{
				{
					FriendlyName: "test_light",
					IEEEAddress:  "0x12345",
					Definition: &struct {
						Description string `json:"description"`
						Model       string `json:"model"`
						Exposes     []struct {
							Type     string `json:"type"`
							Features []struct {
								Name     string `json:"name"`
								Property string `json:"property"`
							} `json:"features"`
						} `json:"exposes"`
					}{
						Exposes: []struct {
							Type     string `json:"type"`
							Features []struct {
								Name     string `json:"name"`
								Property string `json:"property"`
							} `json:"features"`
						}{
							{Type: "light"},
						},
					},
				},
			},
			states: map[string]map[string]any{
				"test_light": {"state": "ON"},
			},
		}
		clients[key] = mc
		return mc, nil
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(settings.MQTTURL)
	if settings.MQTTUsername != "" {
		opts.SetUsername(settings.MQTTUsername)
	}
	if string(settings.MQTTPassword) != "" {
		opts.SetPassword(string(settings.MQTTPassword))
	}
	opts.SetClientID(fmt.Sprintf("jute-dash-%s", instanceID))
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)

	mc := &mqttClient{
		devices: []RawDevice{},
		states:  make(map[string]map[string]any),
	}

	opts.OnConnect = func(client mqtt.Client) {
		slog.Info("MQTT connected", "instanceID", instanceID) //nolint:sloglint // use default global logger
		client.Subscribe("zigbee2mqtt/bridge/devices", 0, func(_ mqtt.Client, msg mqtt.Message) {
			var devList []RawDevice
			if err := json.Unmarshal(msg.Payload(), &devList); err == nil {
				mc.mu.Lock()
				mc.devices = devList
				mc.mu.Unlock()
			}
		})
		client.Subscribe("zigbee2mqtt/+", 0, func(_ mqtt.Client, msg mqtt.Message) {
			topic := msg.Topic()
			parts := strings.Split(topic, "/")
			if len(parts) < 2 || parts[1] == "bridge" {
				return
			}
			friendlyName := parts[1]
			var payload map[string]any
			if err := json.Unmarshal(msg.Payload(), &payload); err == nil {
				mc.mu.Lock()
				if mc.states == nil {
					mc.states = make(map[string]map[string]any)
				}
				mc.states[friendlyName] = payload
				mc.mu.Unlock()
			}
		})
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	mc.client = client
	clients[key] = mc
	return mc, nil
}

func (mc *mqttClient) GetDevices() []Device {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := []Device{}
	for _, rd := range mc.devices {
		if rd.FriendlyName == "Bridge" || rd.FriendlyName == "" {
			continue
		}
		devType := "sensor"
		if rd.Definition != nil {
			for _, exp := range rd.Definition.Exposes {
				if exp.Type == "light" {
					devType = "light"
					break
				}
				if exp.Type == "switch" {
					devType = "switch"
					break
				}
			}
		}

		stateVal := false
		sensorVal := ""
		if st, ok := mc.states[rd.FriendlyName]; ok {
			if s, ok := st["state"].(string); ok {
				stateVal = (s == "ON")
			}
			if val, ok := st["temperature"]; ok {
				sensorVal = fmt.Sprintf("%v°C", val)
			} else if val, ok := st["humidity"]; ok {
				sensorVal = fmt.Sprintf("%v%%", val)
			} else if val, ok := st["battery"]; ok {
				sensorVal = fmt.Sprintf("Battery: %v%%", val)
			} else if val, ok := st["contact"]; ok {
				if b, ok := val.(bool); ok {
					if b {
						sensorVal = "Closed"
					} else {
						sensorVal = "Open"
					}
				}
			}
		}

		result = append(result, Device{
			ID:    rd.FriendlyName,
			Name:  rd.FriendlyName,
			Type:  devType,
			State: stateVal,
			Value: sensorVal,
		})
	}
	return result
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
