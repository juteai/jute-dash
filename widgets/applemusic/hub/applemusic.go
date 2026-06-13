package applemusic

import (
	"context"
	"log/slog"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"jute-dash/widgets/applemusic/hub/internal/provider"
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
		Kind:                   Kind,
		Name:                   "Apple Music",
		Description:            "Control playback and view track info from Apple Music.",
		DefaultTitle:           "Apple Music",
		DefaultW:               6,
		DefaultH:               2,
		MinW:                   4,
		MinH:                   2,
		DefaultSize:            "wide",
		Overflow:               "clip",
		AllowMultiple:          false,
		ConnectionRequirements: w.RequiredConnections(),
	}
}

func (w *AppleMusicWidget) RequiredConnections() []widgets.ConnectionRequirement {
	return []widgets.ConnectionRequirement{{
		Slot:        "account",
		Kind:        "apple-music",
		DisplayName: "Apple Music Account",
		Description: "Apple Music developer and user token references.",
		Required:    true,
		SecretKeys:  []string{"developer_token", "user_token"},
		Fields: []widgets.ConnectionField{
			{
				ID:       "developer_token",
				Type:     widgets.ConnectionFieldString,
				Label:    "Developer token reference",
				Required: true,
				Secret:   true,
				Help:     "Use a secret reference such as env:APPLE_MUSIC_DEVELOPER_TOKEN.",
			},
			{
				ID:       "user_token",
				Type:     widgets.ConnectionFieldString,
				Label:    "Music user token reference",
				Required: true,
				Secret:   true,
				Help:     "Use a secret reference such as env:APPLE_MUSIC_USER_TOKEN.",
			},
		},
	}}
}

func (w *AppleMusicWidget) FetchData(_ context.Context, _ map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // default permitted for widgets
		"fetching apple music data",
	)
	return widgets.Unavailable(
		"connection.missing",
		"Apple Music account needed",
		"Choose an Apple Music Account connection in settings.",
	), nil
}

func (w *AppleMusicWidget) FetchDataWithConnections(
	ctx context.Context,
	input widgets.RuntimeInput,
) (widgets.RuntimePayload, error) {
	settings := appleMusicSettingsFromConnection(input.Connections["account"])
	if string(settings.DeveloperToken) == "" || string(settings.UserToken) == "" {
		return widgets.Unavailable(
			"connection.missing_credentials",
			"Apple Music account needed",
			"Choose an Apple Music Account connection in settings.",
		), nil
	}
	playback, err := provider.NewClient(providerSettings(settings)).FetchPlayback(ctx)
	if err != nil {
		return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
			"apple_music.unavailable",
			"Apple Music unavailable",
			"Jute could not load Apple Music playback.",
		), nil
	}
	return widgets.OK(map[string]any{
		"track_title": playback.TrackTitle,
		"artist_name": playback.ArtistName,
		"is_playing":  playback.IsPlaying,
	}), nil
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
			applePlaybackAction("play", "Play", "Start Apple Music playback."),
			applePlaybackAction("pause", "Pause", "Pause Apple Music playback."),
			applePlaybackAction("next", "Next track", "Skip to the next Apple Music track."),
			applePlaybackAction("previous", "Previous track", "Return to the previous Apple Music track."),
		},
	}
}

func applePlaybackAction(id, title, description string) widgetskills.Action {
	return widgetskills.Action{
		ID:                   id,
		Title:                title,
		Description:          description,
		SideEffect:           "home_action",
		RequiresConfirmation: false,
		InputSchema:          map[string]any{"type": "object", "additionalProperties": true},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
	}
}

func providerSettings(settings Settings) provider.Settings {
	return provider.Settings{
		DeveloperToken: string(settings.DeveloperToken),
		UserToken:      string(settings.UserToken),
	}
}

func (w *AppleMusicWidget) InvokeActionWithConnections(
	ctx context.Context,
	input widgets.ActionInput,
) (map[string]any, error) {
	settings := appleMusicSettingsFromConnection(input.Connections["account"])
	slog.Info( //nolint:sloglint // use default global logger
		"apple music action invoked",
		"actionID", input.ActionID,
	)
	if err := provider.NewClient(providerSettings(settings)).ApplyAction(ctx, input.ActionID); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
}

func appleMusicSettingsFromConnection(connection widgets.ResolvedConnection) Settings {
	return Settings{
		DeveloperToken: SecretString(connection.Secrets["developer_token"]),
		UserToken:      SecretString(connection.Secrets["user_token"]),
	}
}

func init() {
	widgets.RegisterWithSkill(&AppleMusicWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
		for _, w := range snapshot.Layout.Widgets {
			if w.ID == instID {
				if m, ok := widgets.PayloadData(w.Data).(map[string]any); ok {
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
