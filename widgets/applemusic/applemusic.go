package applemusic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

const (
	Kind    = "apple-music"
	SkillID = "jute.applemusic.control"
)

type SecretString string

func (s SecretString) LogValue() slog.Value {
	if s == "" {
		return slog.StringValue("")
	}
	return slog.StringValue("[redacted]")
}

type Settings struct {
	DeveloperToken SecretString
	UserToken      SecretString
}

func (s Settings) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("developer_token", s.DeveloperToken),
		slog.Any("user_token", s.UserToken),
	)
}

type AppleMusicWidget struct{}

func NewWidget() *AppleMusicWidget {
	return &AppleMusicWidget{}
}

func (w *AppleMusicWidget) Kind() string {
	return Kind
}

func (w *AppleMusicWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          Kind,
		Name:          "Apple Music",
		Description:   "Control playback and view track info from Apple Music.",
		DefaultTitle:  "Apple Music",
		DefaultW:      6,
		DefaultH:      2,
		MinW:          4,
		MinH:          2,
		DefaultSize:   "wide",
		Overflow:      "clip",
		AllowMultiple: false,
		SettingsSchema: []widgets.SettingField{
			{
				ID:    "developer_token",
				Type:  widgets.SettingString,
				Label: "Developer Token",
				Help:  "JWT Developer Token from Apple Developer Account.",
			},
			{
				ID:    "user_token",
				Type:  widgets.SettingString,
				Label: "User Token",
				Help:  "Music User Token from client-side StoreKit.",
			},
		},
	}
}

func (w *AppleMusicWidget) doRequest(
	ctx context.Context,
	s Settings,
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

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(s.DeveloperToken)))
	req.Header.Set("Music-User-Token", string(s.UserToken))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return http.DefaultClient.Do(req)
}

func (w *AppleMusicWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // default permitted for widgets
		"fetching apple music data",
	)
	s := parseSettings(rawSettings)
	if string(s.DeveloperToken) == "" || string(s.UserToken) == "" {
		return map[string]any{
			"is_configured": false,
		}, nil
	}

	if string(s.DeveloperToken) == "mock-applemusic" || string(s.DeveloperToken) == "test" {
		return map[string]any{
			"is_configured": true,
			"track_title":   "Mock Track",
			"artist_name":   "Mock Artist",
			"is_playing":    true,
		}, nil
	}

	resp, err := w.doRequest(ctx, s, http.MethodGet, "https://api.music.apple.com/v1/me/player/currently-playing", nil)
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
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return map[string]any{
			"is_configured": true,
			"error":         fmt.Sprintf("Apple Music API returned status %d", resp.StatusCode),
		}, nil
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
		return map[string]any{
			"is_configured": true,
			"error":         err.Error(),
		}, nil
	}

	track := "Not Playing"
	artist := "Unknown"
	isPlaying := false

	if len(responseData.Data) > 0 {
		track = responseData.Data[0].Attributes.Name
		artist = responseData.Data[0].Attributes.ArtistName
		isPlaying = true
	}

	return map[string]any{
		"is_configured": true,
		"track_title":   track,
		"artist_name":   artist,
		"is_playing":    isPlaying,
	}, nil
}

func (w *AppleMusicWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          Kind,
		DisplayName:         "Apple Music Control",
		Summary:             "Read playback status and trigger playback control actions on Apple Music.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "track_title", Type: "string", Description: "Currently playing song title.", Sensitivity: "public"},
			{Name: "artist_name", Type: "string", Description: "Artist name.", Sensitivity: "public"},
			{Name: "is_playing", Type: "boolean", Description: "Is music active.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("status", "Get playback status", "Read current track and status."),
		},
	}
}

func (w *AppleMusicWidget) InvokeAction(
	ctx context.Context,
	snap widgetskills.Snapshot,
	instanceID string,
	actionID string,
	_ map[string]any,
) (map[string]any, error) {
	slog.Info( //nolint:sloglint // use default global logger
		"apple music action invoked",
		"actionID", actionID,
	)

	s := getSettings(snap, instanceID)
	if string(s.DeveloperToken) == "" || string(s.UserToken) == "" {
		return nil, errors.New("apple music is not configured")
	}

	if string(s.DeveloperToken) == "mock-applemusic" || string(s.DeveloperToken) == "test" {
		return map[string]any{"status": "ok"}, nil
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
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}

	resp, err := w.doRequest(ctx, s, http.MethodPost, urlStr, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("apple music API returned status %d", resp.StatusCode)
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
	if v, ok := raw["developer_token"].(string); ok {
		s.DeveloperToken = SecretString(v)
	}
	if v, ok := raw["user_token"].(string); ok {
		s.UserToken = SecretString(v)
	}
	return s
}

func init() {
	widgets.RegisterWithSkill(&AppleMusicWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
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
		}
	})
}
