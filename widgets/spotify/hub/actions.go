package spotify

import (
	"context"
	"fmt"
	"strings"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets/spotify/hub/internal/provider"
)

type spotifyActionHandler func(
	ctx context.Context,
	client provider.Client,
	arguments map[string]any,
) (map[string]any, error)

type spotifyActionCatalogItem struct {
	action widgetskills.Action
	handle spotifyActionHandler
}

var spotifyActionCatalog = []spotifyActionCatalogItem{
	{
		action: widgetskills.ReadAction("status", "Get playback status", "Read current track and status."),
		handle: func(ctx context.Context, client provider.Client, _ map[string]any) (map[string]any, error) {
			playback, err := client.FetchPlayback(ctx)
			if err != nil {
				return nil, err
			}
			return spotifyPlaybackData(playback), nil
		},
	},
	{
		action: spotifySearchAction(),
		handle: func(ctx context.Context, client provider.Client, arguments map[string]any) (map[string]any, error) {
			query := stringArgument(arguments, "query")
			itemType := stringArgument(arguments, "type")
			if itemType == "" {
				itemType = "track"
			}
			limit := intArgument(arguments, "limit", 5)
			results, err := client.Search(ctx, query, itemType, limit)
			if err != nil {
				return nil, err
			}
			return map[string]any{"results": results}, nil
		},
	},
	spotifyPlaybackCatalogItem("play", "Play", "Start Spotify playback."),
	spotifyPlaybackCatalogItem("pause", "Pause", "Pause Spotify playback."),
	spotifyPlaybackCatalogItem("next", "Next track", "Skip to the next Spotify track."),
	spotifyPlaybackCatalogItem("previous", "Previous track", "Skip to the previous Spotify track."),
	spotifyPlaybackCatalogItem(
		"restart_track",
		"Restart track",
		"Restart the current Spotify track from the beginning.",
	),
	spotifyPlaybackCatalogItem("seek", "Seek", "Seek within the current Spotify track."),
	spotifyPlaybackCatalogItem("play_album", "Play album", "Search for and play a Spotify album by query or URI."),
	spotifyPlaybackCatalogItem("play_track", "Play song", "Search for and play a Spotify song by query or URI."),
	spotifyPlaybackCatalogItem(
		"play_playlist",
		"Play playlist",
		"Search for and play a Spotify playlist by query or URI.",
	),
	spotifyPlaybackCatalogItem("set_volume", "Set volume", "Set Spotify player volume."),
	spotifyPlaybackCatalogItem("set_shuffle", "Set shuffle", "Enable or disable Spotify shuffle."),
	spotifyPlaybackCatalogItem("set_repeat", "Set repeat", "Set Spotify repeat mode to off, context, or track."),
	spotifyPlaybackCatalogItem(
		"transfer_playback",
		"Use Jute player",
		"Transfer Spotify playback to this Jute Dash display.",
	),
}

func spotifySkillActions() []widgetskills.Action {
	actions := make([]widgetskills.Action, 0, len(spotifyActionCatalog))
	for _, item := range spotifyActionCatalog {
		actions = append(actions, item.action)
	}
	return actions
}

func invokeSpotifyAction(
	ctx context.Context,
	client provider.Client,
	actionID string,
	arguments map[string]any,
) (map[string]any, error) {
	for _, item := range spotifyActionCatalog {
		if item.action.ID == actionID {
			return item.handle(ctx, client, arguments)
		}
	}
	return nil, fmt.Errorf("unknown action: %s", actionID)
}

func spotifySearchAction() widgetskills.Action {
	return widgetskills.Action{
		ID:                   "search",
		Title:                "Search",
		Description:          "Search Spotify albums, songs, or playlists and return safe suggestions.",
		SideEffect:           "read",
		RequiresConfirmation: false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Spotify search query.",
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"album", "track", "playlist"},
					"description": "Search result type.",
				},
				"limit": map[string]any{
					"type":        "number",
					"minimum":     1,
					"maximum":     10,
					"description": "Maximum number of suggestions.",
				},
			},
			"required":             []string{"query", "type"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"results": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":            map[string]any{"type": "string"},
							"type":          map[string]any{"type": "string"},
							"name":          map[string]any{"type": "string"},
							"subtitle":      map[string]any{"type": "string"},
							"uri":           map[string]any{"type": "string"},
							"album_art_url": map[string]any{"type": "string"},
						},
					},
				},
			},
			"required": []string{"results"},
		},
	}
}

func spotifyPlaybackCatalogItem(id, title, description string) spotifyActionCatalogItem {
	return spotifyActionCatalogItem{
		action: spotifyPlaybackAction(id, title, description),
		handle: func(ctx context.Context, client provider.Client, arguments map[string]any) (map[string]any, error) {
			if err := client.ApplyAction(ctx, id, arguments); err != nil {
				return nil, err
			}
			return map[string]any{"status": "ok"}, nil
		},
	}
}

func spotifyPlaybackAction(id, title, description string) widgetskills.Action {
	properties := map[string]any{}
	switch id {
	case "set_volume":
		properties["volume"] = map[string]any{
			"type":        "number",
			"minimum":     0,
			"maximum":     100,
			"description": "Target volume percentage.",
		}
	case "set_shuffle":
		properties["state"] = map[string]any{
			"type":        "boolean",
			"description": "Whether shuffle should be enabled.",
		}
	case "set_repeat":
		properties["state"] = map[string]any{
			"type":        "string",
			"enum":        []string{"off", "context", "track"},
			"description": "Repeat mode.",
		}
	case "seek":
		properties["position_ms"] = map[string]any{
			"type":        "number",
			"minimum":     0,
			"description": "Target playback position in milliseconds.",
		}
	case "play", "play_album", "play_track", "play_playlist":
		properties["uri"] = map[string]any{
			"type":        "string",
			"description": "Optional Spotify URI to play directly.",
		}
		properties["query"] = map[string]any{
			"type":        "string",
			"description": "Song or playlist search query when a URI is not provided.",
		}
	case "transfer_playback":
		properties["device_id"] = map[string]any{
			"type":        "string",
			"description": "Spotify device ID for the Jute web player.",
		}
		properties["play"] = map[string]any{
			"type":        "boolean",
			"description": "Whether playback should start immediately after transfer.",
		}
	}

	return widgetskills.Action{
		ID:                   id,
		Title:                title,
		Description:          description,
		SideEffect:           "home_action",
		RequiresConfirmation: false,
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           properties,
			"additionalProperties": true,
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
	}
}

func stringArgument(arguments map[string]any, key string) string {
	value, _ := arguments[key].(string)
	value = strings.TrimSpace(value)
	return value
}

func intArgument(arguments map[string]any, key string, fallback int) int {
	switch value := arguments[key].(type) {
	case int:
		if value > 0 {
			return value
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	}
	return fallback
}
