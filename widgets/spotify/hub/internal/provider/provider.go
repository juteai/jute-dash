package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Settings struct {
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

type Playback struct {
	TrackTitle  string `json:"track_title"`
	ArtistName  string `json:"artist_name"`
	AlbumArtURL string `json:"album_art_url,omitempty"`
	IsPlaying   bool   `json:"is_playing"`
	Volume      int    `json:"volume"`
}

type Client struct {
	settings Settings
}

func NewClient(settings Settings) Client {
	return Client{settings: settings}
}

func (c Client) FetchPlayback(ctx context.Context) (Playback, error) {
	if c.settings.ClientID == "mock-spotify" || c.settings.ClientID == "test" {
		return Playback{
			TrackTitle:  "Mock Track",
			ArtistName:  "Mock Artist",
			AlbumArtURL: "https://example.test/mock-album.jpg",
			IsPlaying:   true,
			Volume:      75,
		}, nil
	}

	resp, err := c.doRequest(ctx, http.MethodGet, "https://api.spotify.com/v1/me/player", nil)
	if err != nil {
		return Playback{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return Playback{
			TrackTitle: "Not Playing",
			ArtistName: "Unknown",
			IsPlaying:  false,
			Volume:     50,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return Playback{}, fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}

	var playerState struct {
		IsPlaying bool `json:"is_playing"`
		Device    struct {
			VolumePercent int `json:"volume_percent"`
		} `json:"device"`
		Item *struct {
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Images []struct {
					URL    string `json:"url"`
					Height int    `json:"height"`
					Width  int    `json:"width"`
				} `json:"images"`
			} `json:"album"`
		} `json:"item"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&playerState); err != nil {
		return Playback{}, err
	}

	track := "Not Playing"
	artist := "Unknown"
	albumArtURL := ""
	if playerState.Item != nil {
		track = playerState.Item.Name
		var artistNames []string
		for _, a := range playerState.Item.Artists {
			artistNames = append(artistNames, a.Name)
		}
		if len(artistNames) > 0 {
			artist = strings.Join(artistNames, ", ")
		}
		albumArtURL = bestAlbumImage(playerState.Item.Album.Images)
	}

	return Playback{
		TrackTitle:  track,
		ArtistName:  artist,
		AlbumArtURL: albumArtURL,
		IsPlaying:   playerState.IsPlaying,
		Volume:      playerState.Device.VolumePercent,
	}, nil
}

func bestAlbumImage(images []struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}) string {
	if len(images) == 0 {
		return ""
	}
	for _, image := range images {
		if strings.TrimSpace(image.URL) != "" && image.Width <= 320 && image.Height <= 320 {
			return image.URL
		}
	}
	return strings.TrimSpace(images[len(images)-1].URL)
}

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
	case "set_volume":
		vol, err := volumeArgument(arguments)
		if err != nil {
			return err
		}
		method = http.MethodPut
		urlStr = fmt.Sprintf("https://api.spotify.com/v1/me/player/volume?volume_percent=%d", vol)
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

func boolArgument(arguments map[string]any, key string) (bool, bool) {
	value, ok := arguments[key].(bool)
	return value, ok
}

func stringArgument(arguments map[string]any, key string) (string, bool) {
	value, ok := arguments[key].(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func volumeArgument(arguments map[string]any) (int, error) {
	if v, ok := arguments["volume"].(float64); ok {
		return int(v), nil
	}
	if v, ok := arguments["volume"].(int); ok {
		return v, nil
	}
	return 0, errors.New("missing or invalid volume parameter")
}

func (c Client) refreshToken(ctx context.Context) (string, error) {
	if c.settings.RefreshToken == "" || c.settings.ClientID == "" {
		return "", errors.New("missing credentials for refresh")
	}

	tokenURL := "https://accounts.spotify.com/api/token" //nolint:gosec // URL is not a secret credential
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", c.settings.RefreshToken)
	data.Set("client_id", c.settings.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	if c.settings.ClientSecret != "" {
		authHeader := base64.StdEncoding.EncodeToString(
			[]byte(fmt.Sprintf("%s:%s", c.settings.ClientID, c.settings.ClientSecret)),
		)
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", authHeader))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

func (c Client) doRequest(
	ctx context.Context,
	method, urlStr string,
	body []byte,
) (*http.Response, error) {
	token := c.settings.AccessToken

	if token == "" || (c.settings.ExpiresAt > 0 && time.Now().Unix() >= c.settings.ExpiresAt-60) {
		newToken, err := c.refreshToken(ctx)
		if err == nil {
			token = newToken
		}
	}

	makeReq := func(tok string) (*http.Request, error) {
		var rBody io.Reader
		if body != nil {
			rBody = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, urlStr, rBody)
		if err != nil {
			return nil, err
		}
		if tok != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tok))
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}

	req, err := makeReq(token)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		newToken, err := c.refreshToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("unauthorized and refresh failed: %w", err)
		}
		req2, err := makeReq(newToken)
		if err != nil {
			return nil, err
		}
		return http.DefaultClient.Do(req2)
	}

	return resp, nil
}
