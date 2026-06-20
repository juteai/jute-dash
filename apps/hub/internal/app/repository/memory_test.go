package repository

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryDashboardRepositoryNormalizesAndSwitchesActiveScreen(t *testing.T) {
	repo := NewMemoryDashboardRepository()
	repo.SetCatalog([]WidgetCatalogItem{{
		Kind:          "clock",
		DefaultTitle:  "Clock",
		DefaultSize:   "medium",
		MinW:          2,
		MinH:          1,
		AllowMultiple: true,
	}})

	layout, err := repo.SaveWidgetLayout(context.Background(), WidgetLayout{
		ProfileID: "profile-1",
		Screens: []DashboardScreen{
			{ID: "home", Widgets: []WidgetInstance{{ID: "clock-1", Kind: "clock", W: 2, H: 1, Visible: true}}},
			{ID: "second", Widgets: []WidgetInstance{{ID: "clock-2", Kind: "clock", W: 2, H: 1, Visible: true}}},
		},
	})
	if err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}
	if layout.Widgets[0].Title != "Clock" || layout.Widgets[0].MinW != 2 || layout.Widgets[0].Mode != WidgetModeUI {
		t.Fatalf("layout was not normalized: %+v", layout.Widgets[0])
	}

	layout, err = repo.SetActiveScreen(context.Background(), "profile-1", "second")
	if err != nil {
		t.Fatalf("SetActiveScreen() error = %v", err)
	}
	if layout.ActiveScreen != "second" || layout.DefaultVariant == "" || len(layout.Widgets) != 2 {
		t.Fatalf("active screen did not switch flattened widgets: %+v", layout)
	}
}

func TestMemoryDashboardRepositoryRejectsUnknownWidgetKind(t *testing.T) {
	repo := NewMemoryDashboardRepository()
	repo.SetCatalog([]WidgetCatalogItem{{Kind: "known", AllowMultiple: true}})

	_, err := repo.SaveWidgetLayout(context.Background(), WidgetLayout{
		ProfileID: "profile-1",
		Widgets:   []WidgetInstance{{ID: "missing-1", Kind: "missing", W: 1, H: 1}},
	})
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("SaveWidgetLayout() error = %v, want ErrInvalidLayout", err)
	}
}

func TestMemoryHomeRepositoryClonesAdapterConnections(t *testing.T) {
	repo := NewMemoryHomeRepository(SetupStatus{Complete: true})
	connection := AdapterConnection{
		ID:         "spotify-main",
		Kind:       "spotify",
		Name:       "Spotify",
		Settings:   map[string]any{"client_id": "one"},
		SecretRefs: map[string]string{"refresh_token": "secret/ref"},
		Enabled:    true,
	}
	saved, err := repo.SaveAdapterConnection(context.Background(), connection)
	if err != nil {
		t.Fatalf("SaveAdapterConnection() error = %v", err)
	}
	saved.Settings["client_id"] = "mutated"
	saved.SecretRefs["refresh_token"] = "mutated"

	got, err := repo.AdapterConnection(context.Background(), "spotify-main")
	if err != nil {
		t.Fatalf("AdapterConnection() error = %v", err)
	}
	if got.Settings["client_id"] != "one" || got.SecretRefs["refresh_token"] != "secret/ref" {
		t.Fatalf("connection was not cloned: %+v", got)
	}
}

func TestMemoryVoiceRepositoryAppliesSettingsAndDefaultsTTSVoicesProvider(t *testing.T) {
	repo := NewMemoryVoiceRepository()
	enabled := true
	speed := 1.25
	settings, err := repo.SaveVoiceSettings(context.Background(), SettingsUpdateRequest{
		Enabled:       &enabled,
		TTSProviderID: ptrString("local-tts"),
		TTSVoiceID:    ptrString("amy"),
		TTSSpeed:      &speed,
	})
	if err != nil {
		t.Fatalf("SaveVoiceSettings() error = %v", err)
	}
	if !settings.Enabled || settings.TTSProviderID != "local-tts" || settings.TTSSpeed != 1.25 {
		t.Fatalf("settings not applied: %+v", settings)
	}

	voices, err := repo.TTSVoices(context.Background(), "", "")
	if err != nil {
		t.Fatalf("TTSVoices() error = %v", err)
	}
	if voices.ProviderID != "local-tts" || voices.SelectedVoiceID != "amy" || voices.Speed != 1.25 {
		t.Fatalf("voices did not use active settings: %+v", voices)
	}
}

func ptrString(value string) *string {
	return &value
}
