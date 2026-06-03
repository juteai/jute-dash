package widgetskills

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/store"
)

const (
	testSkillID = "test.widget.current"
	testKind    = "test-widget"
)

func TestRegisteredSkillsAreAvailableForVisibleWidgets(t *testing.T) {
	registerTestSkill()
	snapshot := testSnapshot()

	skills := Available(snapshot)

	if len(skills) != 1 {
		t.Fatalf("expected one skill, got %+v", skills)
	}
	if skills[0].SkillID != testSkillID || skills[0].WidgetInstanceID != "test-widget" {
		t.Fatalf("unexpected skill: %+v", skills[0])
	}
}

func TestHiddenWidgetsAreOmitted(t *testing.T) {
	registerTestSkill()
	snapshot := testSnapshot()
	snapshot.Layout.Widgets[0].Visible = false

	skills := Available(snapshot)

	if len(skills) != 0 {
		t.Fatalf("hidden widget exposed skills: %+v", skills)
	}
}

func TestSkillContextUsesRegisteredContext(t *testing.T) {
	registerTestSkill()

	context, err := SkillContext(testSnapshot(), testSkillID, "")
	if err != nil {
		t.Fatalf("SkillContext() error = %v", err)
	}

	if context.Context["publicValue"] != "visible" {
		t.Fatalf("unexpected context: %+v", context.Context)
	}
	if _, ok := context.Context["auth"]; ok {
		t.Fatalf("context exposed private auth field: %+v", context.Context)
	}
}

func TestVisibleWidgetsExposeSkillMappings(t *testing.T) {
	registerTestSkill()
	widgets := VisibleWidgetsSnapshot(testSnapshot())

	if len(widgets.Widgets) != 1 {
		t.Fatalf("expected one visible widget, got %+v", widgets.Widgets)
	}
	if widgets.Widgets[0].Skill == nil || widgets.Widgets[0].Skill.SkillID != testSkillID {
		t.Fatalf("widget did not expose skill mapping: %+v", widgets.Widgets[0])
	}
	if widgets.Widgets[0].ContextURI != "jute://widgets/test-widget/context" {
		t.Fatalf("unexpected widget context URI: %+v", widgets.Widgets[0])
	}
}

func TestWidgetContextUsesOwningSkill(t *testing.T) {
	registerTestSkill()
	context, err := WidgetContext(testSnapshot(), "test-widget")
	if err != nil {
		t.Fatalf("WidgetContext() error = %v", err)
	}
	if context.Skill.SkillID != testSkillID || context.Context["publicValue"] != "visible" {
		t.Fatalf("unexpected widget context: %+v", context)
	}
}

func TestUnknownSkillFailsSafely(t *testing.T) {
	_, err := SkillContext(testSnapshot(), "missing", "")
	if err == nil {
		t.Fatal("SkillContext() expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestInvokeActionReturnsContext(t *testing.T) {
	registerTestSkill()
	result, err := InvokeAction(testSnapshot(), testSkillID, "", "read", nil)
	if err != nil {
		t.Fatalf("InvokeAction() error = %v", err)
	}
	if result["status"] != "completed" || result["actionId"] != "read" {
		t.Fatalf("unexpected action result: %+v", result)
	}
	if result["context"] == nil {
		t.Fatalf("expected context in action result: %+v", result)
	}
}

func TestDefaultExtractorReadsDeclaredSettingsOnly(t *testing.T) {
	dynamicSkillID := "test.dynamic.current"
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
		Actions: []Action{ReadAction("dynamic_action", "Dynamic Action", "Invokes a dynamic action")},
	}, nil)

	snapshot := testSnapshot()
	snapshot.Layout.Widgets = append(snapshot.Layout.Widgets, store.WidgetInstance{
		ID:      "dynamic-instance",
		Kind:    dynamicKind,
		Title:   "My Dynamic Widget",
		Visible: true,
		Settings: map[string]any{
			"customKey": "hello-dynamic",
			"auth":      "must-not-leak",
		},
	})

	context, err := SkillContext(snapshot, dynamicSkillID, "dynamic-instance")
	if err != nil {
		t.Fatalf("failed to retrieve context: %v", err)
	}
	if context.Context["customKey"] != "hello-dynamic" {
		t.Fatalf("expected customKey value, got %v", context.Context["customKey"])
	}
	if _, exists := context.Context["auth"]; exists {
		t.Fatalf("default extractor leaked undeclared setting: %+v", context.Context)
	}
	val, exists := context.Context["missingKey"]
	if !exists || val != nil {
		t.Fatalf("expected missingKey to exist and be nil, got %v (exists=%t)", val, exists)
	}
}

func TestWidgetSkillsDoesNotHardCodeBuiltInWidgets(t *testing.T) {
	source, err := os.ReadFile("widgetskills.go")
	if err != nil {
		t.Fatalf("read widgetskills.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		`"jute-dash/internal/weather"`,
		"DateTimeSkillID",
		"WeatherSkillID",
		"ChatHistorySkillID",
		`WidgetKind:          "weather"`,
		`WidgetKind:          "date-time"`,
		`WidgetKind:          "chat-history"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("widgetskills.go should stay generic; found forbidden reference %q", forbidden)
		}
	}
}

func registerTestSkill() {
	Register(Definition{
		SkillID:             testSkillID,
		WidgetKind:          testKind,
		DisplayName:         "Test Widget",
		Summary:             "A generic test widget skill.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []Field{
			{Name: "publicValue", Type: "string", Description: "Public value.", Sensitivity: "public"},
		},
		Actions: []Action{ReadAction("read", "Read context", "Return public context.")},
		Prompts: []Prompt{{
			ID:      "test_prompt",
			Title:   "Use test context",
			Purpose: "Guide a test agent.",
		}},
	}, func(snapshot Snapshot, instanceID string) map[string]any {
		return map[string]any{"publicValue": "visible"}
	})
}

func testSnapshot() Snapshot {
	cfg := config.Default()
	cfg.Home.Timezone = "Europe/London"
	cfg.Home.Locale = "en-GB"
	cfg.Voice.PreferredAgentID = "house"
	return Snapshot{
		Config: cfg,
		Layout: store.WidgetLayout{
			ProfileID: "default-display",
			Widgets: []store.WidgetInstance{{
				ID:       "test-widget",
				Kind:     testKind,
				Title:    "Test Widget",
				Size:     "medium",
				Visible:  true,
				Settings: map[string]any{"auth": "must-not-leak"},
			}},
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
