package spotify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

const (
	Kind    = "spotify"
	SkillID = "jute.spotify.control"
)

type SecretString string

func (s SecretString) LogValue() slog.Value {
	if s == "" {
		return slog.StringValue("")
	}
	return slog.StringValue("[redacted]")
}

type Settings struct {
	ClientID     string
	ClientSecret SecretString
	AccessToken  SecretString
	RefreshToken SecretString
	ExpiresAt    int64
	InstanceID   string
}

func (s Settings) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("client_id", s.ClientID),
		slog.Any("client_secret", s.ClientSecret),
		slog.Any("access_token", s.AccessToken),
		slog.Any("refresh_token", s.RefreshToken),
		slog.Int64("expires_at", s.ExpiresAt),
		slog.String("instance_id", s.InstanceID),
	)
}

type SpotifyWidget struct{}

func NewWidget() *SpotifyWidget {
	return &SpotifyWidget{}
}

func (w *SpotifyWidget) Kind() string {
	return Kind
}

func (w *SpotifyWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          Kind,
		Name:          "Spotify",
		Description:   "Control playback and view track info from Spotify.",
		DefaultTitle:  "Spotify",
		DefaultW:      6,
		DefaultH:      2,
		MinW:          4,
		MinH:          2,
		DefaultSize:   "wide",
		Overflow:      "clip",
		AllowMultiple: false,
		SettingsSchema: []widgets.SettingField{
			{
				ID:    "client_id",
				Type:  widgets.SettingString,
				Label: "Client ID",
				Help:  "Spotify Developer Client ID.",
			},
			{
				ID:    "client_secret",
				Type:  widgets.SettingString,
				Label: "Client Secret",
				Help:  "Spotify Developer Client Secret.",
			},
		},
	}
}

func (w *SpotifyWidget) refreshToken(ctx context.Context, s Settings) (string, error) {
	if string(s.RefreshToken) == "" || s.ClientID == "" || string(s.ClientSecret) == "" {
		return "", errors.New("missing credentials for refresh")
	}

	tokenURL := "https://accounts.spotify.com/api/token" //nolint:gosec // URL is not a secret credential
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", string(s.RefreshToken))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	authHeader := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", s.ClientID, string(s.ClientSecret))))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", authHeader))
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

	newSettings := map[string]any{
		"access_token": tokenResp.AccessToken,
		"expires_at":   time.Now().Unix() + tokenResp.ExpiresIn,
	}
	if tokenResp.RefreshToken != "" {
		newSettings["refresh_token"] = tokenResp.RefreshToken
	}

	if widgets.SaveSettingsHook != nil && s.InstanceID != "" {
		if err := widgets.SaveSettingsHook(ctx, s.InstanceID, newSettings); err != nil {
			slog.Error( //nolint:sloglint // global slog permitted for widgets
				"failed to save refreshed spotify settings",
				"error",
				err,
			)
		}
	}

	return tokenResp.AccessToken, nil
}

func (w *SpotifyWidget) doRequest(
	ctx context.Context,
	s Settings,
	method, urlStr string,
	body []byte,
) (*http.Response, error) {
	token := string(s.AccessToken)

	if token == "" || (s.ExpiresAt > 0 && time.Now().Unix() >= s.ExpiresAt-60) {
		newToken, err := w.refreshToken(ctx, s)
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
		newToken, err := w.refreshToken(ctx, s)
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

func (w *SpotifyWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // default permitted for widgets
		"fetching spotify data",
	)
	s := parseSettings(rawSettings)
	if s.ClientID == "" || string(s.ClientSecret) == "" {
		return map[string]any{
			"is_configured": false,
		}, nil
	}

	if string(s.AccessToken) == "" {
		return map[string]any{
			"is_configured": false,
		}, nil
	}

	if s.ClientID == "mock-spotify" || s.ClientID == "test" {
		return map[string]any{
			"is_configured": true,
			"track_title":   "Mock Track",
			"artist_name":   "Mock Artist",
			"is_playing":    true,
			"volume":        75,
		}, nil
	}

	resp, err := w.doRequest(ctx, s, http.MethodGet, "https://api.spotify.com/v1/me/player", nil)
	if err != nil {
		return map[string]any{
			"is_configured": true,
			"error":         err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return map[string]any{
			"is_configured": true,
			"track_title":   "Not Playing",
			"artist_name":   "Unknown",
			"is_playing":    false,
			"volume":        50,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return map[string]any{
			"is_configured": true,
			"error":         fmt.Sprintf("Spotify API returned status %d", resp.StatusCode),
		}, nil
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
		} `json:"item"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&playerState); err != nil {
		return map[string]any{
			"is_configured": true,
			"error":         err.Error(),
		}, nil
	}

	track := "Not Playing"
	artist := "Unknown"
	if playerState.Item != nil {
		track = playerState.Item.Name
		var artistNames []string
		for _, a := range playerState.Item.Artists {
			artistNames = append(artistNames, a.Name)
		}
		if len(artistNames) > 0 {
			artist = strings.Join(artistNames, ", ")
		}
	}

	return map[string]any{
		"is_configured": true,
		"track_title":   track,
		"artist_name":   artist,
		"is_playing":    playerState.IsPlaying,
		"volume":        playerState.Device.VolumePercent,
	}, nil
}

func (w *SpotifyWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          Kind,
		DisplayName:         "Spotify Control",
		Summary:             "Read playback status and trigger playback control actions on Spotify.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "track_title", Type: "string", Description: "Currently playing song title.", Sensitivity: "public"},
			{Name: "artist_name", Type: "string", Description: "Artist name.", Sensitivity: "public"},
			{Name: "is_playing", Type: "boolean", Description: "Is music active.", Sensitivity: "public"},
			{Name: "volume", Type: "number", Description: "Player volume.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("status", "Get playback status", "Read current track and status."),
		},
	}
}

func (w *SpotifyWidget) InvokeAction(
	ctx context.Context,
	snap widgetskills.Snapshot,
	instanceID string,
	actionID string,
	arguments map[string]any,
) (map[string]any, error) {
	slog.Info( //nolint:sloglint // use default global logger
		"spotify action invoked",
		"actionID", actionID,
	)

	s := getSettings(snap, instanceID)
	if s.ClientID == "" || string(s.ClientSecret) == "" {
		return nil, errors.New("spotify is not configured")
	}

	if s.ClientID == "mock-spotify" || s.ClientID == "test" {
		return map[string]any{"status": "ok"}, nil
	}

	var method, urlStr string
	var body []byte

	switch actionID {
	case "play":
		method = http.MethodPut
		urlStr = "https://api.spotify.com/v1/me/player/play"
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
		method = http.MethodPut
		var vol int
		if v, ok := arguments["volume"].(float64); ok {
			vol = int(v)
		} else if v, ok := arguments["volume"].(int); ok {
			vol = v
		} else {
			return nil, errors.New("missing or invalid volume parameter")
		}
		urlStr = fmt.Sprintf("https://api.spotify.com/v1/me/player/volume?volume_percent=%d", vol)
	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}

	resp, err := w.doRequest(ctx, s, method, urlStr, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}

	return map[string]any{"status": "ok"}, nil
}

func getSettings(snap widgetskills.Snapshot, instanceID string) Settings {
	for _, w := range snap.Layout.Widgets {
		if w.ID == instanceID {
			return parseSettings(w.Settings)
		}
	}
	return Settings{}
}

func parseSettings(raw map[string]any) Settings {
	s := Settings{}
	if v, ok := raw["client_id"].(string); ok {
		s.ClientID = v
	}
	if v, ok := raw["client_secret"].(string); ok {
		s.ClientSecret = SecretString(v)
	}
	if v, ok := raw["access_token"].(string); ok {
		s.AccessToken = SecretString(v)
	}
	if v, ok := raw["refresh_token"].(string); ok {
		s.RefreshToken = SecretString(v)
	}
	if v, ok := raw["expires_at"].(int64); ok {
		s.ExpiresAt = v
	} else if v, ok := raw["expires_at"].(float64); ok {
		s.ExpiresAt = int64(v)
	}
	if v, ok := raw["instanceId"].(string); ok {
		s.InstanceID = v
	}
	return s
}

func init() {
	widgets.RegisterWithSkill(&SpotifyWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
		for _, w := range snapshot.Layout.Widgets {
			if w.ID == instID {
				if m, ok := w.Data.(map[string]any); ok {
					return m
				}
			}
		}
		return map[string]any{
			"track_title": "Not Playing",
			"artist_name": "Unknown",
			"is_playing":  false,
			"volume":      50,
		}
	})
}
