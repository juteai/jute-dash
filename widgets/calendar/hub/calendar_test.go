package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jute-dash/widgets"
	"jute-dash/widgets/calendar/hub/internal/provider"
)

func TestCalendarFetchDataWithConnectionsLoadsEvents(t *testing.T) {
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = w.Write([]byte(`BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:dinner
DTSTAMP:20260601T090000Z
SUMMARY:Dinner
DTSTART:20260615T090000Z
DTEND:20260615T100000Z
LOCATION:Kitchen
END:VEVENT
END:VCALENDAR`))
	}))
	defer server.Close()

	widget := &CalendarWidget{now: func() time.Time { return now }}
	payload, err := widget.FetchDataWithConnections(context.Background(), widgets.RuntimeInput{
		InstanceID: "calendar",
		Settings:   map[string]any{"lookaheadDays": float64(2), "alertLeadMinutes": float64(10)},
		Connections: map[string]widgets.ResolvedConnection{
			"account": {
				ID:   "conn",
				Name: "House",
				Settings: map[string]any{
					"feed_url":      server.URL,
					"calendar_name": "House calendar",
				},
				Secrets: map[string]string{},
			},
		},
	})
	if err != nil {
		t.Fatalf("FetchDataWithConnections error = %v", err)
	}
	data, ok := payload.Data.(map[string]any)
	if !ok {
		t.Fatalf("payload data = %#v, want map", payload.Data)
	}
	events, ok := data["events"].([]provider.Event)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v, want one event", data["events"])
	}
	if events[0].Title != "Dinner" || events[0].Calendar != "House calendar" {
		t.Fatalf("event = %#v, want Dinner on House calendar", events[0])
	}
}

func TestCalendarSnoozeEventPersistsSettings(t *testing.T) {
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	widget := &CalendarWidget{now: func() time.Time { return now }}
	res, err := widget.InvokeActionWithSettings(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{
			InstanceID: "calendar",
			Settings:   map[string]any{"defaultSnoozeMins": float64(5)},
		},
		ActionID:  "snooze_event",
		Arguments: map[string]any{"id": "calendar:event-1", "minutes": float64(3)},
	})
	if err != nil {
		t.Fatalf("InvokeActionWithSettings error = %v", err)
	}
	items, ok := res.Settings["snoozedAlerts"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("snoozedAlerts = %#v, want one item", res.Settings["snoozedAlerts"])
	}
	item := items[0].(map[string]any)
	if item["id"] != "calendar:event-1" {
		t.Fatalf("id = %v, want calendar:event-1", item["id"])
	}
	until, err := time.Parse(time.RFC3339, item["snoozedUntil"].(string))
	if err != nil {
		t.Fatalf("parse snoozedUntil: %v", err)
	}
	if got, want := until.Format(time.RFC3339), "2026-06-15T08:03:00Z"; got != want {
		t.Fatalf("snoozedUntil = %s, want %s", got, want)
	}
}

func TestCalendarDismissEventPersistsSettings(t *testing.T) {
	widget := &CalendarWidget{}
	res, err := widget.InvokeActionWithSettings(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{InstanceID: "calendar"},
		ActionID:     "dismiss_event",
		Arguments:    map[string]any{"id": "calendar:event-1"},
	})
	if err != nil {
		t.Fatalf("InvokeActionWithSettings error = %v", err)
	}
	items, ok := res.Settings["dismissedAlerts"].([]string)
	if !ok || len(items) != 1 || items[0] != "calendar:event-1" {
		t.Fatalf("dismissedAlerts = %#v, want calendar:event-1", res.Settings["dismissedAlerts"])
	}
}

func TestCalendarSetNotificationSoundPersistsSettings(t *testing.T) {
	widget := &CalendarWidget{}
	res, err := widget.InvokeActionWithSettings(context.Background(), widgets.ActionInput{
		RuntimeInput: widgets.RuntimeInput{InstanceID: "calendar"},
		ActionID:     "set_event_notification_sound",
		Arguments:    map[string]any{"sound": "bell"},
	})
	if err != nil {
		t.Fatalf("InvokeActionWithSettings error = %v", err)
	}
	if got, want := res.Settings["notificationSound"], "bell"; got != want {
		t.Fatalf("notificationSound = %v, want %s", got, want)
	}
}

func TestCalendarEventAlertsUseFixedClockForDueSnoozeDismissAndExpiry(t *testing.T) {
	now := time.Date(2026, 6, 15, 8, 55, 0, 0, time.UTC)
	events := []provider.Event{
		{
			ID:       "due",
			Title:    "Standup",
			Calendar: "House",
			Start:    "2026-06-15T09:00:00Z",
			End:      "2026-06-15T09:30:00Z",
		},
		{
			ID:       "snoozed",
			Title:    "Workshop",
			Calendar: "House",
			Start:    "2026-06-15T09:00:00Z",
			End:      "2026-06-15T09:30:00Z",
		},
		{
			ID:       "dismissed",
			Title:    "Focus",
			Calendar: "House",
			Start:    "2026-06-15T09:00:00Z",
			End:      "2026-06-15T09:30:00Z",
		},
		{
			ID:       "expired",
			Title:    "Breakfast",
			Calendar: "House",
			Start:    "2026-06-15T07:00:00Z",
			End:      "2026-06-15T07:30:00Z",
		},
	}

	alerts := eventAlerts(events, Settings{
		NotificationSound: "soft",
		AlertLeadMinutes:  10,
		DefaultSnoozeMins: 5,
		DismissedAlerts:   []string{"calendar:dismissed"},
		SnoozedAlerts: []SnoozedAlert{
			{ID: "calendar:snoozed", SnoozedUntil: "2026-06-15T09:05:00Z"},
		},
	}, now)

	if len(alerts) != 2 {
		t.Fatalf("alerts len = %d, want 2: %#v", len(alerts), alerts)
	}
	if !alerts[0].Ringing || alerts[0].Sound != "soft" {
		t.Fatalf("due alert = %#v, want ringing soft alert", alerts[0])
	}
	if alerts[1].Ringing || alerts[1].DueAt != "2026-06-15T09:05:00Z" {
		t.Fatalf("snoozed alert = %#v, want quiet until 09:05", alerts[1])
	}
}

func TestCalendarDataFiltersExpiredRingingState(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	data := calendarData([]provider.Event{
		{
			ID:       "expired",
			Title:    "Breakfast",
			Calendar: "House",
			Start:    "2026-06-15T07:00:00Z",
			End:      "2026-06-15T07:30:00Z",
		},
	}, Settings{NotificationSound: "chime", AlertLeadMinutes: 10, DefaultSnoozeMins: 9}, now)

	if got, want := data["ringingCount"], 0; got != want {
		t.Fatalf("ringingCount = %v, want %v", got, want)
	}
	if alerts, ok := data["alerts"].([]EventAlert); !ok || len(alerts) != 0 {
		t.Fatalf("alerts = %#v, want none", data["alerts"])
	}
}

func TestCalendarInvalidSoundFallsBackToDefault(t *testing.T) {
	settings := parseSettings(map[string]any{"notificationSound": "gong"})
	if got, want := settings.NotificationSound, "chime"; got != want {
		t.Fatalf("NotificationSound = %q, want %q", got, want)
	}
}

func TestCalendarCatalogDoesNotExposeSyncMode(t *testing.T) {
	widget := &CalendarWidget{}
	requirements := widget.RequiredConnections()
	if len(requirements) != 1 {
		t.Fatalf("requirements len = %d, want 1", len(requirements))
	}
	for _, field := range requirements[0].Fields {
		if field.ID == "sync_mode" {
			t.Fatalf("RequiredConnections exposed sync_mode field: %#v", field)
		}
	}
}
