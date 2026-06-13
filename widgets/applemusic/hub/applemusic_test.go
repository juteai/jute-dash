package applemusic

import (
	"context"
	"testing"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

func TestAppleMusicWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "apple-music" {
		t.Errorf("expected kind 'apple-music', got %q", w.Kind())
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
		InstanceID: "apple-music-1",
		Connections: map[string]widgets.ResolvedConnection{
			"account": appleMusicConnection(),
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
}

func TestAppleMusicWidgetActions(t *testing.T) {
	w := NewWidget()

	snap := widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					ID:             "apple-music-1",
					Kind:           Kind,
					ConnectionRefs: map[string]string{"account": "apple-test"},
				},
			},
		},
	}

	// Invoke actions in mock mode
	actions := []string{"play", "pause", "next", "previous"}
	for _, act := range actions {
		res, err := w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
			RuntimeInput: widgets.RuntimeInput{
				InstanceID: "apple-music-1",
				Connections: map[string]widgets.ResolvedConnection{
					"account": appleMusicConnection(),
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
}

func appleMusicConnection() widgets.ResolvedConnection {
	return widgets.ResolvedConnection{
		ID:   "apple-test",
		Kind: "apple-music",
		Secrets: map[string]string{
			"developer_token": "test",
			"user_token":      "user_token",
		},
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
