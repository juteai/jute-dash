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
		"bridge_ip": "192.168.1.100",
		"api_key":   "my_api_key",
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
