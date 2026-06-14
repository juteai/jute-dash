package spotify

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"jute-dash/widgets/spotify/hub/internal/provider"
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
	if m["track_uri"] != "spotify:track:mock" {
		t.Errorf("expected mock track URI, got %q", m["track_uri"])
	}
	if m["progress_ms"] != 48000 || m["duration_ms"] != 192000 {
		t.Errorf("expected mock progress/duration, got %v/%v", m["progress_ms"], m["duration_ms"])
	}
	if m["shuffle"] != true || m["repeat_state"] != "context" {
		t.Errorf("expected mock shuffle/repeat, got %v/%v", m["shuffle"], m["repeat_state"])
	}
	topAlbums := reflect.ValueOf(m["top_albums"])
	if topAlbums.Kind() != reflect.Slice || topAlbums.Len() == 0 {
		t.Fatalf("expected mock top albums, got %#v", m["top_albums"])
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
	actions := []string{
		"play",
		"pause",
		"next",
		"previous",
		"restart_track",
		"play_album",
		"play_track",
		"play_playlist",
		"search",
	}
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
			Arguments: map[string]any{
				"query": "Mock Track",
				"type":  "track",
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

	// Seek
	res, err = w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "spotify-1",
			Connections: map[string]widgets.ResolvedConnection{
				"account": spotifyConnection(),
			},
		},
		Snapshot:  snap,
		ActionID:  "seek",
		Arguments: map[string]any{"position_ms": 30000},
	})
	if err != nil {
		t.Errorf("seek failed: %v", err)
	}
	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}

	// Set shuffle
	res, err = w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "spotify-1",
			Connections: map[string]widgets.ResolvedConnection{
				"account": spotifyConnection(),
			},
		},
		Snapshot:  snap,
		ActionID:  "set_shuffle",
		Arguments: map[string]any{"state": true},
	})
	if err != nil {
		t.Errorf("set_shuffle failed: %v", err)
	}
	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}

	// Set repeat
	res, err = w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "spotify-1",
			Connections: map[string]widgets.ResolvedConnection{
				"account": spotifyConnection(),
			},
		},
		Snapshot:  snap,
		ActionID:  "set_repeat",
		Arguments: map[string]any{"state": "track"},
	})
	if err != nil {
		t.Errorf("set_repeat failed: %v", err)
	}
	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}
}

func TestSpotifySkillDeclaresSearchPlaybackActions(t *testing.T) {
	w := NewWidget()
	skill := w.Skill()
	actionSchemas := map[string]map[string]any{}
	for _, action := range skill.Actions {
		actionSchemas[action.ID] = action.InputSchema
	}

	for _, actionID := range []string{
		"play_album",
		"play_track",
		"play_playlist",
		"restart_track",
		"seek",
		"set_shuffle",
		"set_repeat",
		"search",
	} {
		if _, ok := actionSchemas[actionID]; !ok {
			t.Fatalf("expected action %q to be declared", actionID)
		}
	}

	schema := actionSchemas["play_track"]
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected play_track properties, got %#v", schema["properties"])
	}
	if _, ok := properties["query"]; !ok {
		t.Fatalf("expected play_track query property, got %#v", properties)
	}
}

func TestSpotifyActionCatalogDrivesSkillAndInvocation(t *testing.T) {
	w := NewWidget()
	skill := w.Skill()
	declared := map[string]widgetskills.Action{}
	for _, action := range skill.Actions {
		declared[action.ID] = action
	}

	for _, item := range spotifyActionCatalog {
		action, ok := declared[item.action.ID]
		if !ok {
			t.Fatalf("catalog action %q was not declared by Skill", item.action.ID)
		}
		if action.SideEffect != item.action.SideEffect {
			t.Fatalf(
				"expected side effect %q for %q, got %q",
				item.action.SideEffect,
				item.action.ID,
				action.SideEffect,
			)
		}
	}

	res, err := w.InvokeActionWithConnections(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "spotify-1",
			Connections: map[string]widgets.ResolvedConnection{
				"account": spotifyConnection(),
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

func TestSpotifySuggestionCacheReusesAlbums(t *testing.T) {
	cache := newSpotifySuggestionCache()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	fetches := 0
	fetch := func(context.Context, int) ([]provider.Album, error) {
		fetches++
		return []provider.Album{{
			ID:          "album-1",
			Name:        "Odelay",
			ArtistName:  "Beck",
			URI:         "spotify:album:album-1",
			AlbumArtURL: "https://example.test/odelay.jpg",
		}}, nil
	}

	first := cache.get(context.Background(), "spotify-main", fetch)
	second := cache.get(context.Background(), "spotify-main", fetch)

	if fetches != 1 {
		t.Fatalf("expected one discovery fetch, got %d", fetches)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected cached albums, got %#v then %#v", first, second)
	}
}

func TestSpotifySuggestionCacheReturnsStaleAlbumsDuringCooldown(t *testing.T) {
	cache := newSpotifySuggestionCache()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	fetches := 0
	fetch := func(context.Context, int) ([]provider.Album, error) {
		fetches++
		if fetches == 1 {
			return []provider.Album{{
				ID:         "album-1",
				Name:       "Odelay",
				ArtistName: "Beck",
				URI:        "spotify:album:album-1",
			}}, nil
		}
		return nil, errors.New("spotify API returned status 429")
	}

	_ = cache.get(context.Background(), "spotify-main", fetch)
	now = now.Add(31 * time.Minute)
	stale := cache.get(context.Background(), "spotify-main", fetch)
	again := cache.get(context.Background(), "spotify-main", fetch)

	if fetches != 2 {
		t.Fatalf("expected failed refresh to start cooldown, got %d fetches", fetches)
	}
	if len(stale) != 1 || len(again) != 1 {
		t.Fatalf("expected stale albums during cooldown, got %#v then %#v", stale, again)
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
