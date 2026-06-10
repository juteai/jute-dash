package spotify

import (
	"context"
	"testing"
)

func TestSpotifyWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "spotify" {
		t.Errorf("expected kind 'spotify', got %q", w.Kind())
	}

	raw := map[string]any{
		"client_id":     "my_client_id",
		"client_secret": "my_client_secret",
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
