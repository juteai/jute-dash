package spotify

import (
	"context"
	"errors"
	"log/slog"

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
}

func (s Settings) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("client_id", s.ClientID),
		slog.Any("client_secret", s.ClientSecret),
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

func (w *SpotifyWidget) FetchData(_ context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // default permitted for widgets
		"fetching spotify data",
	)
	s := parseSettings(rawSettings)
	if s.ClientID == "" || string(s.ClientSecret) == "" {
		return map[string]any{
			"is_configured": false,
		}, nil
	}
	return map[string]any{
		"is_configured": true,
		"track_title":   "Not Playing",
		"artist_name":   "Unknown",
		"is_playing":    false,
		"volume":        50,
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
	_ context.Context,
	_ widgetskills.Snapshot,
	_ string,
	actionID string,
	_ map[string]any,
) (map[string]any, error) {
	slog.Info( //nolint:sloglint // use default global logger
		"spotify action invoked",
		"actionID", actionID,
	)
	return nil, errors.New("live integration not implemented")
}

func parseSettings(raw map[string]any) Settings {
	s := Settings{}
	if v, ok := raw["client_id"].(string); ok {
		s.ClientID = v
	}
	if v, ok := raw["client_secret"].(string); ok {
		s.ClientSecret = SecretString(v)
	}
	return s
}

func init() {
	widgets.RegisterWithSkill(&SpotifyWidget{}, func(_ widgetskills.Snapshot, _ string) map[string]any {
		return map[string]any{
			"track_title": "Not Playing",
			"artist_name": "Unknown",
			"is_playing":  false,
			"volume":      50,
		}
	})
}
