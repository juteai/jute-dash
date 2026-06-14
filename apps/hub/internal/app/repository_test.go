package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/pkg/a2a"
)

func TestInitializeMigratesAndSeedsEmptyDB(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	needsSeedVal := needsSeed(t, st)
	if !needsSeedVal {
		t.Fatal("expected empty store to need seed")
	}

	result, err := st.Initialize(context.Background(), DefaultConfig(), false)
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

	assertCount(t, st, "household_settings", 1)
	assertCount(t, st, "device_profiles", 1)
	assertCount(t, st, "layout_profiles", 1)
	assertCount(t, st, "widget_instances", 4)
	assertCount(t, st, "voice_settings", 1)

	cfg := result.Config.(config.Config)
	if cfg.Home.Name != DefaultConfig().Home.Name {
		t.Fatalf("unexpected home name: %q", cfg.Home.Name)
	}
	if !cfg.Voice.MutedByDefault || cfg.Voice.FollowupWindowSeconds != 8 {
		t.Fatalf("unexpected voice defaults: %+v", cfg.Voice)
	}
	if len(cfg.Agents) != 0 {
		t.Fatalf("production empty-store defaults should not include fake agents: %+v", cfg.Agents)
	}

	needsSeedVal = needsSeed(t, st)
	if needsSeedVal {
		t.Fatal("expected initialized store not to need seed")
	}
}

func TestBootstrapConfigAppliesOnlyOnce(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	first := DefaultConfig()
	first.Home.Name = "Bootstrap One"
	first.Agents = []AgentConfig{
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
	cfg1 := result.Config.(config.Config)
	if cfg1.Home.Name != "Bootstrap One" {
		t.Fatalf("unexpected first home name: %q", cfg1.Home.Name)
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
	cfg2 := result.Config.(config.Config)
	if cfg2.Home.Name != "Bootstrap One" {
		t.Fatalf("bootstrap config should only apply once, got home name %q", cfg2.Home.Name)
	}
	if len(cfg2.Agents) != 0 {
		t.Fatalf("store runtime config should not own agents, got %+v", cfg2.Agents)
	}
}

func TestBootstrapDashboardWidgetsSeedRuntimeStore(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := DefaultConfig()
	bootstrap.Home.Name = "Bootstrap House"
	bootstrap.Dashboard.Widgets = []DashboardWidgetConfig{
		{
			ID:      "custom-clock",
			Type:    "date-time",
			Title:   "Kitchen Clock",
			X:       0,
			Y:       0,
			W:       9,
			H:       2,
			MinW:    3,
			MinH:    1,
			Size:    "large",
			Visible: true,
		},
	}

	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout, err := st.DashboardRepo.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if len(layout.Widgets) != 1 {
		t.Fatalf("expected one bootstrapped widget, got %+v", layout.Widgets)
	}
	widget := layout.Widgets[0]
	if widget.ID != "custom-clock" || widget.W != 9 || widget.H != 2 || widget.Size != "large" {
		t.Fatalf("bootstrap dashboard widget was not seeded: %+v", widget)
	}
}

func TestVoiceSettingsSeededFromBootstrap(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := DefaultConfig()
	bootstrap.Voice.Enabled = true
	bootstrap.Voice.MutedByDefault = false
	bootstrap.Voice.STTProviderID = "wyoming-local"
	bootstrap.Voice.PreferredAgentID = "house"
	bootstrap.Voice.FollowupWindowSeconds = 10

	result, err := st.Initialize(context.Background(), bootstrap, true)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	cfg := result.Config.(config.Config)
	if !cfg.Voice.Enabled || cfg.Voice.MutedByDefault ||
		cfg.Voice.STTProviderID != "wyoming-local" {
		t.Fatalf("unexpected config voice settings: %+v", cfg.Voice)
	}

	settings, err := st.VoiceRepo.VoiceSettings(context.Background(), "")
	if err != nil {
		t.Fatalf("VoiceSettings() error = %v", err)
	}
	if !settings.Enabled || settings.Muted || settings.STTProviderID != "wyoming-local" ||
		settings.PreferredAgentID != "house" ||
		settings.FollowupWindowSeconds != 10 {
		t.Fatalf("unexpected voice settings: %+v", settings)
	}
}

func TestVoiceMuteAndCancelUpdateState(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := DefaultConfig()
	bootstrap.Voice.Enabled = true
	bootstrap.Voice.MutedByDefault = false
	bootstrap.Voice.STTProviderID = "wyoming-local"
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	muted, err := st.VoiceRepo.SetVoiceMuted(context.Background(), "", true)
	if err != nil {
		t.Fatalf("SetVoiceMuted(true) error = %v", err)
	}
	if !muted.Muted || muted.UpdatedAt == "" {
		t.Fatalf("unexpected muted settings: %+v", muted)
	}

	unmuted, err := st.VoiceRepo.SetVoiceMuted(context.Background(), "", false)
	if err != nil {
		t.Fatalf("SetVoiceMuted(false) error = %v", err)
	}
	if unmuted.Muted {
		t.Fatalf("unexpected unmuted settings: %+v", unmuted)
	}

	cancelled, err := st.VoiceRepo.CancelVoice(context.Background(), "")
	if err != nil {
		t.Fatalf("CancelVoice() error = %v", err)
	}
	if cancelled.Muted || cancelled.STTProviderID != "wyoming-local" {
		t.Fatalf("cancel should preserve durable voice settings: %+v", cancelled)
	}
}

func TestVoiceProvidersDefaultsToEmptyList(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	bootstrap := DefaultConfig()
	bootstrap.Agents = []AgentConfig{{
		ID:              "dev-agent",
		Name:            "Dev Agent",
		CardURL:         "http://127.0.0.1:9797/.well-known/agent-card.json",
		EndpointURL:     "http://127.0.0.1:9797/invoke",
		ProtocolBinding: a2a.ProtocolJSONRPC,
		Enabled:         true,
	}}
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	providers, err := st.VoiceRepo.VoiceProviders(context.Background())
	if err != nil {
		t.Fatalf("VoiceProviders() error = %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected no voice providers, got %+v", providers)
	}
}

func TestYAMLBootstrapConfigAppliesOnlyOnce(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	firstPath := writeStoreYAMLConfig(t, `
home:
  name: YAML Bootstrap One
agents:
  - id: yaml-house
    name: YAML House
    card-url: https://agent.example.com/.well-known/agent-card.json
    endpoint-url: https://agent.example.com/a2a/v1
    protocol-binding: JSONRPC
    enabled: true
rooms: []
tiles: []
`)
	first, err := LoadConfig(firstPath)
	if err != nil {
		t.Fatalf("Load(first YAML) error = %v", err)
	}
	result, err := st.Initialize(context.Background(), first, true)
	if err != nil {
		t.Fatalf("Initialize(first) error = %v", err)
	}
	cfg1 := result.Config.(config.Config)
	if cfg1.Home.Name != "YAML Bootstrap One" {
		t.Fatalf("unexpected first home name: %q", cfg1.Home.Name)
	}

	secondPath := writeStoreYAMLConfig(t, `
home:
  name: YAML Bootstrap Two
agents: []
rooms: []
tiles: []
`)
	second, err := LoadConfig(secondPath)
	if err != nil {
		t.Fatalf("Load(second YAML) error = %v", err)
	}
	result, err = st.Initialize(context.Background(), second, true)
	if err != nil {
		t.Fatalf("Initialize(second) error = %v", err)
	}
	if result.Seeded {
		t.Fatal("expected existing store not to be seeded again")
	}
	cfg2 := result.Config.(config.Config)
	if cfg2.Home.Name != "YAML Bootstrap One" {
		t.Fatalf("YAML bootstrap should only apply once, got home name %q", cfg2.Home.Name)
	}
	if len(cfg2.Agents) != 0 {
		t.Fatalf("store runtime config should not own YAML agents, got %+v", cfg2.Agents)
	}
}

func TestDisplayCustomizationSeededFromBootstrap(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := DefaultConfig()
	bootstrap.Display.ColorMode = "dark"
	bootstrap.Display.Theme = "dark"
	bootstrap.Display.ThemeID = "jute-mono"
	bootstrap.Display.Density = "large-touch"
	bootstrap.Display.Motion = "reduced"
	bootstrap.Display.Background = DisplayBackground{
		Kind:     "asset",
		Value:    "/backgrounds/kitchen.jpg",
		Fit:      "cover",
		Position: "center",
		Overlay:  "smoked",
	}
	bootstrap.Display.WidgetChrome = DisplayWidgetChrome{Default: "frosted"}

	result, err := st.Initialize(context.Background(), bootstrap, true)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	cfg := result.Config.(config.Config)
	if cfg.Display.ColorMode != "dark" || cfg.Display.Theme != "dark" ||
		cfg.Display.Density != "large-touch" {
		t.Fatalf("unexpected display settings: %+v", cfg.Display)
	}
	if cfg.Display.Background.Value != "/backgrounds/kitchen.jpg" ||
		cfg.Display.Background.Overlay != "smoked" {
		t.Fatalf("unexpected background settings: %+v", cfg.Display.Background)
	}
	if cfg.Display.WidgetChrome.Default != "frosted" {
		t.Fatalf("unexpected widget chrome: %+v", cfg.Display.WidgetChrome)
	}

	settings, err := st.HomestateRepo.HouseholdSettings(context.Background())
	if err != nil {
		t.Fatalf("HouseholdSettings() error = %v", err)
	}
	displayCfg := settings.Display.(homestate.DisplaySettings)
	if displayCfg.WidgetChrome["default"] != "frosted" {
		t.Fatalf("household settings did not include widget chrome: %+v", settings.Display)
	}
}

func TestStoreBackedPublicConfigDoesNotExposeSecretReferences(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := DefaultConfig()
	bootstrap.Home.Name = "Secret Test"
	bootstrap.Agents = []AgentConfig{
		{
			ID:              "house",
			Name:            "House",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     "https://agent.example.com/a2a/v1",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
			Auth:            &AuthConfig{Type: "bearer", EnvToken: "JUTE_SECRET_TOKEN"},
		},
	}

	_, err := st.Initialize(context.Background(), bootstrap, true)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	public := bootstrap.Public()
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

	bootstrap := DefaultConfig()
	bootstrap.Home.Name = "Configured Home"

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

	if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout, err := st.DashboardRepo.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if layout.ProfileID != defaultLayoutProfileID {
		t.Fatalf("unexpected profile ID: %q", layout.ProfileID)
	}
	if len(layout.Widgets) != 4 {
		t.Fatalf("expected 4 widgets, got %+v", layout.Widgets)
	}
	wantKinds := []string{"date-time", "weather", "chat-history", "spotify"}
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

func TestWidgetCatalogReturnsBuiltIns(t *testing.T) {
	catalog := WidgetCatalog()
	if len(catalog) != 3 {
		t.Fatalf("expected 3 built-in widgets, got %+v", catalog)
	}
	want := []string{"date-time", "weather", "chat-history"}
	for i, kind := range want {
		if catalog[i].Kind != kind {
			t.Fatalf("catalog item %d kind = %q, want %q", i, catalog[i].Kind, kind)
		}
		if catalog[i].AllowMultiple {
			t.Fatalf("%s should be single-instance in v1", kind)
		}
	}
}

func TestSaveWidgetLayoutPersists(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout := DefaultWidgetLayout()
	layout.Widgets[0].X = 1
	layout.Widgets[0].W = 3
	layout.Widgets[1].Visible = false

	saved, err := st.DashboardRepo.SaveWidgetLayout(context.Background(), layout)
	if err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}
	if saved.Widgets[0].X != 1 || saved.Widgets[0].W != 3 || saved.Widgets[1].Visible {
		t.Fatalf("unexpected saved layout: %+v", saved.Widgets)
	}

	reloaded, err := st.DashboardRepo.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if reloaded.Widgets[0].X != 1 || reloaded.Widgets[0].W != 3 || reloaded.Widgets[1].Visible {
		t.Fatalf("layout did not persist: %+v", reloaded.Widgets)
	}
}

func TestSaveWidgetLayoutPersistsVariants(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout := DefaultWidgetLayout()
	layout.Variants[0].Columns = 2
	layout.Variants[0].Rows = 12
	layout.Variants[0].Placements[layout.Widgets[0].ID] = dashboard.WidgetPlacement{
		X: 0,
		Y: 1,
		W: 2,
		H: 1,
	}

	if _, err := st.DashboardRepo.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}

	reloaded, err := st.DashboardRepo.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if reloaded.SchemaVersion != dashboard.LayoutSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", reloaded.SchemaVersion, dashboard.LayoutSchemaVersion)
	}
	if len(reloaded.Variants) == 0 || reloaded.Variants[0].Columns != 2 || reloaded.Variants[0].Rows != 12 {
		t.Fatalf("layout variants did not persist: %+v", reloaded.Variants)
	}
	if got := reloaded.Variants[0].Placements[layout.Widgets[0].ID]; got.Y != 1 || got.W != 2 {
		t.Fatalf("variant placement did not persist: %+v", got)
	}
}

func TestConfigExportsWidgetLayoutVariants(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout := DefaultWidgetLayout()
	layout.DefaultVariant = "desktop"
	layout.Variants[3].Columns = 14
	layout.Variants[3].Rows = 7
	layout.Variants[3].Placements[layout.Widgets[0].ID] = dashboard.WidgetPlacement{
		X: 2,
		Y: 1,
		W: 4,
		H: 1,
	}
	if _, err := st.DashboardRepo.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}

	cfg, err := st.Config(context.Background())
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if cfg.Dashboard.SchemaVersion != dashboard.LayoutSchemaVersion ||
		cfg.Dashboard.DefaultVariant != "desktop" {
		t.Fatalf("layout metadata was not exported: %+v", cfg.Dashboard)
	}
	if len(cfg.Dashboard.Variants) == 0 || cfg.Dashboard.Variants[3].Columns != 14 {
		t.Fatalf("layout variants were not exported: %+v", cfg.Dashboard.Variants)
	}
	if got := cfg.Dashboard.Variants[3].Placements[layout.Widgets[0].ID]; got.X != 2 || got.W != 4 {
		t.Fatalf("layout variant placement was not exported: %+v", got)
	}
}

func TestSaveWidgetLayoutRejectsInvalidLayouts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*WidgetLayout)
	}{
		{
			name: "empty profile",
			mutate: func(layout *WidgetLayout) {
				layout.ProfileID = ""
			},
		},
		{
			name: "duplicate id",
			mutate: func(layout *WidgetLayout) {
				layout.Widgets[1].ID = layout.Widgets[0].ID
			},
		},
		{
			name: "duplicate single instance kind",
			mutate: func(layout *WidgetLayout) {
				layout.Widgets[1].Kind = layout.Widgets[0].Kind
			},
		},
		{
			name: "unknown kind",
			mutate: func(layout *WidgetLayout) {
				layout.Widgets[0].Kind = "unknown"
			},
		},
		{
			name: "bad dimensions",
			mutate: func(layout *WidgetLayout) {
				layout.Widgets[0].W = 0
			},
		},
		{
			name: "out of bounds",
			mutate: func(layout *WidgetLayout) {
				layout.Widgets[0].X = 3
				layout.Widgets[0].W = 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := openTestStore(t)
			defer st.Close()
			if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			layout := DefaultWidgetLayout()
			tt.mutate(&layout)
			if _, err := st.DashboardRepo.SaveWidgetLayout(
				context.Background(),
				layout,
			); !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("SaveWidgetLayout() error = %v, want ErrInvalidLayout", err)
			}
		})
	}
}

func TestResetWidgetLayoutRestoresDefaults(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout := DefaultWidgetLayout()
	layout.Widgets[0].Visible = false
	if _, err := st.DashboardRepo.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}

	reset, err := st.DashboardRepo.ResetWidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("ResetWidgetLayout() error = %v", err)
	}
	if len(reset.Widgets) != 4 || !reset.Widgets[0].Visible || reset.Widgets[0].X != 0 {
		t.Fatalf("unexpected reset layout: %+v", reset)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	st, err := Open(filepath.Join(t.TempDir(), "jute.db"), logger)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return st
}

func assertCount(t *testing.T, st *Store, table string, want int) {
	t.Helper()
	var got int64
	if err := st.DB().Table(table).Count(&got).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if int(got) != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}

func writeStoreYAMLConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jute.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write YAML config: %v", err)
	}
	return path
}
