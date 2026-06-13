package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Settings struct {
	MQTTURL      string
	MQTTUsername string
	MQTTPassword string
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

type Client struct {
	client  mqtt.Client
	devices []RawDevice
	states  map[string]map[string]any
	mu      sync.RWMutex
}

var (
	clientsMu sync.Mutex
	clients   = make(map[string]*Client)
)

func GetOrCreateClient(settings Settings, instanceID string) (*Client, error) {
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
		mc := &Client{
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
	if settings.MQTTPassword != "" {
		opts.SetPassword(settings.MQTTPassword)
	}
	opts.SetClientID(fmt.Sprintf("jute-dash-%s", instanceID))
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)

	mc := &Client{
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

func ApplyAction(instanceID string, deviceID string, actionID string, value any) error {
	if deviceID == "" {
		return errors.New("missing deviceId")
	}

	clientsMu.Lock()
	mc, ok := clients[instanceID]
	clientsMu.Unlock()
	if !ok || mc == nil {
		return errors.New("MQTT broker not connected")
	}
	if mc.client != nil && !mc.client.IsConnected() {
		return errors.New("MQTT broker not connected")
	}

	payload := make(map[string]any)
	switch actionID {
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
		if b, ok := value.(float64); ok {
			payload["brightness"] = int(b)
		} else if b, ok := value.(int); ok {
			payload["brightness"] = b
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if mc.client != nil {
		topic := fmt.Sprintf("zigbee2mqtt/%s/set", deviceID)
		token := mc.client.Publish(topic, 0, false, body)
		if token.Wait() && token.Error() != nil {
			return token.Error()
		}
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

	return nil
}

func (mc *Client) GetDevices() []Device {
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
