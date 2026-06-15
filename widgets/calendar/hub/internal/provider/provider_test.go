package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseICSExpandsRecurringEvents(t *testing.T) {
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	events, err := ParseICS(`BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:standup
DTSTAMP:20260601T090000Z
SUMMARY:Team standup
DTSTART:20260615T090000Z
DTEND:20260615T091500Z
RRULE:FREQ=DAILY;COUNT=3
EXDATE:20260616T090000Z
LOCATION:Kitchen
END:VEVENT
END:VCALENDAR`, Settings{CalendarName: "Home", LookaheadDays: 7}, now)
	if err != nil {
		t.Fatalf("ParseICS error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2: %#v", len(events), events)
	}
	if got, want := events[0].Title, "Team standup"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	if got, want := events[0].Start, "2026-06-15T09:00:00Z"; got != want {
		t.Fatalf("first start = %s, want %s", got, want)
	}
	if got, want := events[1].Start, "2026-06-17T09:00:00Z"; got != want {
		t.Fatalf("second start = %s, want %s", got, want)
	}
}

func TestParseICSHandlesAllDayEvents(t *testing.T) {
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	events, err := ParseICS(`BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:holiday
DTSTAMP:20260601T090000Z
SUMMARY:Inset day
DTSTART;VALUE=DATE:20260616
DTEND;VALUE=DATE:20260617
END:VEVENT
END:VCALENDAR`, Settings{CalendarName: "School", Timezone: "Europe/London"}, now)
	if err != nil {
		t.Fatalf("ParseICS error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if !events[0].AllDay {
		t.Fatalf("AllDay = false, want true")
	}
	if got, want := events[0].Calendar, "School"; got != want {
		t.Fatalf("calendar = %q, want %q", got, want)
	}
}

func TestClientFetchUsesBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = w.Write([]byte(`BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:one
DTSTAMP:20260601T090000Z
SUMMARY:Breakfast
DTSTART:20260615T090000Z
DTEND:20260615T093000Z
END:VEVENT
END:VCALENDAR`))
	}))
	defer server.Close()

	events, err := (Client{HTTPClient: server.Client()}).Fetch(context.Background(), Settings{
		FeedURL:       server.URL,
		Username:      "user",
		Password:      "secret",
		CalendarName:  "Private",
		LookaheadDays: 1,
	}, time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if len(events) != 1 || events[0].Title != "Breakfast" {
		t.Fatalf("events = %#v, want Breakfast", events)
	}
}
