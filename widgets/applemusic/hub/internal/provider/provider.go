package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Settings struct {
	DeveloperToken string
	UserToken      string
}

type Playback struct {
	TrackTitle string `json:"track_title"`
	ArtistName string `json:"artist_name"`
	IsPlaying  bool   `json:"is_playing"`
}

type Client struct {
	settings Settings
}

func NewClient(settings Settings) Client {
	return Client{settings: settings}
}

func (c Client) FetchPlayback(ctx context.Context) (Playback, error) {
	if c.settings.DeveloperToken == "mock-applemusic" || c.settings.DeveloperToken == "test" {
		return Playback{
			TrackTitle: "Mock Track",
			ArtistName: "Mock Artist",
			IsPlaying:  true,
		}, nil
	}

	resp, err := c.doRequest(ctx, http.MethodGet, "https://api.music.apple.com/v1/me/player/currently-playing", nil)
	if err != nil {
		return Playback{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return Playback{
			TrackTitle: "Not Playing",
			ArtistName: "Unknown",
			IsPlaying:  false,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return Playback{}, fmt.Errorf("apple music API returned status %d", resp.StatusCode)
	}

	var responseData struct {
		Data []struct {
			Attributes struct {
				Name       string `json:"name"`
				ArtistName string `json:"artistName"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		return Playback{}, err
	}

	playback := Playback{
		TrackTitle: "Not Playing",
		ArtistName: "Unknown",
		IsPlaying:  false,
	}
	if len(responseData.Data) > 0 {
		playback.TrackTitle = responseData.Data[0].Attributes.Name
		playback.ArtistName = responseData.Data[0].Attributes.ArtistName
		playback.IsPlaying = true
	}
	return playback, nil
}

func (c Client) ApplyAction(ctx context.Context, actionID string) error {
	if c.settings.DeveloperToken == "mock-applemusic" || c.settings.DeveloperToken == "test" {
		return nil
	}

	var urlStr string
	switch actionID {
	case "play":
		urlStr = "https://api.music.apple.com/v1/me/player/play"
	case "pause":
		urlStr = "https://api.music.apple.com/v1/me/player/pause"
	case "next":
		urlStr = "https://api.music.apple.com/v1/me/player/next"
	case "previous":
		urlStr = "https://api.music.apple.com/v1/me/player/previous"
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, urlStr, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("apple music API returned status %d", resp.StatusCode)
	}
	return nil
}

func (c Client) doRequest(
	ctx context.Context,
	method, urlStr string,
	body []byte,
) (*http.Response, error) {
	var rBody io.Reader
	if body != nil {
		rBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, rBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.settings.DeveloperToken))
	req.Header.Set("Music-User-Token", c.settings.UserToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return http.DefaultClient.Do(req)
}
