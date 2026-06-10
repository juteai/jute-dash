package zigbee2mqtt

import (
	"context"
	"testing"
)

func TestZigbee2MQTTWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "zigbee2mqtt" {
		t.Errorf("expected kind 'zigbee2mqtt', got %q", w.Kind())
	}

	raw := map[string]any{
		"mqtt_url":      "mock://test",
		"mqtt_username": "my_user",
		"instanceId":    "test-instance-z2m",
	}
	data, err := w.FetchData(context.Background(), raw)
	if err != nil {
		t.Fatalf("FetchData failed: %v", err)
	}
	m, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", data)
	}
	if m["is_configured"] != true {
		t.Errorf("expected is_configured to be true")
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
