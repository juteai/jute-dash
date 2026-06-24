package service

import "testing"

func TestProjectFiltersVisibleWidgetsAndKeepsHeadlessContext(t *testing.T) {
	layout := WidgetLayout{
		ProfileID:    "wall",
		ActiveScreen: "main",
		Widgets: []WidgetInstance{
			{
				ID:       "clock",
				Kind:     "date-time",
				Visible:  true,
				Settings: map[string]any{"timezone": "Europe/London", "locale": "en-GB"},
			},
			{ID: "hidden", Kind: "weather", Visible: false},
			{ID: "headless", Kind: "rss", Visible: true, Mode: WidgetModeHeadless},
		},
		Screens: []DashboardScreen{{
			ID:    "main",
			Label: "Main",
			Widgets: []WidgetInstance{
				{ID: "clock", Visible: true},
				{ID: "headless", Visible: true, Mode: WidgetModeHeadless},
			},
		}},
	}

	snapshot := Project(t.Context(), layout)

	if snapshot.Display.Profile != "wall" ||
		snapshot.Display.Timezone != "Europe/London" ||
		snapshot.Display.Locale != "en-GB" {
		t.Fatalf("unexpected display info: %+v", snapshot.Display)
	}
	if len(snapshot.Dashboard.VisibleWidgetIDs) != 1 ||
		snapshot.Dashboard.VisibleWidgetIDs[0] != "clock" {
		t.Fatalf("unexpected visible widgets: %+v", snapshot.Dashboard.VisibleWidgetIDs)
	}
	if len(snapshot.Widgets) != 2 {
		t.Fatalf("expected visible and headless widgets only, got %+v", snapshot.Widgets)
	}
	if len(snapshot.Dashboard.Screens) != 1 ||
		!snapshot.Dashboard.Screens[0].Active ||
		len(snapshot.Dashboard.Screens[0].VisibleWidgetIDs) != 1 {
		t.Fatalf("unexpected screens: %+v", snapshot.Dashboard.Screens)
	}
}

func TestWidgetSkillsLayoutPreservesWidgetData(t *testing.T) {
	layout := WidgetLayout{
		ProfileID: "profile",
		Widgets: []WidgetInstance{{
			ID:             "weather-1",
			Kind:           "weather",
			Visible:        true,
			Data:           map[string]any{"temp": 12},
			ConnectionRefs: map[string]string{"account": "weather"},
		}},
	}

	projected := WidgetSkillsLayout(layout)

	if projected.ProfileID != "profile" ||
		projected.Widgets[0].Data == nil ||
		projected.Widgets[0].ConnectionRefs["account"] != "weather" {
		t.Fatalf("unexpected widget skills layout: %+v", projected)
	}
}
