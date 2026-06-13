package philipshue

import (
	"context"
	"testing"

	"jute-dash/widgets"
)

func TestPhilipsHueWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "philips-hue" {
		t.Errorf("expected kind 'philips-hue', got %q", w.Kind())
	}

	data, err := w.FetchDataWithConnections(context.Background(), widgets.RuntimeInput{
		InstanceID: "hue-1",
		Connections: map[string]widgets.ResolvedConnection{
			"bridge": {
				ID:   "hue-test",
				Kind: "philips-hue",
				Settings: map[string]any{
					"bridge_ip": "test",
				},
				Secrets: map[string]string{
					"username": "my_username",
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
		t.Fatalf("expected map[string]any data, got %T", data.Data)
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
