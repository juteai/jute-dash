package spotify

import (
	"context"
	"errors"
	"log/slog"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"jute-dash/widgets/spotify/hub/internal/provider"
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
		Kind:                   Kind,
		Name:                   "Spotify",
		Description:            "Control playback and view track info from Spotify.",
		DefaultTitle:           "Spotify",
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

func (w *SpotifyWidget) RequiredConnections() []widgets.ConnectionRequirement {
	return []widgets.ConnectionRequirement{{
		Slot:        "account",
		Kind:        "spotify",
		DisplayName: "Spotify Account",
		Description: "Spotify Web API client and OAuth token material.",
		Required:    true,
		SecretKeys:  []string{"client_secret", "access_token", "refresh_token"},
	}}
}

func (w *SpotifyWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // default permitted for widgets
		"fetching spotify data",
	)
	s := parseSettings(rawSettings)
	if s.ClientID == "" || string(s.ClientSecret) == "" {
		return widgets.Unavailable(
			"connection.missing",
			"Spotify account needed",
			"Choose a Spotify Account connection in settings.",
		), nil
	}

	if string(s.AccessToken) == "" {
		return widgets.Unavailable(
			"connection.missing_credentials",
			"Spotify account needed",
			"Choose a Spotify Account connection with an access token.",
		), nil
	}

	playback, err := provider.NewClient(providerSettings(s)).FetchPlayback(ctx)
	if err != nil {
		return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
			"spotify.unavailable",
			"Spotify unavailable",
			"Jute could not load Spotify playback.",
		), nil
	}

	return map[string]any{
		"track_title": playback.TrackTitle,
		"artist_name": playback.ArtistName,
		"is_playing":  playback.IsPlaying,
		"volume":      playback.Volume,
	}, nil
}

func (w *SpotifyWidget) FetchDataWithConnections(
	ctx context.Context,
	input widgets.RuntimeInput,
) (widgets.RuntimePayload, error) {
	settings := spotifySettingsFromConnection(input.Connections["account"])
	if settings.ClientID == "" || string(settings.AccessToken) == "" {
		return widgets.Unavailable(
			"connection.missing_credentials",
			"Spotify account needed",
			"Choose a Spotify Account connection in settings.",
		), nil
	}
	data, err := w.FetchData(ctx, map[string]any{
		"client_id":     settings.ClientID,
		"client_secret": string(settings.ClientSecret),
		"access_token":  string(settings.AccessToken),
		"refresh_token": string(settings.RefreshToken),
		"expires_at":    settings.ExpiresAt,
		"instanceId":    input.InstanceID,
	})
	if err != nil {
		return widgets.ErrorPayload( //nolint:nilerr // provider error is mapped to a safe widget issue
			"spotify.fetch_failed",
			"Spotify unavailable",
			"Jute could not load Spotify playback.",
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
			playbackAction("play", "Play", "Start Spotify playback."),
			playbackAction("pause", "Pause", "Pause Spotify playback."),
			playbackAction("next", "Next track", "Skip to the next Spotify track."),
			playbackAction("previous", "Previous track", "Return to the previous Spotify track."),
			playbackAction("set_volume", "Set volume", "Set Spotify player volume."),
		},
	}
}

func playbackAction(id, title, description string) widgetskills.Action {
	return widgetskills.Action{
		ID:                   id,
		Title:                title,
		Description:          description,
		SideEffect:           "home_action",
		RequiresConfirmation: false,
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
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

	if err := provider.NewClient(providerSettings(s)).ApplyAction(ctx, actionID, arguments); err != nil {
		return nil, err
	}

	return map[string]any{"status": "ok"}, nil
}

func providerSettings(settings Settings) provider.Settings {
	return provider.Settings{
		ClientID:     settings.ClientID,
		ClientSecret: string(settings.ClientSecret),
		AccessToken:  string(settings.AccessToken),
		RefreshToken: string(settings.RefreshToken),
		ExpiresAt:    settings.ExpiresAt,
	}
}

func (w *SpotifyWidget) InvokeActionWithConnections(
	ctx context.Context,
	input widgets.ActionInput,
) (map[string]any, error) {
	settings := spotifySettingsFromConnection(input.Connections["account"])
	snap := input.Snapshot
	snap.Layout.Widgets = append([]widgetskills.WidgetInstance(nil), snap.Layout.Widgets...)
	found := false
	for i := range snap.Layout.Widgets {
		if snap.Layout.Widgets[i].ID == input.InstanceID {
			snap.Layout.Widgets[i].Settings = map[string]any{
				"client_id":     settings.ClientID,
				"client_secret": string(settings.ClientSecret),
				"access_token":  string(settings.AccessToken),
				"refresh_token": string(settings.RefreshToken),
				"expires_at":    settings.ExpiresAt,
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

func spotifySettingsFromConnection(connection widgets.ResolvedConnection) Settings {
	settings := Settings{}
	if v, ok := connection.Settings["client_id"].(string); ok {
		settings.ClientID = v
	}
	settings.ClientSecret = SecretString(connection.Secrets["client_secret"])
	settings.AccessToken = SecretString(connection.Secrets["access_token"])
	settings.RefreshToken = SecretString(connection.Secrets["refresh_token"])
	if v, ok := connection.Settings["expires_at"].(int64); ok {
		settings.ExpiresAt = v
	} else if v, ok := connection.Settings["expires_at"].(float64); ok {
		settings.ExpiresAt = int64(v)
	}
	return settings
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
				if m, ok := widgets.PayloadData(w.Data).(map[string]any); ok {
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
