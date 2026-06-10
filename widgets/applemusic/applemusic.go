package applemusic

import (
	"context"
	"errors"
	"log/slog"

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

func (w *AppleMusicWidget) FetchData(_ context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // use default global logger
		"fetching apple music data",
	)
	s := parseSettings(rawSettings)
	if string(s.DeveloperToken) == "" {
		return map[string]any{
			"is_configured": false,
		}, nil
	}
	return map[string]any{
		"is_configured": true,
		"track_title":   "Not Playing",
		"artist_name":   "Unknown",
		"is_playing":    false,
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
	_ context.Context,
	_ widgetskills.Snapshot,
	_ string,
	actionID string,
	_ map[string]any,
) (map[string]any, error) {
	slog.Info( //nolint:sloglint // use default global logger
		"apple music action invoked",
		"actionID", actionID,
	)
	return nil, errors.New("live integration not implemented")
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
	widgets.RegisterWithSkill(&AppleMusicWidget{}, func(_ widgetskills.Snapshot, _ string) map[string]any {
		return map[string]any{
			"track_title": "Not Playing",
			"artist_name": "Unknown",
			"is_playing":  false,
		}
	})
}
