package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"jute-dash/internal/a2a"
	"jute-dash/internal/config"
)

func TestInitializeMigratesAndSeedsEmptyDB(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	needsSeed, err := st.NeedsSeed(context.Background())
	if err != nil {
		t.Fatalf("NeedsSeed() error = %v", err)
	}
	if !needsSeed {
		t.Fatal("expected empty store to need seed")
	}

	result, err := st.Initialize(context.Background(), config.Default(), false)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if !result.Seeded {
		t.Fatal("expected empty store to be seeded")
	}
	if result.Setup.Complete {
		t.Fatal("expected setup to be incomplete without bootstrap config")
	}
	if !contains(result.Setup.Missing, "home.name") {
		t.Fatalf("expected home.name missing field, got %+v", result.Setup.Missing)
	}

	assertCount(t, st, "schema_migrations", 1)
	assertCount(t, st, "household_settings", 1)
	assertCount(t, st, "device_profiles", 1)
	assertCount(t, st, "layout_profiles", 1)
	assertCount(t, st, "widget_instances", 3)

	if result.Config.Home.Name != config.Default().Home.Name {
		t.Fatalf("unexpected home name: %q", result.Config.Home.Name)
	}
	if len(result.Config.Agents) != 0 {
		t.Fatalf("production empty-store defaults should not include fake agents: %+v", result.Config.Agents)
	}

	needsSeed, err = st.NeedsSeed(context.Background())
	if err != nil {
		t.Fatalf("NeedsSeed() after initialize error = %v", err)
	}
	if needsSeed {
		t.Fatal("expected initialized store not to need seed")
	}
}

func TestBootstrapConfigAppliesOnlyOnce(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	first := config.Default()
	first.Home.Name = "Bootstrap One"
	first.Home.Timezone = "Europe/London"
	first.Home.Locale = "en-GB"
	first.Agents = []config.AgentConfig{
		{
			ID:              "house",
			Name:            "House Concierge",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     "https://agent.example.com/a2a/v1",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
		},
	}

	result, err := st.Initialize(context.Background(), first, true)
	if err != nil {
		t.Fatalf("Initialize(first) error = %v", err)
	}
	if !result.Setup.Complete {
		t.Fatalf("expected setup complete from bootstrap, got %+v", result.Setup)
	}
	if result.Config.Home.Name != "Bootstrap One" {
		t.Fatalf("unexpected first home name: %q", result.Config.Home.Name)
	}

	second := first
	second.Home.Name = "Bootstrap Two"
	second.Agents = nil

	result, err = st.Initialize(context.Background(), second, true)
	if err != nil {
		t.Fatalf("Initialize(second) error = %v", err)
	}
	if result.Seeded {
		t.Fatal("expected existing store not to be seeded again")
	}
	if result.Config.Home.Name != "Bootstrap One" {
		t.Fatalf("bootstrap config should only apply once, got home name %q", result.Config.Home.Name)
	}
	if len(result.Config.Agents) != 1 || result.Config.Agents[0].ID != "house" {
		t.Fatalf("bootstrap agents should remain from first seed, got %+v", result.Config.Agents)
	}
}

func TestStoreBackedPublicConfigDoesNotExposeSecretReferences(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := config.Default()
	bootstrap.Home.Name = "Secret Test"
	bootstrap.Agents = []config.AgentConfig{
		{
			ID:              "house",
			Name:            "House",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     "https://agent.example.com/a2a/v1",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
			Auth:            &config.AuthConfig{Type: "bearer", EnvToken: "JUTE_SECRET_TOKEN"},
		},
	}

	result, err := st.Initialize(context.Background(), bootstrap, true)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	public := result.Config.Public()
	if len(public.Agents) != 1 || !public.Agents[0].AuthConfigured {
		t.Fatalf("expected public auth configured projection, got %+v", public.Agents)
	}
	body, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("marshal public config: %v", err)
	}
	if strings.Contains(string(body), "JUTE_SECRET_TOKEN") || strings.Contains(string(body), "bearer") {
		t.Fatalf("public config leaked auth details: %s", body)
	}
}

func TestSetupStatusReportsCompleteBootstrap(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := config.Default()
	bootstrap.Home.Name = "Configured Home"
	bootstrap.Home.Timezone = "Europe/London"
	bootstrap.Home.Locale = "en-GB"

	result, err := st.Initialize(context.Background(), bootstrap, true)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if !result.Setup.Complete || len(result.Setup.Missing) != 0 {
		t.Fatalf("unexpected setup status: %+v", result.Setup)
	}
}

func TestWidgetLayoutReturnsSeededWidgets(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	if _, err := st.Initialize(context.Background(), config.Default(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout, err := st.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if layout.ProfileID != defaultLayoutProfileID {
		t.Fatalf("unexpected profile ID: %q", layout.ProfileID)
	}
	if len(layout.Widgets) != 3 {
		t.Fatalf("expected 3 widgets, got %+v", layout.Widgets)
	}
	wantKinds := []string{"date-time", "weather", "chat-history"}
	for i, want := range wantKinds {
		if layout.Widgets[i].Kind != want {
			t.Fatalf("widget %d kind = %q, want %q", i, layout.Widgets[i].Kind, want)
		}
		if !layout.Widgets[i].Visible {
			t.Fatalf("widget %s should be visible", layout.Widgets[i].ID)
		}
		if layout.Widgets[i].Settings == nil {
			t.Fatalf("widget %s settings should be an empty object, not nil", layout.Widgets[i].ID)
		}
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return st
}

func assertCount(t *testing.T, st *Store, table string, want int) {
	t.Helper()
	var got int
	if err := st.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
