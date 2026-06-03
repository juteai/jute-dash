package datetime

import (
	"context"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

const SkillID = "jute.date_time.current"

type DateTimeWidget struct{}

func (w *DateTimeWidget) Kind() string {
	return "date-time"
}

func (w *DateTimeWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          "date-time",
		Name:          "Date & Time",
		Description:   "Clock, date, timezone, and local display timing.",
		DefaultTitle:  "Clock",
		DefaultW:      2,
		DefaultH:      1,
		MinW:          1,
		MinH:          1,
		DefaultSize:   "wide",
		Overflow:      "clip",
		AllowMultiple: false,
	}
}

func (w *DateTimeWidget) FetchData(_ context.Context, _ map[string]any) (any, error) {
	return map[string]any{}, nil
}

func (w *DateTimeWidget) Skill() *widgetskills.Definition {
	return dateTimeSkill()
}

func init() {
	widgets.RegisterWithSkill(&DateTimeWidget{}, dateTimeContext)
}

func dateTimeSkill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          "date-time",
		DisplayName:         "Date & Time",
		Summary:             "Read the configured household date, time, timezone, locale, and display format.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "timezone", Type: "string", Description: "Configured IANA timezone.", Sensitivity: "public"},
			{Name: "locale", Type: "string", Description: "Configured locale.", Sensitivity: "public"},
			{Name: "date", Type: "string", Description: "Localized date.", Sensitivity: "public"},
			{Name: "time", Type: "string", Description: "Localized 24-hour time.", Sensitivity: "public"},
			{Name: "weekday", Type: "string", Description: "Localized weekday.", Sensitivity: "public"},
			{Name: "isoTime", Type: "datetime", Description: "Current time in RFC3339 format.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction(
				"read",
				"Read date and time context",
				"Return the current public date and time context.",
			),
		},
		Prompts: []widgetskills.Prompt{{
			ID:      "date_time_context",
			Title:   "Use date and time context",
			Purpose: "Guide an agent when answering time-sensitive household questions.",
		}},
		SupportedWidgetSizes: []string{"small", "medium", "wide"},
	}
}

func dateTimeContext(snapshot widgetskills.Snapshot, _ string) map[string]any {
	now := snapshot.GeneratedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	location, err := time.LoadLocation(snapshot.Config.Home.Timezone)
	if err != nil {
		location = time.UTC
	}
	local := now.In(location)
	return map[string]any{
		"timezone": snapshot.Config.Home.Timezone,
		"locale":   snapshot.Config.Home.Locale,
		"date":     local.Format("2006-01-02"),
		"time":     local.Format("15:04"),
		"weekday":  local.Weekday().String(),
		"isoTime":  local.Format(time.RFC3339),
	}
}
