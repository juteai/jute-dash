package applemusic

import (
	"context"
	"errors"
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
	}}
}

func (w *AppleMusicWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // default permitted for widgets
		"fetching apple music data",
	)
	s := parseSettings(rawSettings)
	if string(s.DeveloperToken) == "" || string(s.UserToken) == "" {
		return widgets.Unavailable(
			"connection.missing",
			"Apple Music account needed",
			"Choose an Apple Music Account connection in settings.",
		), nil
	}

	playback, err := provider.NewClient(providerSettings(s)).FetchPlayback(ctx)
	if err != nil {
		return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
			"apple_music.unavailable",
			"Apple Music unavailable",
			"Jute could not load Apple Music playback.",
		), nil
	}

	return map[string]any{
		"track_title": playback.TrackTitle,
		"artist_name": playback.ArtistName,
		"is_playing":  playback.IsPlaying,
	}, nil
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
	data, err := w.FetchData(ctx, map[string]any{
		"developer_token": string(settings.DeveloperToken),
		"user_token":      string(settings.UserToken),
	})
	if err != nil {
		return widgets.ErrorPayload( //nolint:nilerr // provider error is mapped to a safe widget issue
			"apple_music.fetch_failed",
			"Apple Music unavailable",
			"Jute could not load Apple Music playback.",
		), nil
	}
	payload := widgets.NormalizePayload(data, nil)
	if payload.Status != widgets.StatusOK {
		return payload, nil
	}
	if m, ok := data.(map[string]any); ok {
		return widgets.OK(m), nil
	}
	return widgets.OK(data), nil
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

	if err := provider.NewClient(providerSettings(s)).ApplyAction(ctx, actionID); err != nil {
		return nil, err
	}

	return map[string]any{"status": "ok"}, nil
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
	snap := input.Snapshot
	snap.Layout.Widgets = append([]widgetskills.WidgetInstance(nil), snap.Layout.Widgets...)
	found := false
	for i := range snap.Layout.Widgets {
		if snap.Layout.Widgets[i].ID == input.InstanceID {
			snap.Layout.Widgets[i].Settings = map[string]any{
				"developer_token": string(settings.DeveloperToken),
				"user_token":      string(settings.UserToken),
			}
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("widget instance not found")
	}
	return w.InvokeAction(ctx, snap, input.InstanceID, input.ActionID, input.Arguments)
}

func appleMusicSettingsFromConnection(connection widgets.ResolvedConnection) Settings {
	return Settings{
		DeveloperToken: SecretString(connection.Secrets["developer_token"]),
		UserToken:      SecretString(connection.Secrets["user_token"]),
	}
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
