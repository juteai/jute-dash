package spotify

import (
	"context"
	"testing"

	"jute-dash/apps/hub/pkg/widgetskills"
)

func TestSpotifyWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "spotify" {
		t.Errorf("expected kind 'spotify', got %q", w.Kind())
	}

	// 1. Completely unconfigured
	rawEmpty := map[string]any{}
	data, err := w.FetchData(context.Background(), rawEmpty)
	if err != nil {
		t.Fatalf("FetchData empty failed: %v", err)
	}
	m, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", data)
	}
	if m["is_configured"] != false {
		t.Errorf("expected is_configured to be false for empty settings")
	}

	// 2. Partially configured (no access token)
	rawPartial := map[string]any{
		"client_id":     "my_client_id",
		"client_secret": "my_client_secret",
	}
	data, err = w.FetchData(context.Background(), rawPartial)
	if err != nil {
		t.Fatalf("FetchData partial failed: %v", err)
	}
	m, ok = data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", data)
	}
	if m["is_configured"] != false {
		t.Errorf("expected is_configured to be false without access token")
	}

	// 3. Mock/Test mode (configured with mock)
	rawMock := map[string]any{
		"client_id":     "test",
		"client_secret": "my_client_secret",
		"access_token":  "my_access_token",
	}
	data, err = w.FetchData(context.Background(), rawMock)
	if err != nil {
		t.Fatalf("FetchData mock failed: %v", err)
	}
	m, ok = data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", data)
	}
	if m["is_configured"] != true {
		t.Errorf("expected is_configured to be true in mock mode")
	}
	if m["track_title"] != "Mock Track" {
		t.Errorf("expected track title 'Mock Track', got %q", m["track_title"])
	}
}

func TestSpotifyWidgetActions(t *testing.T) {
	w := NewWidget()

	// Setup snapshot with a configured widget instance
	snap := widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					ID:   "spotify-1",
					Kind: Kind,
					Settings: map[string]any{
						"client_id":     "test",
						"client_secret": "secret",
						"access_token":  "token",
					},
				},
			},
		},
	}

	// Invoke actions in mock mode
	actions := []string{"play", "pause", "next", "previous"}
	for _, act := range actions {
		res, err := w.InvokeAction(context.Background(), snap, "spotify-1", act, nil)
		if err != nil {
			t.Errorf("action %q failed: %v", act, err)
		}
		if res["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", res["status"])
		}
	}

	// Set volume
	res, err := w.InvokeAction(context.Background(), snap, "spotify-1", "set_volume", map[string]any{
		"volume": 80,
	})
	if err != nil {
		t.Errorf("set_volume failed: %v", err)
	}
	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}
}

func TestSpotifyLogValueRedaction(t *testing.T) {
	secret := SecretString("my_super_secret")
	logVal := secret.LogValue()
	if logVal.String() != "[redacted]" {
		t.Errorf("expected secret string to be redacted in logs, got %q", logVal.String())
	}

	emptySecret := SecretString("")
	emptyLogVal := emptySecret.LogValue()
	if emptyLogVal.String() != "" {
		t.Errorf("expected empty secret string log value to be empty, got %q", emptyLogVal.String())
	}
}
