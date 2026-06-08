package datetime

import (
	"context"
	"testing"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
)

func TestDateTimeWidget_FetchData(t *testing.T) {
	w := &DateTimeWidget{}
	data, err := w.FetchData(context.Background(), nil)
	if err != nil {
		t.Fatalf("FetchData error: %v", err)
	}
	if data == nil {
		t.Fatalf("expected non-nil data")
	}
}

func TestDateTimeWidget_ParseSettings(t *testing.T) {
	raw := map[string]any{
		"timezone": "Europe/London",
		"locale":   "en-GB",
	}
	s := parseSettings(raw)
	if s.Timezone != "Europe/London" {
		t.Errorf("expected timezone Europe/London, got %q", s.Timezone)
	}
	if s.Locale != "en-GB" {
		t.Errorf("expected locale en-GB, got %q", s.Locale)
	}
}

func TestDateTimeWidget_DateTimeContext(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	snapshot := widgetskills.Snapshot{
		GeneratedAt: now,
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					Kind: "date-time",
					Settings: map[string]any{
						"timezone": "Europe/London",
						"locale":   "en-GB",
					},
				},
			},
		},
	}

	ctx := dateTimeContext(snapshot, "")
	if ctx["timezone"] != "Europe/London" {
		t.Errorf("expected timezone Europe/London, got %v", ctx["timezone"])
	}
	if ctx["locale"] != "en-GB" {
		t.Errorf("expected locale en-GB, got %v", ctx["locale"])
	}
	// Europe/London is UTC+1 in June (BST)
	expectedTime := "13:00"
	if ctx["time"] != expectedTime {
		t.Errorf("expected time %q, got %v", expectedTime, ctx["time"])
	}
}
