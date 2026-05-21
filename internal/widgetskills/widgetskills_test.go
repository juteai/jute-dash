package widgetskills

import (
	"testing"
	"time"

	"jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/store"
	"jute-dash/internal/weather"
)

func TestBuiltInSkillsAreAvailableForVisibleWidgets(t *testing.T) {
	snapshot := testSnapshot()

	skills := Available(snapshot)

	if len(skills) != 3 {
		t.Fatalf("expected three skills, got %+v", skills)
	}
	if skills[0].SkillID != DateTimeSkillID || skills[1].SkillID != WeatherSkillID || skills[2].SkillID != ChatHistorySkillID {
		t.Fatalf("unexpected skill order: %+v", skills)
	}
}

func TestHiddenWidgetsAreOmitted(t *testing.T) {
	snapshot := testSnapshot()
	snapshot.Layout.Widgets[1].Visible = false

	skills := Available(snapshot)

	for _, skill := range skills {
		if skill.SkillID == WeatherSkillID {
			t.Fatalf("hidden weather widget exposed skill: %+v", skills)
		}
	}
}

func TestSkillContextUsesOnlyPublicFields(t *testing.T) {
	snapshot := testSnapshot()

	context, err := SkillContext(snapshot, WeatherSkillID, "")
	if err != nil {
		t.Fatalf("SkillContext() error = %v", err)
	}

	if context.Context["locationName"] != "London" || context.Context["condition"] != "Clear sky" {
		t.Fatalf("unexpected weather context: %+v", context.Context)
	}
	if _, ok := context.Context["auth"]; ok {
		t.Fatalf("context exposed private auth field: %+v", context.Context)
	}
}

func TestVisibleWidgetsExposeSkillMappings(t *testing.T) {
	widgets := VisibleWidgetsSnapshot(testSnapshot())

	if len(widgets.Widgets) != 3 {
		t.Fatalf("expected three visible widgets, got %+v", widgets.Widgets)
	}
	if widgets.Widgets[1].Skill == nil || widgets.Widgets[1].Skill.SkillID != WeatherSkillID {
		t.Fatalf("weather widget did not expose skill mapping: %+v", widgets.Widgets[1])
	}
	if widgets.Widgets[1].ContextURI != "jute://widgets/weather/context" {
		t.Fatalf("unexpected widget context URI: %+v", widgets.Widgets[1])
	}
}

func TestWidgetContextUsesOwningSkill(t *testing.T) {
	context, err := WidgetContext(testSnapshot(), "weather")
	if err != nil {
		t.Fatalf("WidgetContext() error = %v", err)
	}
	if context.Skill.SkillID != WeatherSkillID || context.Context["locationName"] != "London" {
		t.Fatalf("unexpected widget context: %+v", context)
	}
}

func TestUnknownSkillFailsSafely(t *testing.T) {
	_, err := SkillContext(testSnapshot(), "missing", "")
	if err == nil {
		t.Fatal("SkillContext() expected error")
	}
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestInvokeActionReturnsContext(t *testing.T) {
	result, err := InvokeAction(testSnapshot(), WeatherSkillID, "", "refresh", nil)
	if err != nil {
		t.Fatalf("InvokeAction() error = %v", err)
	}
	if result["status"] != "completed" || result["actionId"] != "refresh" {
		t.Fatalf("unexpected action result: %+v", result)
	}
	if result["context"] == nil {
		t.Fatalf("expected context in action result: %+v", result)
	}
}

func testSnapshot() Snapshot {
	cfg := config.Default()
	cfg.Home.Timezone = "Europe/London"
	cfg.Home.Locale = "en-GB"
	cfg.Voice.PreferredAgentID = "house"
	layout := store.DefaultWidgetLayout()
	temp := 18.5
	humidity := 60
	wind := 9.2
	isDay := true
	return Snapshot{
		Config: cfg,
		Layout: layout,
		Weather: weather.State{
			LocationName:    "London",
			Temperature:     &temp,
			TemperatureUnit: "°C",
			Condition:       "Clear sky",
			Humidity:        &humidity,
			WindSpeed:       &wind,
			WindSpeedUnit:   "km/h",
			Sunrise:         "2026-05-19T05:00",
			Sunset:          "2026-05-19T20:55",
			IsDay:           &isDay,
			UpdatedAt:       "2026-05-19T12:00",
			Source:          weather.ProviderOpenMeteo,
			Status:          weather.StatusAvailable,
		},
		Agents: []Agent{
			{
				ID:              "house",
				Name:            "House",
				ProtocolBinding: a2a.ProtocolJSONRPC,
				Enabled:         true,
				Capabilities:    []string{"conversation"},
				AuthConfigured:  true,
			},
		},
		GeneratedAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
	}
}

func TestDynamicRegistryAndDefaultExtractor(t *testing.T) {
	// Register a dynamic widget skill definition
	dynamicSkillID := "custom.dynamic.skill"
	dynamicKind := "dynamic-widget"

	Register(Definition{
		SkillID:     dynamicSkillID,
		WidgetKind:  dynamicKind,
		DisplayName: "Dynamic Widget",
		Summary:     "A custom dynamically registered widget for testing.",
		ContextFields: []Field{
			{Name: "customKey", Type: "string", Description: "A custom settings value."},
			{Name: "missingKey", Type: "string", Description: "A missing key.", Nullable: true},
		},
		Actions: []Action{
			{
				ID:          "dynamic_action",
				Title:       "Dynamic Action",
				Description: "Invokes a dynamic action",
			},
		},
	}, nil) // Passing nil ContextFunc to force fallback to defaultContextExtractor!

	// Create a snapshot with a visible instance of this dynamic widget
	snapshot := testSnapshot()
	snapshot.Layout.Widgets = append(snapshot.Layout.Widgets, store.WidgetInstance{
		ID:      "dynamic_instance_1",
		Kind:    dynamicKind,
		Title:   "My Dynamic Widget",
		Visible: true,
		Settings: map[string]any{
			"customKey": "hello-dynamic",
		},
	})

	// 1. Verify it is available
	skills := Available(snapshot)
	found := false
	for _, s := range skills {
		if s.SkillID == dynamicSkillID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected custom dynamic skill %s to be available", dynamicSkillID)
	}

	// 2. Verify that the context extractor successfully fell back and read Settings
	context, err := SkillContext(snapshot, dynamicSkillID, "dynamic_instance_1")
	if err != nil {
		t.Fatalf("failed to retrieve context: %v", err)
	}
	if context.Context["customKey"] != "hello-dynamic" {
		t.Fatalf("expected customKey value 'hello-dynamic', got %v", context.Context["customKey"])
	}
	val, exists := context.Context["missingKey"]
	if !exists || val != nil {
		t.Fatalf("expected missingKey to exist and be nil, got %v (exists=%t)", val, exists)
	}
}

