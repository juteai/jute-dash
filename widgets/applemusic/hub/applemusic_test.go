package applemusic

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"jute-dash/widgets/applemusic/hub/internal/provider"
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
	actions := []string{
		"play",
		"pause",
		"next",
		"previous",
		"restart_track",
		"seek",
		"play_album",
		"play_track",
		"play_playlist",
		"set_volume",
		"set_shuffle",
		"set_repeat",
		"search",
	}
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
			Arguments: map[string]any{
				"query":       "Mock Track",
				"type":        "track",
				"volume":      80,
				"position_ms": 30000,
				"state":       "context",
			},
		})
		if err != nil {
			t.Errorf("action %q failed: %v", act, err)
		}
		if act == "search" {
			results := reflect.ValueOf(res["results"])
			if results.Kind() != reflect.Slice || results.Len() == 0 {
				t.Errorf("expected search results, got %#v", res["results"])
			}
			continue
		}
		if res["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", res["status"])
		}
	}
}

func TestAppleMusicActionCatalogDrivesSkillAndInvocation(t *testing.T) {
	w := NewWidget()
	skill := w.Skill()
	declared := map[string]widgetskills.Action{}
	for _, action := range skill.Actions {
		declared[action.ID] = action
	}
	for _, actionID := range []string{
		"status",
		"search",
		"play_album",
		"play_track",
		"play_playlist",
		"seek",
		"set_shuffle",
		"set_repeat",
	} {
		if _, ok := declared[actionID]; !ok {
			t.Fatalf("expected action %q to be declared", actionID)
		}
	}

	res, err := w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "apple-music-1",
			Connections: map[string]widgets.ResolvedConnection{
				"account": appleMusicConnection(),
			},
		},
		ActionID: "status",
	})
	if err != nil {
		t.Fatalf("status action failed: %v", err)
	}
	if res["track_title"] != "Mock Track" {
		t.Fatalf("expected status action to return playback data, got %#v", res)
	}
}

func TestAppleMusicSuggestionCacheReturnsStaleItemsDuringCooldown(t *testing.T) {
	cache := newAppleMusicSuggestionCache()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	fetches := 0
	fetch := func(context.Context, int) ([]provider.PlayableItem, error) {
		fetches++
		if fetches == 1 {
			return []provider.PlayableItem{{
				ID:       "album-1",
				Type:     "album",
				Name:     "Mock Album",
				Subtitle: "Mock Artist",
				URI:      "apple-music:albums:album-1",
			}}, nil
		}
		return nil, errors.New("apple music API returned status 429")
	}

	_ = cache.get(context.Background(), "apple-main", fetch)
	now = now.Add(31 * time.Minute)
	stale := cache.get(context.Background(), "apple-main", fetch)
	again := cache.get(context.Background(), "apple-main", fetch)

	if fetches != 2 {
		t.Fatalf("expected failed refresh to start cooldown, got %d fetches", fetches)
	}
	if len(stale) != 1 || len(again) != 1 {
		t.Fatalf("expected stale albums during cooldown, got %#v then %#v", stale, again)
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
