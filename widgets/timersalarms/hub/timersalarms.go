package timersalarms

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"jute-dash/widgets/alerts"
)

const SkillID = "jute.timers_alarms.control"

type TimersAlarmsWidget struct {
	now func() time.Time
}

type Settings struct {
	NotificationSound string
	DefaultSnoozeMins int
	Timezone          string
	Items             []Item
}

type Item struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Label           string `json:"label"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	DueAt           string `json:"dueAt,omitempty"`
	DurationSeconds int    `json:"durationSeconds,omitempty"`
	Time            string `json:"time,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	Weekdays        []int  `json:"weekdays,omitempty"`
	Sound           string `json:"sound,omitempty"`
	SnoozeCount     int    `json:"snoozeCount,omitempty"`
	LastSnoozedAt   string `json:"lastSnoozedAt,omitempty"`
}

type ItemView struct {
	Item

	Ringing          bool `json:"ringing"`
	RemainingSeconds int  `json:"remainingSeconds"`
	Recurring        bool `json:"recurring"`
}

func (w *TimersAlarmsWidget) Kind() string {
	return "timers-alarms"
}

func (w *TimersAlarmsWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          "timers-alarms",
		Name:          "Timers & Alarms",
		Description:   "Create timers, one-off alarms, recurring alarms, snooze, and configure notification sounds.",
		DefaultTitle:  "Timers",
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
				Default: alerts.DefaultSound,
				Options: alerts.SupportedSounds(),
			},
			{
				ID:      "defaultSnoozeMins",
				Type:    widgets.SettingNumber,
				Label:   "Default snooze minutes",
				Default: alerts.DefaultSnoozeMins,
			},
			{
				ID:      "timezone",
				Type:    widgets.SettingString,
				Label:   "Alarm timezone",
				Default: "UTC",
			},
		},
	}
}

func (w *TimersAlarmsWidget) FetchData(_ context.Context, raw map[string]any) (any, error) {
	settings := parseSettings(raw)
	now := w.currentTime()
	items := itemViews(settings, now)
	ringing := make([]ItemView, 0)
	active := make([]ItemView, 0)
	for _, item := range items {
		switch item.Status {
		case "dismissed", "cancelled":
			continue
		}
		if item.Ringing {
			ringing = append(ringing, item)
		}
		active = append(active, item)
	}
	return widgets.OK(map[string]any{
		"items":             items,
		"active":            active,
		"ringing":           ringing,
		"ringingCount":      len(ringing),
		"notificationSound": settings.NotificationSound,
		"defaultSnoozeMins": settings.DefaultSnoozeMins,
		"timezone":          settings.Timezone,
		"nextRingingItemId": firstRingingID(ringing),
		"generatedAt":       now.Format(time.RFC3339),
		"supportedSounds":   alerts.SupportedSounds(),
		"supportedWeekdays": []int{0, 1, 2, 3, 4, 5, 6},
		"weekdayConvention": "0=Sunday",
	}), nil
}

func (w *TimersAlarmsWidget) Skill() *widgetskills.Definition {
	return timersAlarmsSkill()
}

func (w *TimersAlarmsWidget) InvokeActionWithSettings(
	_ context.Context,
	input widgets.ActionInput,
) (widgets.SettingsMutationResult, error) {
	settings := parseSettings(input.Settings)
	now := w.currentTime()
	var body map[string]any

	switch input.ActionID {
	case "create_timer":
		item, err := createTimer(settings, input.Arguments, now)
		if err != nil {
			return widgets.SettingsMutationResult{}, err
		}
		settings.Items = append(settings.Items, item)
		body = okBody(input, item)
	case "create_alarm":
		item, err := createAlarm(settings, input.Arguments, now)
		if err != nil {
			return widgets.SettingsMutationResult{}, err
		}
		settings.Items = append(settings.Items, item)
		body = okBody(input, item)
	case "snooze":
		item, err := updateItem(settings.Items, input.Arguments, func(item Item) (Item, error) {
			mins := intArg(input.Arguments, "minutes", settings.DefaultSnoozeMins)
			if mins <= 0 {
				mins = settings.DefaultSnoozeMins
			}
			item.Status = "snoozed"
			item.DueAt = now.Add(time.Duration(mins) * time.Minute).Format(time.RFC3339)
			item.SnoozeCount++
			item.LastSnoozedAt = now.Format(time.RFC3339)
			return item, nil
		})
		if err != nil {
			return widgets.SettingsMutationResult{}, err
		}
		settings.Items = item.list
		body = okBody(input, item.item)
	case "dismiss":
		item, err := updateItem(settings.Items, input.Arguments, func(item Item) (Item, error) {
			if item.Kind == "alarm" && len(item.Weekdays) > 0 {
				next, err := nextAlarmDue(item.Time, item.Timezone, item.Weekdays, now.Add(time.Minute))
				if err != nil {
					return Item{}, err
				}
				item.Status = "active"
				item.DueAt = next.Format(time.RFC3339)
			} else {
				item.Status = "dismissed"
			}
			return item, nil
		})
		if err != nil {
			return widgets.SettingsMutationResult{}, err
		}
		settings.Items = item.list
		body = okBody(input, item.item)
	case "cancel":
		item, err := updateItem(settings.Items, input.Arguments, func(item Item) (Item, error) {
			item.Status = "cancelled"
			return item, nil
		})
		if err != nil {
			return widgets.SettingsMutationResult{}, err
		}
		settings.Items = item.list
		body = okBody(input, item.item)
	case "set_notification_sound":
		sound := soundArg(input.Arguments, "sound", settings.NotificationSound)
		settings.NotificationSound = sound
		body = map[string]any{"status": "ok", "actionId": input.ActionID, "notificationSound": sound}
	default:
		return widgets.SettingsMutationResult{}, fmt.Errorf("unsupported action %q", input.ActionID)
	}

	return widgets.SettingsMutationResult{
		Body:     body,
		Settings: settings.toMap(),
	}, nil
}

func timersAlarmsSkill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          "timers-alarms",
		DisplayName:         "Timers & Alarms",
		Summary:             "Create timers and one-off or recurring alarms, read public timer/alarm state, configure sounds, snooze, dismiss, and cancel ringing items.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "always",
		ContextFields: []widgetskills.Field{
			{
				Name:        "active",
				Type:        "array",
				Description: "Active timers and alarms with public labels and due times.",
				Sensitivity: "public",
			},
			{
				Name:        "ringing",
				Type:        "array",
				Description: "Timers or alarms currently due and ringing.",
				Sensitivity: "public",
			},
			{
				Name:        "ringingCount",
				Type:        "integer",
				Description: "Number of currently ringing timers or alarms.",
				Sensitivity: "public",
			},
			{
				Name:        "notificationSound",
				Type:        "enum",
				Description: "Configured notification sound.",
				EnumValues:  alerts.SupportedSounds(),
				Sensitivity: "public",
			},
			{
				Name:        "defaultSnoozeMins",
				Type:        "integer",
				Description: "Default snooze interval in minutes.",
				Sensitivity: "public",
			},
		},
		Actions: []widgetskills.Action{
			timerAction(
				"create_timer",
				"Create timer",
				"Create a countdown timer.",
				map[string]any{
					"label":           map[string]any{"type": "string"},
					"durationSeconds": map[string]any{"type": "integer", "minimum": 1},
					"sound":           soundSchema(),
				},
				[]string{"durationSeconds"},
			),
			timerAction(
				"create_alarm",
				"Create alarm",
				"Create a one-off or recurring alarm. Use weekdays for recurrence, where 0 is Sunday.",
				map[string]any{
					"label": map[string]any{"type": "string"},
					"time": map[string]any{
						"type":        "string",
						"description": "Alarm time in 24-hour HH:MM format.",
					},
					"timezone": map[string]any{"type": "string"},
					"weekdays": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":    "integer",
							"minimum": 0,
							"maximum": 6,
						},
					},
					"sound": soundSchema(),
				},
				[]string{"time"},
			),
			timerAction(
				"snooze",
				"Snooze",
				"Snooze a ringing or active timer/alarm by minutes.",
				map[string]any{
					"id":      map[string]any{"type": "string"},
					"minutes": map[string]any{"type": "integer", "minimum": 1},
				},
				[]string{"id"},
			),
			timerAction(
				"dismiss",
				"Dismiss",
				"Dismiss a ringing timer/alarm. Recurring alarms are scheduled for their next occurrence.",
				map[string]any{"id": map[string]any{"type": "string"}},
				[]string{"id"},
			),
			timerAction(
				"cancel",
				"Cancel",
				"Cancel an active timer or alarm.",
				map[string]any{"id": map[string]any{"type": "string"}},
				[]string{"id"},
			),
			timerAction(
				"set_notification_sound",
				"Set notification sound",
				"Set the widget's default notification sound.",
				map[string]any{"sound": soundSchema()},
				[]string{"sound"},
			),
		},
		Prompts: []widgetskills.Prompt{
			{
				ID:    "timer_alarm_control",
				Title: "Use timer and alarm controls",
				Purpose: "Guide an agent to create explicit timers or alarms, " +
					"ask for missing time details, and use declared actions for " +
					"snooze, dismiss, and sound changes.",
			},
		},
		SupportedWidgetSizes: []string{"medium", "wide", "large"},
	}
}

func timersAlarmsContext(snapshot widgetskills.Snapshot, instanceID string) map[string]any {
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

func init() {
	widgets.RegisterWithSkill(&TimersAlarmsWidget{}, timersAlarmsContext)
}

func (w *TimersAlarmsWidget) currentTime() time.Time {
	if w != nil && w.now != nil {
		return w.now().UTC()
	}
	return time.Now().UTC()
}

type updateResult struct {
	item Item
	list []Item
}

func updateItem(items []Item, args map[string]any, mutate func(Item) (Item, error)) (updateResult, error) {
	id := strings.TrimSpace(stringArg(args, "id", ""))
	if id == "" {
		return updateResult{}, errors.New("id is required")
	}
	next := make([]Item, len(items))
	copy(next, items)
	for i, item := range next {
		if item.ID != id {
			continue
		}
		updated, err := mutate(item)
		if err != nil {
			return updateResult{}, err
		}
		next[i] = updated
		return updateResult{item: updated, list: next}, nil
	}
	return updateResult{}, fmt.Errorf("timer or alarm %q not found", id)
}

func createTimer(settings Settings, args map[string]any, now time.Time) (Item, error) {
	seconds := intArg(args, "durationSeconds", 0)
	if seconds <= 0 {
		seconds = intArg(args, "seconds", 0)
	}
	if seconds <= 0 {
		return Item{}, errors.New("durationSeconds must be positive")
	}
	label := stringArg(args, "label", "Timer")
	return Item{
		ID:              idFor("timer", now),
		Kind:            "timer",
		Label:           label,
		Status:          "active",
		CreatedAt:       now.Format(time.RFC3339),
		DueAt:           now.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339),
		DurationSeconds: seconds,
		Sound:           soundArg(args, "sound", settings.NotificationSound),
	}, nil
}

func createAlarm(settings Settings, args map[string]any, now time.Time) (Item, error) {
	alarmTime := strings.TrimSpace(stringArg(args, "time", ""))
	if _, _, err := parseHHMM(alarmTime); err != nil {
		return Item{}, err
	}
	timezone := strings.TrimSpace(stringArg(args, "timezone", settings.Timezone))
	if timezone == "" {
		timezone = "UTC"
	}
	weekdays := weekdaysArg(args, "weekdays")
	due, err := nextAlarmDue(alarmTime, timezone, weekdays, now)
	if err != nil {
		return Item{}, err
	}
	label := stringArg(args, "label", "Alarm")
	return Item{
		ID:        idFor("alarm", now),
		Kind:      "alarm",
		Label:     label,
		Status:    "active",
		CreatedAt: now.Format(time.RFC3339),
		DueAt:     due.Format(time.RFC3339),
		Time:      alarmTime,
		Timezone:  timezone,
		Weekdays:  weekdays,
		Sound:     soundArg(args, "sound", settings.NotificationSound),
	}, nil
}

func parseSettings(raw map[string]any) Settings {
	settings := Settings{
		NotificationSound: alerts.DefaultSound,
		DefaultSnoozeMins: alerts.DefaultSnoozeMins,
		Timezone:          "UTC",
	}
	if raw == nil {
		return settings
	}
	settings.NotificationSound = soundArg(raw, "notificationSound", settings.NotificationSound)
	if v := intArg(raw, "defaultSnoozeMins", settings.DefaultSnoozeMins); v > 0 {
		settings.DefaultSnoozeMins = v
	}
	if v := strings.TrimSpace(stringArg(raw, "timezone", settings.Timezone)); v != "" {
		settings.Timezone = v
	}
	settings.Items = parseItems(raw["items"])
	return settings
}

func parseItems(raw any) []Item {
	rawItems, ok := raw.([]any)
	if !ok {
		return []Item{}
	}
	items := make([]Item, 0, len(rawItems))
	for _, rawItem := range rawItems {
		m, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		item := Item{
			ID:              stringArg(m, "id", ""),
			Kind:            stringArg(m, "kind", ""),
			Label:           stringArg(m, "label", ""),
			Status:          stringArg(m, "status", "active"),
			CreatedAt:       stringArg(m, "createdAt", ""),
			DueAt:           stringArg(m, "dueAt", ""),
			DurationSeconds: intArg(m, "durationSeconds", 0),
			Time:            stringArg(m, "time", ""),
			Timezone:        stringArg(m, "timezone", ""),
			Weekdays:        weekdaysArg(m, "weekdays"),
			Sound:           soundArg(m, "sound", ""),
			SnoozeCount:     intArg(m, "snoozeCount", 0),
			LastSnoozedAt:   stringArg(m, "lastSnoozedAt", ""),
		}
		if item.ID != "" && (item.Kind == "timer" || item.Kind == "alarm") {
			items = append(items, item)
		}
	}
	return items
}

func (s Settings) toMap() map[string]any {
	items := make([]any, 0, len(s.Items))
	for _, item := range s.Items {
		items = append(items, itemToMap(item))
	}
	return map[string]any{
		"notificationSound": s.NotificationSound,
		"defaultSnoozeMins": s.DefaultSnoozeMins,
		"timezone":          s.Timezone,
		"items":             items,
	}
}

func itemToMap(item Item) map[string]any {
	out := map[string]any{
		"id":              item.ID,
		"kind":            item.Kind,
		"label":           item.Label,
		"status":          item.Status,
		"createdAt":       item.CreatedAt,
		"sound":           item.Sound,
		"snoozeCount":     item.SnoozeCount,
		"durationSeconds": item.DurationSeconds,
	}
	if item.DueAt != "" {
		out["dueAt"] = item.DueAt
	}
	if item.Time != "" {
		out["time"] = item.Time
	}
	if item.Timezone != "" {
		out["timezone"] = item.Timezone
	}
	if len(item.Weekdays) > 0 {
		out["weekdays"] = item.Weekdays
	}
	if item.LastSnoozedAt != "" {
		out["lastSnoozedAt"] = item.LastSnoozedAt
	}
	return out
}

func itemViews(settings Settings, now time.Time) []ItemView {
	views := make([]ItemView, 0, len(settings.Items))
	for _, item := range settings.Items {
		due, _ := time.Parse(time.RFC3339, item.DueAt)
		remaining := int(math.Ceil(due.Sub(now).Seconds()))
		if remaining < 0 {
			remaining = 0
		}
		ringing := (item.Status == "active" || item.Status == "snoozed") && !due.IsZero() && !due.After(now)
		views = append(views, ItemView{
			Item:             item,
			Ringing:          ringing,
			RemainingSeconds: remaining,
			Recurring:        item.Kind == "alarm" && len(item.Weekdays) > 0,
		})
	}
	return views
}

func nextAlarmDue(value string, timezone string, weekdays []int, after time.Time) (time.Time, error) {
	hour, minute, err := parseHHMM(value)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q", timezone)
	}
	localAfter := after.In(loc)
	allowed := map[int]bool{}
	for _, day := range weekdays {
		allowed[day] = true
	}
	for offset := range 8 {
		candidateDay := localAfter.AddDate(0, 0, offset)
		candidate := time.Date(
			candidateDay.Year(),
			candidateDay.Month(),
			candidateDay.Day(),
			hour,
			minute,
			0,
			0,
			loc,
		)
		if len(allowed) > 0 && !allowed[int(candidate.Weekday())] {
			continue
		}
		if candidate.After(localAfter) {
			return candidate.UTC(), nil
		}
	}
	return time.Time{}, errors.New("could not schedule alarm")
}

func parseHHMM(value string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("time must be HH:MM")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, errors.New("time hour must be 00-23")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, errors.New("time minute must be 00-59")
	}
	return hour, minute, nil
}

func timerAction(
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
			"type": "object",
			"properties": map[string]any{
				"status":   map[string]any{"type": "string"},
				"actionId": map[string]any{"type": "string"},
				"item":     map[string]any{"type": "object"},
			},
			"required": []string{"status", "actionId"},
		},
	}
}

func soundSchema() map[string]any {
	return alerts.SoundSchema()
}

func okBody(input widgets.ActionInput, item Item) map[string]any {
	return map[string]any{
		"status":           "ok",
		"skillId":          SkillID,
		"widgetInstanceId": input.InstanceID,
		"actionId":         input.ActionID,
		"item":             itemToMap(item),
	}
}

func cleanContext(state map[string]any) map[string]any {
	cleaned := map[string]any{}
	for _, key := range []string{"active", "ringing", "ringingCount", "notificationSound", "defaultSnoozeMins", "generatedAt"} {
		if value, ok := state[key]; ok {
			cleaned[key] = value
		}
	}
	return cleaned
}

func firstRingingID(items []ItemView) any {
	if len(items) == 0 {
		return nil
	}
	return items[0].ID
}

func idFor(prefix string, now time.Time) string {
	return fmt.Sprintf("%s-%d", prefix, now.UnixNano())
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

func weekdaysArg(args map[string]any, key string) []int {
	if args == nil {
		return nil
	}
	raw, ok := args[key]
	if !ok {
		return nil
	}
	values := []int{}
	seen := map[int]bool{}
	add := func(day int) {
		if day < 0 || day > 6 || seen[day] {
			return
		}
		seen[day] = true
		values = append(values, day)
	}
	switch v := raw.(type) {
	case []int:
		for _, day := range v {
			add(day)
		}
	case []any:
		for _, item := range v {
			switch day := item.(type) {
			case float64:
				add(int(day))
			case int:
				add(day)
			case string:
				if parsed, err := strconv.Atoi(strings.TrimSpace(day)); err == nil {
					add(parsed)
				}
			}
		}
	}
	return values
}

func soundArg(args map[string]any, key string, fallback string) string {
	return alerts.NormalizeSound(stringArg(args, key, fallback), fallback)
}
