package timersalarms

import (
	"context"
	"testing"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

func TestTimersAlarmsCreateTimerPersistsSettings(t *testing.T) {
	now := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	w := &TimersAlarmsWidget{now: func() time.Time { return now }}
	res, err := w.InvokeActionWithSettings(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "timers",
			Settings:   map[string]any{"notificationSound": "bell"},
		},
		ActionID:  "create_timer",
		Arguments: map[string]any{"durationSeconds": float64(120), "label": "Tea"},
	})
	if err != nil {
		t.Fatalf("InvokeActionWithSettings error = %v", err)
	}
	if res.Body["status"] != "ok" {
		t.Fatalf("status = %v, want ok", res.Body["status"])
	}
	items, ok := res.Settings["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("settings items = %#v, want one item", res.Settings["items"])
	}
	item := items[0].(map[string]any)
	if item["kind"] != "timer" || item["label"] != "Tea" || item["sound"] != "bell" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if got, want := item["dueAt"], "2026-06-15T09:02:00Z"; got != want {
		t.Fatalf("dueAt = %v, want %s", got, want)
	}
}

func TestTimersAlarmsSnoozeUpdatesDueAt(t *testing.T) {
	now := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	w := &TimersAlarmsWidget{now: func() time.Time { return now }}
	settings := map[string]any{
		"defaultSnoozeMins": 5,
		"items": []any{map[string]any{
			"id":        "timer-1",
			"kind":      "timer",
			"label":     "Tea",
			"status":    "active",
			"createdAt": "2026-06-14T10:00:00Z",
			"dueAt":     "2026-06-15T08:59:00Z",
		}},
	}
	res, err := w.InvokeActionWithSettings(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{InstanceID: "timers", Settings: settings},
		ActionID:     "snooze",
		Arguments:    map[string]any{"id": "timer-1", "minutes": float64(3)},
	})
	if err != nil {
		t.Fatalf("InvokeActionWithSettings error = %v", err)
	}
	item := res.Settings["items"].([]any)[0].(map[string]any)
	if item["status"] != "snoozed" {
		t.Fatalf("status = %v, want snoozed", item["status"])
	}
	due, err := time.Parse(time.RFC3339, item["dueAt"].(string))
	if err != nil {
		t.Fatalf("parse dueAt: %v", err)
	}
	if got, want := due.Format(time.RFC3339), "2026-06-15T09:03:00Z"; got != want {
		t.Fatalf("dueAt = %s, want %s", got, want)
	}
}

func TestNextAlarmDueUsesRecurringWeekdays(t *testing.T) {
	after := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC) // Sunday
	due, err := nextAlarmDue("07:30", "UTC", []int{1}, after)
	if err != nil {
		t.Fatalf("nextAlarmDue error = %v", err)
	}
	if got, want := due.Format(time.RFC3339), "2026-06-15T07:30:00Z"; got != want {
		t.Fatalf("due = %s, want %s", got, want)
	}
}

func TestRecurringAlarmDismissSchedulesNextOccurrence(t *testing.T) {
	now := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC) // Monday
	w := &TimersAlarmsWidget{now: func() time.Time { return now }}
	settings := map[string]any{
		"items": []any{map[string]any{
			"id":        "alarm-1",
			"kind":      "alarm",
			"label":     "School",
			"status":    "active",
			"createdAt": "2026-06-14T10:00:00Z",
			"dueAt":     "2026-06-15T08:59:00Z",
			"time":      "09:02",
			"timezone":  "UTC",
			"weekdays":  []any{float64(time.Monday)},
		}},
	}
	res, err := w.InvokeActionWithSettings(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{InstanceID: "timers", Settings: settings},
		Snapshot:     widgetskills.Snapshot{},
		ActionID:     "dismiss",
		Arguments:    map[string]any{"id": "alarm-1"},
	})
	if err != nil {
		t.Fatalf("InvokeActionWithSettings error = %v", err)
	}
	item := res.Settings["items"].([]any)[0].(map[string]any)
	if item["status"] != "active" {
		t.Fatalf("status = %v, want active", item["status"])
	}
	if item["dueAt"] == "" {
		t.Fatalf("dueAt not rescheduled: %#v", item)
	}
	if got, want := item["dueAt"], "2026-06-15T09:02:00Z"; got != want {
		t.Fatalf("dueAt = %v, want %s", got, want)
	}
}

func TestTimersAlarmsFetchDataUsesFixedClockForRingingState(t *testing.T) {
	now := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	w := &TimersAlarmsWidget{now: func() time.Time { return now }}

	payload, err := w.FetchData(context.Background(), map[string]any{
		"items": []any{map[string]any{
			"id":        "timer-1",
			"kind":      "timer",
			"label":     "Tea",
			"status":    "active",
			"createdAt": "2026-06-15T08:58:00Z",
			"dueAt":     "2026-06-15T09:00:00Z",
		}},
	})
	if err != nil {
		t.Fatalf("FetchData error = %v", err)
	}
	data := payload.(widgets.RuntimePayload).Data.(map[string]any)
	if got, want := data["ringingCount"], 1; got != want {
		t.Fatalf("ringingCount = %v, want %v", got, want)
	}
	if got, want := data["generatedAt"], "2026-06-15T09:00:00Z"; got != want {
		t.Fatalf("generatedAt = %v, want %s", got, want)
	}
}

func TestTimersAlarmsInvalidSoundFallsBackToDefault(t *testing.T) {
	settings := parseSettings(map[string]any{"notificationSound": "gong"})
	if got, want := settings.NotificationSound, "chime"; got != want {
		t.Fatalf("NotificationSound = %q, want %q", got, want)
	}
}
