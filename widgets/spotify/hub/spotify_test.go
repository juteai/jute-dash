package spotify

import (
	"context"
	"testing"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

func TestSpotifyWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "spotify" {
		t.Errorf("expected kind 'spotify', got %q", w.Kind())
	}

	data, err := w.FetchData(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("FetchData empty failed: %v", err)
	}
	payload, ok := data.(widgets.RuntimePayload)
	if !ok {
		t.Fatalf("expected RuntimePayload, got %T", data)
	}
	if payload.Status != widgets.StatusUnavailable {
		t.Errorf("expected unavailable status, got %q", payload.Status)
	}

	payload, err = w.FetchDataWithConnections(context.Background(), widgets.RuntimeInput{
		InstanceID: "spotify-1",
		Connections: map[string]widgets.ResolvedConnection{
			"account": spotifyConnection(),
		},
	})
	if err != nil {
		t.Fatalf("FetchData mock failed: %v", err)
	}
	if payload.Status != widgets.StatusOK {
		t.Fatalf("expected ok status, got %q", payload.Status)
	}
	m, ok := payload.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", payload.Data)
	}
	if m["track_title"] != "Mock Track" {
		t.Errorf("expected track title 'Mock Track', got %q", m["track_title"])
	}
	if m["album_art_url"] != "https://example.test/mock-album.jpg" {
		t.Errorf("expected mock album art URL, got %q", m["album_art_url"])
	}
}

func TestSpotifyWidgetActions(t *testing.T) {
	w := NewWidget()

	snap := widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					ID:             "spotify-1",
					Kind:           Kind,
					ConnectionRefs: map[string]string{"account": "spotify-test"},
				},
			},
		},
	}

	// Invoke actions in mock mode
	actions := []string{"play", "pause", "next", "previous"}
	for _, act := range actions {
		res, err := w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
			RuntimeInput: widgets.RuntimeInput{
				InstanceID: "spotify-1",
				Connections: map[string]widgets.ResolvedConnection{
					"account": spotifyConnection(),
				},
			},
			Snapshot: snap,
			ActionID: act,
		})
		if err != nil {
			t.Errorf("action %q failed: %v", act, err)
		}
		if res["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", res["status"])
		}
	}

	// Set volume
	res, err := w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "spotify-1",
			Connections: map[string]widgets.ResolvedConnection{
				"account": spotifyConnection(),
			},
		},
		Snapshot:  snap,
		ActionID:  "set_volume",
		Arguments: map[string]any{"volume": 80},
	})
	if err != nil {
		t.Errorf("set_volume failed: %v", err)
	}
	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}
}

func spotifyConnection() widgets.ResolvedConnection {
	return widgets.ResolvedConnection{
		ID:   "spotify-test",
		Kind: "spotify",
		Settings: map[string]any{
			"client_id": "test",
		},
		Secrets: map[string]string{
			"client_secret": "secret",
			"access_token":  "token",
			"refresh_token": "refresh",
		},
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
