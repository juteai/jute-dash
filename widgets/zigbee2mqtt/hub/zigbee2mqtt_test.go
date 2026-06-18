package zigbee2mqtt

import (
	"context"
	"testing"

	"jute-dash/widgets"
)

func TestZigbee2MQTTWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "zigbee2mqtt" {
		t.Errorf("expected kind 'zigbee2mqtt', got %q", w.Kind())
	}

	data, err := w.FetchDataWithConnections(context.Background(), widgets.RuntimeInput{
		InstanceID: "test-instance-z2m",
		Connections: map[string]widgets.ResolvedConnection{
			"broker": {
				ID:   "z2m-test",
				Kind: "zigbee2mqtt",
				Settings: map[string]any{
					"mqtt_url":      "mock://test",
					"mqtt_username": "my_user",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("FetchData failed: %v", err)
	}
	if data.Status != widgets.StatusOK {
		t.Fatalf("expected ok payload, got %q", data.Status)
	}
	m, ok := data.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", data.Data)
	}
	devices, ok := m["devices"].([]Device)
	if !ok {
		t.Fatalf("expected []Device, got %T", m["devices"])
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].ID != "test_light" || devices[0].Type != "light" || !devices[0].State {
		t.Errorf("unexpected device data: %+v", devices[0])
	}
}
