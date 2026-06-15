package calendar

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	alertpolicy "jute-dash/widgets/alerts"
	"jute-dash/widgets/calendar/hub/internal/provider"
)

const (
	Kind    = "calendar"
	SkillID = "jute.calendar.events"
)

type CalendarWidget struct {
	client provider.Client
	now    func() time.Time
}

type Settings struct {
	NotificationSound string
	Timezone          string
	LookaheadDays     int
	AlertLeadMinutes  int
	DefaultSnoozeMins int
	DismissedAlerts   []string
	SnoozedAlerts     []SnoozedAlert
}

type SnoozedAlert struct {
	ID           string `json:"id"`
	SnoozedUntil string `json:"snoozedUntil"`
}

type EventAlert struct {
	ID                string         `json:"id"`
	Kind              string         `json:"kind"`
	Label             string         `json:"label"`
	Status            string         `json:"status"`
	DueAt             string         `json:"dueAt"`
	EventStart        string         `json:"eventStart"`
	EventEnd          string         `json:"eventEnd"`
	Location          string         `json:"location,omitempty"`
	Calendar          string         `json:"calendar"`
	Sound             string         `json:"sound"`
	Ringing           bool           `json:"ringing"`
	DefaultSnoozeMins int            `json:"defaultSnoozeMins"`
	Event             provider.Event `json:"event"`
}

func (w *CalendarWidget) Kind() string {
	return Kind
}

func (w *CalendarWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          Kind,
		Name:          "Calendar",
		Description:   "Upcoming events from a private iCalendar feed with full-screen event alerts.",
		DefaultTitle:  "Calendar",
		DefaultW:      6,
		DefaultH:      2,
		MinW:          3,
		MinH:          2,
		DefaultSize:   "medium",
		Overflow:      "clip",
		AllowMultiple: false,
		SettingsSchema: []widgets.SettingField{
			{
				ID:      "notificationSound",
				Type:    widgets.SettingEnum,
				Label:   "Notification sound",
				Default: alertpolicy.DefaultSound,
				Options: alertpolicy.SupportedSounds(),
			},
			{
				ID:      "timezone",
				Type:    widgets.SettingString,
				Label:   "Display timezone",
				Default: "UTC",
			},
			{
				ID:      "lookaheadDays",
				Type:    widgets.SettingNumber,
				Label:   "Days ahead",
				Default: 14,
			},
			{
				ID:      "alertLeadMinutes",
				Type:    widgets.SettingNumber,
				Label:   "Event alert lead minutes",
				Default: 10,
			},
			{
				ID:      "defaultSnoozeMins",
				Type:    widgets.SettingNumber,
				Label:   "Default snooze minutes",
				Default: alertpolicy.DefaultSnoozeMins,
			},
		},
		ConnectionRequirements: w.RequiredConnections(),
	}
}

func (w *CalendarWidget) RequiredConnections() []widgets.ConnectionRequirement {
	return []widgets.ConnectionRequirement{
		{
			Slot:        "account",
			Kind:        "calendar-account",
			DisplayName: "Calendar Account",
			Description: strings.Join([]string{
				"Private iCalendar feed account.",
				"CalDAV and plain IMAP email are not v1 calendar sync sources.",
			}, " "),
			Required:   true,
			SecretKeys: []string{"password"},
			Fields: []widgets.ConnectionField{
				{
					ID:       "feed_url",
					Type:     widgets.ConnectionFieldString,
					Label:    "Calendar URL",
					Required: true,
					Help:     "Private iCalendar .ics URL or provider calendar export URL.",
				},
				{
					ID:    "username",
					Type:  widgets.ConnectionFieldString,
					Label: "Username",
					Help:  "Optional for Basic Auth calendar feeds.",
				},
				{
					ID:     "password",
					Type:   widgets.ConnectionFieldString,
					Label:  "Password or app password reference",
					Secret: true,
					Help:   "Optional secret reference for Basic Auth calendar feeds.",
				},
				{
					ID:      "calendar_name",
					Type:    widgets.ConnectionFieldString,
					Label:   "Calendar name",
					Default: "Calendar",
				},
			},
		},
	}
}

func (w *CalendarWidget) FetchData(ctx context.Context, raw map[string]any) (any, error) {
	now := w.currentTime()
	if url, ok := raw["feedUrl"].(string); ok && strings.TrimSpace(url) != "" {
		settings := parseSettings(raw)
		events, err := w.client.Fetch(ctx, provider.Settings{
			FeedURL:       url,
			CalendarName:  "Calendar",
			Timezone:      settings.Timezone,
			LookaheadDays: settings.LookaheadDays,
		}, now)
		if err != nil {
			return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
				"calendar.feed_unavailable",
				"Calendar unavailable",
				"Jute could not load this calendar.",
			), nil
		}
		return widgets.OK(calendarData(events, settings, now)), nil
	}
	return widgets.Unavailable(
		"connection.missing",
		"Calendar account needed",
		"Choose a Calendar Account connection in settings.",
	), nil
}

func (w *CalendarWidget) FetchDataWithConnections(
	ctx context.Context,
	input widgets.RuntimeInput,
) (widgets.RuntimePayload, error) {
	settings := parseSettings(input.Settings)
	now := w.currentTime()
	conn := input.Connections["account"]
	providerSettings := provider.Settings{
		FeedURL:       stringSetting(conn.Settings, "feed_url"),
		Username:      stringSetting(conn.Settings, "username"),
		Password:      conn.Secrets["password"],
		CalendarName:  firstNonEmpty(stringSetting(conn.Settings, "calendar_name"), conn.Name, "Calendar"),
		Timezone:      settings.Timezone,
		LookaheadDays: settings.LookaheadDays,
	}
	if providerSettings.FeedURL == "" {
		return widgets.Unavailable(
			"connection.missing_settings",
			"Calendar URL needed",
			"Add an iCalendar URL to this Calendar Account connection.",
		), nil
	}
	events, err := w.client.Fetch(ctx, providerSettings, now)
	if err != nil {
		return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
			"calendar.feed_unavailable",
			"Calendar unavailable",
			"Jute could not load this calendar.",
		), nil
	}
	return widgets.OK(calendarData(events, settings, now)), nil
}

func (w *CalendarWidget) Skill() *widgetskills.Definition {
	return calendarSkill()
}

func (w *CalendarWidget) InvokeActionWithSettings(
	_ context.Context,
	input widgets.ActionInput,
) (widgets.SettingsMutationResult, error) {
	settings := parseSettings(input.Settings)
	now := w.currentTime()
	switch input.ActionID {
	case "snooze_event":
		id := strings.TrimSpace(stringArg(input.Arguments, "id", ""))
		if id == "" {
			return widgets.SettingsMutationResult{}, errors.New("id is required")
		}
		mins := intArg(input.Arguments, "minutes", settings.DefaultSnoozeMins)
		if mins <= 0 {
			mins = settings.DefaultSnoozeMins
		}
		settings.SnoozedAlerts = upsertSnoozed(settings.SnoozedAlerts, SnoozedAlert{
			ID:           id,
			SnoozedUntil: now.Add(time.Duration(mins) * time.Minute).Format(time.RFC3339),
		})
		return mutationResult(input, settings, id), nil
	case "dismiss_event":
		id := strings.TrimSpace(stringArg(input.Arguments, "id", ""))
		if id == "" {
			return widgets.SettingsMutationResult{}, errors.New("id is required")
		}
		if !contains(settings.DismissedAlerts, id) {
			settings.DismissedAlerts = append(settings.DismissedAlerts, id)
		}
		return mutationResult(input, settings, id), nil
	case "set_event_alert_lead":
		mins := intArg(input.Arguments, "minutes", settings.AlertLeadMinutes)
		if mins < 0 {
			mins = 0
		}
		settings.AlertLeadMinutes = mins
		return widgets.SettingsMutationResult{
			Body: map[string]any{
				"status":           "ok",
				"skillId":          SkillID,
				"widgetInstanceId": input.InstanceID,
				"actionId":         input.ActionID,
				"alertLeadMinutes": mins,
			},
			Settings: settings.toMap(),
		}, nil
	case "set_event_notification_sound":
		sound := soundArg(input.Arguments, "sound", settings.NotificationSound)
		settings.NotificationSound = sound
		return widgets.SettingsMutationResult{
			Body: map[string]any{
				"status":            "ok",
				"skillId":           SkillID,
				"widgetInstanceId":  input.InstanceID,
				"actionId":          input.ActionID,
				"notificationSound": sound,
			},
			Settings: settings.toMap(),
		}, nil
	default:
		return widgets.SettingsMutationResult{}, errors.New("unsupported calendar action")
	}
}

func calendarSkill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          Kind,
		DisplayName:         "Calendar",
		Summary:             "Read upcoming calendar events and manage event alert snooze or dismiss state.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{
				Name:        "events",
				Type:        "array",
				Description: "Upcoming public calendar events.",
				Sensitivity: "public",
			},
			{
				Name:        "nextEvent",
				Type:        "object",
				Description: "Next upcoming event.",
				Nullable:    true,
				Sensitivity: "public",
			},
			{
				Name:        "ringing",
				Type:        "array",
				Description: "Calendar event alerts currently due.",
				Sensitivity: "public",
			},
			{
				Name:        "alertLeadMinutes",
				Type:        "integer",
				Description: "Minutes before event start when alerts ring.",
				Sensitivity: "public",
			},
			{
				Name:        "notificationSound",
				Type:        "enum",
				Description: "Configured event notification sound.",
				EnumValues:  alertpolicy.SupportedSounds(),
				Sensitivity: "public",
			},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("read", "Read calendar events", "Read upcoming public calendar event context."),
			calendarAction(
				"snooze_event",
				"Snooze event alert",
				"Snooze a due calendar event alert.",
				map[string]any{
					"id":      map[string]any{"type": "string"},
					"minutes": map[string]any{"type": "integer", "minimum": 1},
				},
				[]string{"id"},
			),
			calendarAction(
				"dismiss_event",
				"Dismiss event alert",
				"Dismiss a calendar event alert for this occurrence.",
				map[string]any{"id": map[string]any{"type": "string"}},
				[]string{"id"},
			),
			calendarAction(
				"set_event_alert_lead",
				"Set event alert lead",
				"Set how many minutes before event start calendar alerts should ring.",
				map[string]any{"minutes": map[string]any{"type": "integer", "minimum": 0}},
				[]string{"minutes"},
			),
			calendarAction(
				"set_event_notification_sound",
				"Set event notification sound",
				"Set which local notification sound calendar event alerts use.",
				map[string]any{"sound": soundSchema()},
				[]string{"sound"},
			),
		},
		Prompts: []widgetskills.Prompt{
			{
				ID:      "calendar_briefing",
				Title:   "Use calendar context",
				Purpose: "Guide an agent when answering schedule, upcoming event, and event reminder questions.",
			},
		},
		SupportedWidgetSizes: []string{"medium", "wide", "large"},
	}
}

func calendarAction(
	id string,
	title string,
	description string,
	properties map[string]any,
	required []string,
) widgetskills.Action {
	return widgetskills.Action{
		ID:                   id,
		Title:                title,
		Description:          description,
		SideEffect:           "configure",
		RequiresConfirmation: false,
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
	}
}

func calendarData(events []provider.Event, settings Settings, now time.Time) map[string]any {
	alerts := eventAlerts(events, settings, now)
	ringing := make([]EventAlert, 0)
	for _, alert := range alerts {
		if alert.Ringing {
			ringing = append(ringing, alert)
		}
	}
	var next any
	if len(events) > 0 {
		next = events[0]
	}
	return map[string]any{
		"events":            events,
		"nextEvent":         next,
		"alerts":            alerts,
		"ringing":           ringing,
		"ringingCount":      len(ringing),
		"alertLeadMinutes":  settings.AlertLeadMinutes,
		"defaultSnoozeMins": settings.DefaultSnoozeMins,
		"notificationSound": settings.NotificationSound,
		"supportedSounds":   alertpolicy.SupportedSounds(),
		"generatedAt":       now.Format(time.RFC3339),
		"source":            "ics",
	}
}

func eventAlerts(events []provider.Event, settings Settings, now time.Time) []EventAlert {
	dismissed := toSet(settings.DismissedAlerts)
	snoozed := snoozedUntilByID(settings.SnoozedAlerts)
	alerts := make([]EventAlert, 0, len(events))
	for _, event := range events {
		start, err := time.Parse(time.RFC3339, event.Start)
		if err != nil {
			continue
		}
		end, err := time.Parse(time.RFC3339, event.End)
		if err != nil {
			end = start.Add(time.Hour)
		}
		alertID := "calendar:" + event.ID
		dueAt := start.Add(-time.Duration(settings.AlertLeadMinutes) * time.Minute)
		if dismissed[alertID] || now.After(end) {
			continue
		}
		if snoozedUntil, ok := snoozed[alertID]; ok && snoozedUntil.After(now) {
			dueAt = snoozedUntil
		}
		ringing := !dueAt.After(now) && now.Before(end)
		alerts = append(alerts, EventAlert{
			ID:                alertID,
			Kind:              "calendar-event",
			Label:             event.Title,
			Status:            "active",
			DueAt:             dueAt.UTC().Format(time.RFC3339),
			EventStart:        event.Start,
			EventEnd:          event.End,
			Location:          event.Location,
			Calendar:          event.Calendar,
			Sound:             settings.NotificationSound,
			Ringing:           ringing,
			DefaultSnoozeMins: settings.DefaultSnoozeMins,
			Event:             event,
		})
	}
	return alerts
}

func parseSettings(raw map[string]any) Settings {
	settings := Settings{
		NotificationSound: alertpolicy.DefaultSound,
		Timezone:          "UTC",
		LookaheadDays:     14,
		AlertLeadMinutes:  10,
		DefaultSnoozeMins: alertpolicy.DefaultSnoozeMins,
	}
	if raw == nil {
		return settings
	}
	settings.NotificationSound = soundArg(raw, "notificationSound", settings.NotificationSound)
	settings.Timezone = stringArg(raw, "timezone", settings.Timezone)
	if v := intArg(raw, "lookaheadDays", settings.LookaheadDays); v > 0 {
		settings.LookaheadDays = v
	}
	if v := intArg(raw, "alertLeadMinutes", settings.AlertLeadMinutes); v >= 0 {
		settings.AlertLeadMinutes = v
	}
	if v := intArg(raw, "defaultSnoozeMins", settings.DefaultSnoozeMins); v > 0 {
		settings.DefaultSnoozeMins = v
	}
	settings.DismissedAlerts = stringSlice(raw["dismissedAlerts"])
	settings.SnoozedAlerts = snoozedSlice(raw["snoozedAlerts"])
	return settings
}

func (s Settings) toMap() map[string]any {
	snoozed := make([]any, 0, len(s.SnoozedAlerts))
	for _, item := range s.SnoozedAlerts {
		snoozed = append(snoozed, map[string]any{
			"id":           item.ID,
			"snoozedUntil": item.SnoozedUntil,
		})
	}
	return map[string]any{
		"notificationSound": s.NotificationSound,
		"timezone":          s.Timezone,
		"lookaheadDays":     s.LookaheadDays,
		"alertLeadMinutes":  s.AlertLeadMinutes,
		"defaultSnoozeMins": s.DefaultSnoozeMins,
		"dismissedAlerts":   s.DismissedAlerts,
		"snoozedAlerts":     snoozed,
	}
}

func mutationResult(input widgets.ActionInput, settings Settings, id string) widgets.SettingsMutationResult {
	return widgets.SettingsMutationResult{
		Body: map[string]any{
			"status":           "ok",
			"skillId":          SkillID,
			"widgetInstanceId": input.InstanceID,
			"actionId":         input.ActionID,
			"id":               id,
		},
		Settings: settings.toMap(),
	}
}

func calendarContext(snapshot widgetskills.Snapshot, instanceID string) map[string]any {
	for _, widget := range snapshot.Layout.Widgets {
		if widget.ID != instanceID {
			continue
		}
		if state, ok := widgets.PayloadData(widget.Data).(map[string]any); ok {
			return cleanContext(state)
		}
	}
	return map[string]any{}
}

func cleanContext(state map[string]any) map[string]any {
	cleaned := map[string]any{}
	for _, key := range []string{
		"events",
		"nextEvent",
		"ringing",
		"ringingCount",
		"alertLeadMinutes",
		"notificationSound",
		"generatedAt",
	} {
		if value, ok := state[key]; ok {
			cleaned[key] = value
		}
	}
	return cleaned
}

func init() {
	widgets.RegisterWithSkill(&CalendarWidget{}, calendarContext)
}

func (w *CalendarWidget) currentTime() time.Time {
	if w != nil && w.now != nil {
		return w.now().UTC()
	}
	return time.Now().UTC()
}

func stringSetting(settings map[string]any, key string) string {
	if settings == nil {
		return ""
	}
	value, _ := settings[key].(string)
	return strings.TrimSpace(value)
}

func stringArg(args map[string]any, key string, fallback string) string {
	if args == nil {
		return fallback
	}
	if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func intArg(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return i
		}
	}
	return fallback
}

func soundArg(args map[string]any, key string, fallback string) string {
	return alertpolicy.NormalizeSound(stringArg(args, key, fallback), fallback)
}

func soundSchema() map[string]any {
	return alertpolicy.SoundSchema()
}

func stringSlice(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, item := range items {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func snoozedSlice(raw any) []SnoozedAlert {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []SnoozedAlert{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := stringArg(m, "id", "")
		until := stringArg(m, "snoozedUntil", "")
		if id != "" && until != "" {
			out = append(out, SnoozedAlert{ID: id, SnoozedUntil: until})
		}
	}
	return out
}

func upsertSnoozed(items []SnoozedAlert, next SnoozedAlert) []SnoozedAlert {
	out := make([]SnoozedAlert, 0, len(items)+1)
	replaced := false
	for _, item := range items {
		if item.ID == next.ID {
			out = append(out, next)
			replaced = true
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, next)
	}
	return out
}

func snoozedUntilByID(items []SnoozedAlert) map[string]time.Time {
	out := map[string]time.Time{}
	for _, item := range items {
		until, err := time.Parse(time.RFC3339, item.SnoozedUntil)
		if err == nil {
			out[item.ID] = until
		}
	}
	return out
}

func toSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		out[item] = true
	}
	return out
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
