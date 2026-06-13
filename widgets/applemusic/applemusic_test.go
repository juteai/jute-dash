package applemusic

import (
	"context"
	"testing"

	"jute-dash/apps/hub/pkg/widgetskills"
)

func TestAppleMusicWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "apple-music" {
		t.Errorf("expected kind 'apple-music', got %q", w.Kind())
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

	// 2. Partially configured (no user token)
	rawPartial := map[string]any{
		"developer_token": "my_dev_token",
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
		t.Errorf("expected is_configured to be false without user token")
	}

	// 3. Mock/Test mode (configured with mock)
	rawMock := map[string]any{
		"developer_token": "test",
		"user_token":      "my_user_token",
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

func TestAppleMusicWidgetActions(t *testing.T) {
	w := NewWidget()

	// Setup snapshot with a configured widget instance
	snap := widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					ID:   "apple-music-1",
					Kind: Kind,
					Settings: map[string]any{
						"developer_token": "test",
						"user_token":      "user_token",
					},
				},
			},
		},
	}

	// Invoke actions in mock mode
	actions := []string{"play", "pause", "next", "previous"}
	for _, act := range actions {
		res, err := w.InvokeAction(context.Background(), snap, "apple-music-1", act, nil)
		if err != nil {
			t.Errorf("action %q failed: %v", act, err)
		}
		if res["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", res["status"])
		}
	}
}

func TestAppleMusicLogValueRedaction(t *testing.T) {
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
