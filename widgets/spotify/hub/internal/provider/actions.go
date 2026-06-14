package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

func (c Client) ApplyAction(ctx context.Context, actionID string, arguments map[string]any) error {
	if c.settings.ClientID == "mock-spotify" || c.settings.ClientID == "test" {
		return nil
	}

	var method, urlStr string
	var body []byte
	var err error

	switch actionID {
	case "play":
		method = http.MethodPut
		urlStr = "https://api.spotify.com/v1/me/player/play"
		if uri, ok := stringArgument(arguments, "uri"); ok {
			body, err = json.Marshal(map[string]any{"uris": []string{uri}})
			if err != nil {
				return err
			}
		}
	case "pause":
		method = http.MethodPut
		urlStr = "https://api.spotify.com/v1/me/player/pause"
	case "next":
		method = http.MethodPost
		urlStr = "https://api.spotify.com/v1/me/player/next"
	case "previous":
		method = http.MethodPost
		urlStr = "https://api.spotify.com/v1/me/player/previous"
	case "restart_track":
		method = http.MethodPut
		urlStr = "https://api.spotify.com/v1/me/player/seek?position_ms=0"
	case "seek":
		positionMS, err := positionArgument(arguments)
		if err != nil {
			return err
		}
		method = http.MethodPut
		urlStr = fmt.Sprintf("https://api.spotify.com/v1/me/player/seek?position_ms=%d", positionMS)
	case "play_album":
		return c.playSearchResult(ctx, arguments, "album")
	case "play_track":
		return c.playSearchResult(ctx, arguments, "track")
	case "play_playlist":
		return c.playSearchResult(ctx, arguments, "playlist")
	case "set_volume":
		vol, err := volumeArgument(arguments)
		if err != nil {
			return err
		}
		method = http.MethodPut
		urlStr = fmt.Sprintf("https://api.spotify.com/v1/me/player/volume?volume_percent=%d", vol)
	case "set_shuffle":
		state, err := shuffleArgument(arguments)
		if err != nil {
			return err
		}
		method = http.MethodPut
		urlStr = fmt.Sprintf("https://api.spotify.com/v1/me/player/shuffle?state=%t", state)
	case "set_repeat":
		state, err := repeatArgument(arguments)
		if err != nil {
			return err
		}
		method = http.MethodPut
		urlStr = fmt.Sprintf("https://api.spotify.com/v1/me/player/repeat?state=%s", url.QueryEscape(state))
	case "transfer_playback":
		deviceID, ok := stringArgument(arguments, "device_id")
		if !ok {
			return errors.New("missing or invalid device_id parameter")
		}
		play, _ := boolArgument(arguments, "play")
		method = http.MethodPut
		urlStr = "https://api.spotify.com/v1/me/player"
		body, err = json.Marshal(map[string]any{
			"device_ids": []string{deviceID},
			"play":       play,
		})
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}

	if actionID != "transfer_playback" {
		urlStr = withDeviceID(urlStr, arguments)
	}
	resp, err := c.doRequest(ctx, method, urlStr, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}
	return nil
}

func (c Client) playSearchResult(ctx context.Context, arguments map[string]any, itemType string) error {
	uri, ok := stringArgument(arguments, "uri")
	if !ok {
		query, hasQuery := stringArgument(arguments, "query")
		if !hasQuery {
			return errors.New("missing uri or query parameter")
		}
		matches, err := c.Search(ctx, query, itemType, 1)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("spotify %s was not found", itemType)
		}
		uri = matches[0].URI
	}
	if uri == "" {
		return fmt.Errorf("spotify %s was not found", itemType)
	}

	body := map[string]any{}
	switch itemType {
	case "playlist":
		body["context_uri"] = uri
	case "album":
		body["context_uri"] = uri
	default:
		body["uris"] = []string{uri}
	}
	playBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(
		ctx,
		http.MethodPut,
		withDeviceID("https://api.spotify.com/v1/me/player/play", arguments),
		playBody,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}
	return nil
}
