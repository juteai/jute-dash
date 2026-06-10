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
		"mqtt_url":      "mqtt://localhost:1883",
		"mqtt_username": "my_user",
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
}
