package philipshue

import (
	"context"
	"testing"
)

func TestPhilipsHueWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "philips-hue" {
		t.Errorf("expected kind 'philips-hue', got %q", w.Kind())
	}

	raw := map[string]any{
		"bridge_ip": "test",
		"username":  "my_username",
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
	if devices[0].ID != "1" || devices[0].Name != "Living Room Light" || !devices[0].State {
		t.Errorf("unexpected device data: %+v", devices[0])
	}
}
