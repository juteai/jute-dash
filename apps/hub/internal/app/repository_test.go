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
	"jute-dash/apps/hub/internal/app/repository"
	"jute-dash/apps/hub/internal/app/voice"
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
	bootstrap.Voice.STTProviderID = "local-stt"
	bootstrap.Voice.PreferredAgentID = "house"
	bootstrap.Voice.FollowupWindowSeconds = 10
	bootstrap.Voice.WakeWordModelID = "hey-jute"
	bootstrap.Voice.WakeWordPhrase = "Hey Jute"
	bootstrap.Voice.WakeSensitivity = 0.7
	bootstrap.Voice.TTSProviderID = "tts-local"
	bootstrap.Voice.TTSModelID = "tts-model"
	bootstrap.Voice.TTSVoiceID = "voice-en"
	bootstrap.Voice.TTSEnabled = true
	bootstrap.Voice.TTSLocale = "en-GB"
	bootstrap.Voice.TTSSpeed = 1.1
	bootstrap.Voice.TTSVolume = 0.8

	result, err := st.Initialize(context.Background(), bootstrap, true)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	cfg := result.Config.(config.Config)
	if !cfg.Voice.Enabled || cfg.Voice.MutedByDefault ||
		cfg.Voice.STTProviderID != "local-stt" {
		t.Fatalf("unexpected config voice settings: %+v", cfg.Voice)
	}

	settings, err := st.VoiceRepo.VoiceSettings(context.Background(), "")
	if err != nil {
		t.Fatalf("VoiceSettings() error = %v", err)
	}
	if !settings.Enabled || settings.Muted || settings.STTProviderID != "local-stt" ||
		settings.PreferredAgentID != "house" ||
		settings.FollowupWindowSeconds != 10 ||
		settings.WakeWordModelID != "hey-jute" ||
		settings.WakeWordPhrase != "Hey Jute" ||
		settings.WakeSensitivity != 0.7 ||
		settings.TTSProviderID != "tts-local" ||
		settings.TTSModelID != "tts-model" ||
		settings.TTSVoiceID != "voice-en" ||
		!settings.TTSEnabled ||
		settings.TTSLocale != "en-GB" ||
		settings.TTSSpeed != 1.1 ||
		settings.TTSVolume != 0.8 {
		t.Fatalf("unexpected voice settings: %+v", settings)
	}
}

func TestVoiceMuteAndCancelUpdateState(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	bootstrap := DefaultConfig()
	bootstrap.Voice.Enabled = true
	bootstrap.Voice.MutedByDefault = false
	bootstrap.Voice.STTProviderID = "local-stt"
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
	if cancelled.Muted || cancelled.STTProviderID != "local-stt" {
		t.Fatalf("cancel should preserve durable voice settings: %+v", cancelled)
	}
}

func TestSaveVoiceSettingsPersistsDeviceProfileDurableFields(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	bootstrap := DefaultConfig()
	bootstrap.Voice.Enabled = false
	bootstrap.Voice.MutedByDefault = true
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	enabled := true
	wakeModel := "hey-jute"
	wakePhrase := "Hey Jute"
	wakeSensitivity := 0.25
	sttProvider := "local-stt"
	ttsProvider := "local-tts"
	sttModel := "tiny-en"
	ttsModel := "voice-model"
	ttsVoice := "amy"
	ttsEnabled := true
	locale := "en-GB"
	speed := 1.2
	volume := 0.0
	agent := "house"
	cloud := true
	command := true
	policy := "ask_before_sensitive"
	followup := 12
	mic := "kitchen-array"

	saved, err := st.VoiceRepo.SaveVoiceSettings(context.Background(), voice.SettingsUpdateRequest{
		Enabled:                 &enabled,
		WakeWordModelID:         &wakeModel,
		WakeWordPhrase:          &wakePhrase,
		WakeSensitivity:         &wakeSensitivity,
		STTProviderID:           &sttProvider,
		TTSProviderID:           &ttsProvider,
		STTModelID:              &sttModel,
		TTSModelID:              &ttsModel,
		TTSVoiceID:              &ttsVoice,
		TTSEnabled:              &ttsEnabled,
		TTSLocale:               &locale,
		TTSSpeed:                &speed,
		TTSVolume:               &volume,
		PreferredAgentID:        &agent,
		CloudOptIn:              &cloud,
		CommandProvidersEnabled: &command,
		SensitiveOutputPolicy:   &policy,
		FollowupWindowSeconds:   &followup,
		MicrophoneProfile:       &mic,
	})
	if err != nil {
		t.Fatalf("SaveVoiceSettings() error = %v", err)
	}

	if !saved.Enabled ||
		saved.WakeWordModelID != "hey-jute" ||
		saved.WakeWordPhrase != "Hey Jute" ||
		saved.WakeSensitivity != 0.25 ||
		saved.STTProviderID != "local-stt" ||
		saved.TTSProviderID != "local-tts" ||
		saved.STTModelID != "tiny-en" ||
		saved.TTSModelID != "voice-model" ||
		saved.TTSVoiceID != "amy" ||
		!saved.TTSEnabled ||
		saved.TTSLocale != "en-GB" ||
		saved.TTSSpeed != 1.2 ||
		saved.TTSVolume != 0 ||
		saved.PreferredAgentID != "house" ||
		!saved.CloudOptIn ||
		!saved.CommandProvidersEnabled ||
		saved.SensitiveOutputPolicy != "ask_before_sensitive" ||
		saved.FollowupWindowSeconds != 12 ||
		saved.MicrophoneProfile != "kitchen-array" {
		t.Fatalf("unexpected saved voice settings: %+v", saved)
	}
}

func TestSaveVoiceSettingsRejectsOutOfRangeValues(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.Initialize(context.Background(), DefaultConfig(), true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	followup := 45

	_, err := st.VoiceRepo.SaveVoiceSettings(context.Background(), voice.SettingsUpdateRequest{
		FollowupWindowSeconds: &followup,
	})
	if err == nil {
		t.Fatal("SaveVoiceSettings() expected validation error")
	}
	if !strings.Contains(err.Error(), "followupWindowSeconds") {
		t.Fatalf("unexpected validation error: %v", err)
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

func TestVoiceProvidersIncludesWakeWordManifestSummary(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	manifest := map[string]any{
		"id":      "org.example.openwakeword",
		"name":    "Example openWakeWord",
		"version": "1.0.0",
		"kind":    voice.ProviderKindWakeWord,
		"transport": map[string]any{
			"type":    "command",
			"command": "/usr/local/bin/jute-wake",
			"args":    []string{"detect", "{inputPath}", "--model", "{modelId}"},
		},
		"capabilities": map[string]any{
			"offline":   true,
			"languages": []string{"en", "en-GB"},
		},
		"credentials": []map[string]any{{
			"id":       "apiKey",
			"label":    "API key",
			"source":   "env",
			"env":      "OPENWAKEWORD_SECRET_ENV",
			"required": false,
		}},
		"wakeWord": map[string]any{
			"defaultModelId": "hey-jute",
			"phrase":         "Hey Jute",
			"languages":      []string{"en"},
			"sensitivity":    0.62,
			"models": []map[string]any{{
				"id":          "hey-jute",
				"path":        "assets/hey-jute.tflite",
				"phrase":      "Hey Jute",
				"languages":   []string{"en"},
				"sensitivity": 0.62,
			}},
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	provider := voice.ProviderPackDB{
		ID:               "org.example.openwakeword",
		Name:             "Example openWakeWord",
		Version:          "1.0.0",
		Kind:             voice.ProviderKindWakeWord,
		TransportType:    "command",
		ManifestJSON:     string(manifestBytes),
		HealthStatus:     "available",
		LastActivationAt: "2026-06-13T12:00:00Z",
		LastError:        "token=provider-secret connection failed",
		UpdatedAt:        "2026-06-13T12:00:00Z",
	}
	if err := st.DB().Create(&provider).Error; err != nil {
		t.Fatalf("insert provider: %v", err)
	}

	providers, err := st.VoiceRepo.VoiceProviders(context.Background())
	if err != nil {
		t.Fatalf("VoiceProviders() error = %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected one provider, got %+v", providers)
	}
	got := providers[0]
	if got.Kind != voice.ProviderKindWakeWord || got.WakeWord == nil {
		t.Fatalf("expected wake-word provider summary, got %+v", got)
	}
	if got.WakeWord.DefaultModelID != "hey-jute" ||
		got.WakeWord.Phrase != "Hey Jute" ||
		got.WakeWord.Sensitivity != 0.62 ||
		len(got.WakeWord.Models) != 1 {
		t.Fatalf("unexpected wake-word summary: %+v", got.WakeWord)
	}
	if got.LastError != "token=[redacted] connection failed" {
		t.Fatalf("provider last error was not redacted: %+v", got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal provider response: %v", err)
	}
	if strings.Contains(string(raw), "OPENWAKEWORD_SECRET_ENV") ||
		strings.Contains(string(raw), "apiKey") ||
		strings.Contains(string(raw), "provider-secret") {
		t.Fatalf("provider response leaked credential metadata: %s", string(raw))
	}
}

func TestActiveWakeProviderIgnoresUnsupportedOrUnsafeProviders(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		health      string
		offline     bool
		modelID     string
		credentials []voice.CredentialManifest
	}{
		{
			name:    "voice disabled",
			enabled: false,
			health:  "available",
			offline: true,
		},
		{
			name:    "unhealthy provider",
			enabled: true,
			health:  "offline",
			offline: true,
		},
		{
			name:    "cloud http provider",
			enabled: true,
			health:  "available",
			offline: false,
		},
		{
			name:    "unknown selected model",
			enabled: true,
			health:  "available",
			offline: true,
			modelID: "not-in-pack",
		},
		{
			name:    "missing required credential",
			enabled: true,
			health:  "available",
			offline: true,
			credentials: []voice.CredentialManifest{{
				ID:       "apiKey",
				Label:    "API key",
				Source:   "env",
				Env:      "JUTE_TEST_MISSING_ACTIVE_WAKE_KEY",
				Required: true,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := openTestStore(t)
			defer st.Close()
			bootstrap := DefaultConfig()
			bootstrap.Voice.Enabled = tt.enabled
			bootstrap.Voice.WakeWordModelID = tt.modelID
			if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			insertWakeProvider(t, st, "local-wake", tt.health, tt.offline, tt.credentials)

			provider, err := st.VoiceRepo.ActiveWakeProvider(context.Background(), "", "")
			if err != nil {
				t.Fatalf("ActiveWakeProvider() error = %v", err)
			}
			if provider != nil {
				t.Fatalf("expected no active wake provider, got %T", provider)
			}
		})
	}
}

func TestActiveSTTProviderResolvesSelectedCommandProviderWhenEnabled(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	bootstrap := DefaultConfig()
	bootstrap.Voice.Enabled = true
	bootstrap.Voice.CommandProvidersEnabled = true
	bootstrap.Voice.STTProviderID = "go-whisper-command"
	bootstrap.Voice.STTModelID = "tiny-en"
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	insertSTTProviderWithTransport(t, st, "go-whisper-command", "available", true, nil, voice.TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/gowhisper",
		Args:    []string{"transcribe", "--model", "{modelId}", "--input", "{inputPath}", "--json"},
	})

	provider, err := st.VoiceRepo.ActiveSTTProvider(context.Background(), "")
	if err != nil {
		t.Fatalf("ActiveSTTProvider() error = %v", err)
	}
	command, ok := provider.(voice.CommandSTTProvider)
	if !ok {
		t.Fatalf("expected CommandSTTProvider, got %T", provider)
	}
	if command.ProviderID != "go-whisper-command" ||
		command.Command != "/usr/local/bin/gowhisper" ||
		command.ModelID != "tiny-en" ||
		command.Language != "en-GB" ||
		strings.Join(command.Args, " ") != "transcribe --model {modelId} --input {inputPath} --json" {
		t.Fatalf("unexpected active STT provider: %+v", command)
	}
}

func TestActiveSTTProviderIgnoresCommandProviderWhenDisabled(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	bootstrap := DefaultConfig()
	bootstrap.Voice.Enabled = true
	bootstrap.Voice.STTProviderID = "go-whisper-command"
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	insertSTTProviderWithTransport(t, st, "go-whisper-command", "available", true, nil, voice.TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/gowhisper",
		Args:    []string{"transcribe", "--model", "{modelId}", "--input", "{inputPath}", "--json"},
	})

	provider, err := st.VoiceRepo.ActiveSTTProvider(context.Background(), "")
	if err != nil {
		t.Fatalf("ActiveSTTProvider() error = %v", err)
	}
	if provider != nil {
		t.Fatalf("expected no active STT provider, got %T", provider)
	}
}

func TestActiveSTTProviderIgnoresUnsupportedOrUnsafeProviders(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		health      string
		offline     bool
		credentials []voice.CredentialManifest
	}{
		{
			name:    "voice disabled",
			enabled: false,
			health:  "available",
			offline: true,
		},
		{
			name:    "unhealthy provider",
			enabled: true,
			health:  "offline",
			offline: true,
		},
		{
			name:    "cloud http provider",
			enabled: true,
			health:  "available",
			offline: false,
		},
		{
			name:    "missing required credential",
			enabled: true,
			health:  "available",
			offline: true,
			credentials: []voice.CredentialManifest{{
				ID:       "apiKey",
				Label:    "API key",
				Source:   "env",
				Env:      "JUTE_TEST_MISSING_ACTIVE_STT_KEY",
				Required: true,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := openTestStore(t)
			defer st.Close()
			bootstrap := DefaultConfig()
			bootstrap.Voice.Enabled = tt.enabled
			bootstrap.Voice.STTProviderID = "local-stt"
			if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			insertSTTProvider(t, st, "local-stt", tt.health, tt.offline, tt.credentials)

			provider, err := st.VoiceRepo.ActiveSTTProvider(context.Background(), "")
			if err != nil {
				t.Fatalf("ActiveSTTProvider() error = %v", err)
			}
			if provider != nil {
				t.Fatalf("expected no active STT provider, got %T", provider)
			}
		})
	}
}

func TestTTSVoicesReturnsLocalProviderVoices(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	bootstrap := DefaultConfig()
	bootstrap.Voice.TTSProviderID = "local-tts"
	bootstrap.Voice.TTSVoiceID = "amy"
	bootstrap.Voice.TTSLocale = "en-GB"
	bootstrap.Voice.TTSSpeed = 1.1
	bootstrap.Voice.TTSVolume = 0.8
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	insertTTSProvider(t, st, "local-tts", "available", true, nil)

	response, err := st.VoiceRepo.TTSVoices(context.Background(), "", "")
	if err != nil {
		t.Fatalf("TTSVoices() error = %v", err)
	}
	if response.ProviderID != "local-tts" ||
		response.SetupStatus != "available" ||
		response.HealthStatus != "available" ||
		response.CloudProvider ||
		response.SelectedVoiceID != "amy" ||
		response.SelectedModelID != "local-model" ||
		response.Locale != "en-GB" ||
		response.Speed != 1.1 ||
		response.Volume != 0.8 ||
		len(response.Voices) != 2 {
		t.Fatalf("unexpected TTS voices response: %+v", response)
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(raw), "env") ||
		strings.Contains(string(raw), "secret") {
		t.Fatalf("TTS voices response leaked credential metadata: %s", string(raw))
	}
}

func TestTTSVoicesFallsBackWhenSelectedVoiceIsNoLongerDeclared(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	bootstrap := DefaultConfig()
	bootstrap.Voice.TTSProviderID = "local-tts"
	bootstrap.Voice.TTSVoiceID = "removed-voice"
	bootstrap.Voice.TTSLocale = "en-GB"
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	insertTTSProvider(t, st, "local-tts", "available", true, nil)

	response, err := st.VoiceRepo.TTSVoices(context.Background(), "", "")
	if err != nil {
		t.Fatalf("TTSVoices() error = %v", err)
	}
	if response.SelectedVoiceID != "amy" ||
		response.SelectedModelID != "local-model" ||
		len(response.Voices) != 2 {
		t.Fatalf("expected selected voice to fall back to provider default, got %+v", response)
	}
}

func TestTTSVoicesHandlesUnavailableProviderStates(t *testing.T) {
	tests := []struct {
		name           string
		providerID     string
		health         string
		offline        bool
		cloudOptIn     bool
		credentials    []voice.CredentialManifest
		wantSetup      string
		wantHealth     string
		wantCloud      bool
		wantVoiceCount int
	}{
		{
			name:           "disabled provider",
			providerID:     "disabled-tts",
			health:         "disabled",
			offline:        true,
			wantSetup:      "disabled",
			wantHealth:     "disabled",
			wantVoiceCount: 0,
		},
		{
			name:           "cloud provider without opt in",
			providerID:     "cloud-tts",
			health:         "available",
			offline:        false,
			cloudOptIn:     false,
			wantSetup:      "disabled",
			wantHealth:     "disabled",
			wantCloud:      true,
			wantVoiceCount: 0,
		},
		{
			name:       "missing required credential",
			providerID: "credential-tts",
			health:     "available",
			offline:    true,
			credentials: []voice.CredentialManifest{{
				ID:       "apiKey",
				Label:    "API key",
				Source:   "env",
				Env:      "JUTE_TEST_MISSING_TTS_KEY",
				Required: true,
			}},
			wantSetup:      "misconfigured",
			wantHealth:     "misconfigured",
			wantVoiceCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := openTestStore(t)
			defer st.Close()
			bootstrap := DefaultConfig()
			bootstrap.Voice.TTSProviderID = tt.providerID
			bootstrap.Voice.CloudOptIn = tt.cloudOptIn
			if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			insertTTSProvider(t, st, tt.providerID, tt.health, tt.offline, tt.credentials)

			response, err := st.VoiceRepo.TTSVoices(context.Background(), "", "")
			if err != nil {
				t.Fatalf("TTSVoices() error = %v", err)
			}
			if response.SetupStatus != tt.wantSetup ||
				response.HealthStatus != tt.wantHealth ||
				response.CloudProvider != tt.wantCloud ||
				len(response.Voices) != tt.wantVoiceCount {
				t.Fatalf("unexpected TTS voices response: %+v", response)
			}
			raw, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			if strings.Contains(string(raw), "JUTE_TEST_MISSING_TTS_KEY") ||
				strings.Contains(string(raw), "apiKey") {
				t.Fatalf("TTS voices response leaked credential metadata: %s", string(raw))
			}
		})
	}
}

func TestActiveTTSProviderFallsBackWhenSelectedVoiceIsNoLongerDeclared(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	bootstrap := DefaultConfig()
	bootstrap.Voice.TTSProviderID = "local-tts"
	bootstrap.Voice.TTSVoiceID = "removed-voice"
	bootstrap.Voice.TTSLocale = "en-GB"
	bootstrap.Voice.TTSEnabled = true
	bootstrap.Voice.CommandProvidersEnabled = true
	if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	insertTTSProvider(t, st, "local-tts", "available", true, nil)

	provider, err := st.VoiceRepo.ActiveTTSProvider(context.Background(), "")
	if err != nil {
		t.Fatalf("ActiveTTSProvider() error = %v", err)
	}
	command, ok := provider.(voice.CommandTTSProvider)
	if !ok {
		t.Fatalf("expected CommandTTSProvider, got %T", provider)
	}
	if command.VoiceID != "amy" || command.Locale != "en-GB" {
		t.Fatalf("expected active provider to use default voice metadata, got %+v", command)
	}
}

func TestActiveTTSProviderIgnoresUnsupportedOrUnsafeProviders(t *testing.T) {
	tests := []struct {
		name        string
		health      string
		offline     bool
		ttsEnabled  bool
		credentials []voice.CredentialManifest
	}{
		{
			name:       "disabled setting",
			health:     "available",
			offline:    true,
			ttsEnabled: false,
		},
		{
			name:       "unhealthy provider",
			health:     "offline",
			offline:    true,
			ttsEnabled: true,
		},
		{
			name:       "cloud http provider",
			health:     "available",
			offline:    false,
			ttsEnabled: true,
		},
		{
			name:       "missing required credential",
			health:     "available",
			offline:    true,
			ttsEnabled: true,
			credentials: []voice.CredentialManifest{{
				ID:       "apiKey",
				Label:    "API key",
				Source:   "env",
				Env:      "JUTE_TEST_MISSING_ACTIVE_TTS_KEY",
				Required: true,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := openTestStore(t)
			defer st.Close()
			bootstrap := DefaultConfig()
			bootstrap.Voice.TTSProviderID = "local-tts"
			bootstrap.Voice.TTSEnabled = tt.ttsEnabled
			bootstrap.Voice.CommandProvidersEnabled = true
			if _, err := st.Initialize(context.Background(), bootstrap, true); err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			insertTTSProvider(t, st, "local-tts", tt.health, tt.offline, tt.credentials)

			provider, err := st.VoiceRepo.ActiveTTSProvider(context.Background(), "")
			if err != nil {
				t.Fatalf("ActiveTTSProvider() error = %v", err)
			}
			if provider != nil {
				t.Fatalf("expected no active TTS provider, got %T", provider)
			}
		})
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

func insertTTSProvider(
	t *testing.T,
	st *Store,
	providerID string,
	health string,
	offline bool,
	credentials []voice.CredentialManifest,
) {
	t.Helper()
	transport := voice.TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/jute-tts",
		Args:    []string{"--voice", "{modelId}", "--locale", "{language}"},
	}
	insertTTSProviderWithTransport(t, st, providerID, health, offline, credentials, transport)
}

func insertTTSProviderWithTransport(
	t *testing.T,
	st *Store,
	providerID string,
	health string,
	offline bool,
	credentials []voice.CredentialManifest,
	transport voice.TransportManifest,
) {
	t.Helper()
	if credentials == nil {
		credentials = []voice.CredentialManifest{}
	}
	manifest := voice.ProviderManifest{
		ID:        providerID,
		Name:      "Test TTS",
		Version:   "1.0.0",
		Kind:      voice.ProviderKindTTS,
		Transport: transport,
		Capabilities: voice.ProviderCapabilities{
			Streaming: true,
			Offline:   offline,
			Languages: []string{"en", "en-GB"},
		},
		Credentials: credentials,
		TTS: voice.TTSManifest{
			DefaultVoiceID: "amy",
			DefaultModelID: "local-model",
			Voices: []voice.TTSVoiceManifest{
				{
					ID:      "amy",
					Label:   "Amy",
					Locale:  "en-GB",
					ModelID: "local-model",
				},
				{
					ID:      "dan",
					Label:   "Dan",
					Locale:  "en-US",
					ModelID: "local-model",
				},
			},
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal TTS manifest: %v", err)
	}
	provider := voice.ProviderPackDB{
		ID:            providerID,
		Name:          "Test TTS",
		Version:       "1.0.0",
		Kind:          voice.ProviderKindTTS,
		TransportType: transport.Type,
		ManifestJSON:  string(manifestBytes),
		HealthStatus:  health,
		UpdatedAt:     "2026-06-13T12:30:00Z",
	}
	if err := st.DB().Create(&provider).Error; err != nil {
		t.Fatalf("insert TTS provider: %v", err)
	}
}

func insertSTTProvider(
	t *testing.T,
	st *Store,
	providerID string,
	health string,
	offline bool,
	credentials []voice.CredentialManifest,
) {
	t.Helper()
	transport := voice.TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/jute-stt",
		Args:    []string{"--model", "{modelId}", "--input", "{inputPath}"},
	}
	insertSTTProviderWithTransport(t, st, providerID, health, offline, credentials, transport)
}

func insertSTTProviderWithTransport(
	t *testing.T,
	st *Store,
	providerID string,
	health string,
	offline bool,
	credentials []voice.CredentialManifest,
	transport voice.TransportManifest,
) {
	t.Helper()
	if credentials == nil {
		credentials = []voice.CredentialManifest{}
	}
	manifest := voice.ProviderManifest{
		ID:        providerID,
		Name:      "Test STT",
		Version:   "1.0.0",
		Kind:      voice.ProviderKindSTT,
		Transport: transport,
		Capabilities: voice.ProviderCapabilities{
			Streaming:          true,
			PartialTranscripts: true,
			Offline:            offline,
			Languages:          []string{"en-GB", "en"},
			InputFormats:       []string{"audio/pcm;rate=16000;width=2;channels=1"},
		},
		Credentials: credentials,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal STT manifest: %v", err)
	}
	provider := voice.ProviderPackDB{
		ID:            providerID,
		Name:          "Test STT",
		Version:       "1.0.0",
		Kind:          voice.ProviderKindSTT,
		TransportType: transport.Type,
		ManifestJSON:  string(manifestBytes),
		HealthStatus:  health,
		UpdatedAt:     "2026-06-13T12:30:00Z",
	}
	if err := st.DB().Create(&provider).Error; err != nil {
		t.Fatalf("insert STT provider: %v", err)
	}
}

func insertWakeProvider(
	t *testing.T,
	st *Store,
	providerID string,
	health string,
	offline bool,
	credentials []voice.CredentialManifest,
) {
	t.Helper()
	transport := voice.TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/jute-wake",
		Args:    []string{"--model", "{modelId}", "--input", "{inputPath}"},
	}
	if credentials == nil {
		credentials = []voice.CredentialManifest{}
	}
	manifest := voice.ProviderManifest{
		ID:        providerID,
		Name:      "Test Wake",
		Version:   "1.0.0",
		Kind:      voice.ProviderKindWakeWord,
		Transport: transport,
		Capabilities: voice.ProviderCapabilities{
			Streaming:    true,
			Offline:      offline,
			Languages:    []string{"en-GB", "en"},
			InputFormats: []string{"audio/pcm;rate=16000;width=2;channels=1"},
		},
		Credentials: credentials,
		WakeWord: voice.WakeWordManifest{
			DefaultModelID: "hey-jute",
			Phrase:         "Hey Jute",
			Languages:      []string{"en"},
			Sensitivity:    0.6,
			Models: []voice.WakeWordModelManifest{{
				ID:          "hey-jute",
				Path:        "models/hey-jute.tflite",
				Phrase:      "Hey Jute",
				Languages:   []string{"en"},
				Sensitivity: 0.6,
			}},
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal wake manifest: %v", err)
	}
	provider := voice.ProviderPackDB{
		ID:            providerID,
		Name:          "Test Wake",
		Version:       "1.0.0",
		Kind:          voice.ProviderKindWakeWord,
		TransportType: transport.Type,
		ManifestJSON:  string(manifestBytes),
		HealthStatus:  health,
		UpdatedAt:     "2026-06-13T12:30:00Z",
	}
	if err := st.DB().Create(&provider).Error; err != nil {
		t.Fatalf("insert wake provider: %v", err)
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

func TestWidgetLayoutMigratesV2LayoutToHomeScreen(t *testing.T) {
	layout := dashboard.WidgetLayout{
		ProfileID:     defaultLayoutProfileID,
		SchemaVersion: 2,
		Widgets: []dashboard.WidgetInstance{{
			ID:       "clock",
			Kind:     "date-time",
			Title:    "Clock",
			X:        0,
			Y:        0,
			W:        6,
			H:        1,
			MinW:     3,
			MinH:     1,
			Size:     "wide",
			Mode:     "ui",
			Settings: map[string]any{},
			Visible:  true,
		}},
	}
	normalized, err := dashboard.NormalizeWidgetLayout(layout, repository.WidgetCatalogForSeed())
	if err != nil {
		t.Fatalf("NormalizeWidgetLayout() error = %v", err)
	}
	if normalized.SchemaVersion != dashboard.LayoutSchemaVersion ||
		normalized.DefaultScreen != "home" ||
		normalized.ActiveScreen != "home" ||
		len(normalized.Screens) != 1 ||
		normalized.Screens[0].Widgets[0].ScreenID != "home" {
		t.Fatalf("layout was not migrated to home screen: %+v", normalized)
	}
}

func TestSaveWidgetLayoutPersistsScreenIDsAndActiveScreen(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.Initialize(context.Background(), DefaultConfig(), false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	layout := DefaultWidgetLayout()
	layout.Screens = append(layout.Screens, dashboard.DashboardScreen{
		ID:    "music",
		Label: "Music",
		Widgets: []dashboard.WidgetInstance{{
			ScreenID:       "music",
			ID:             "music-markets",
			Kind:           "markets",
			Title:          "Music Markets",
			X:              0,
			Y:              0,
			W:              6,
			H:              2,
			MinW:           3,
			MinH:           1,
			Size:           "medium",
			Mode:           "ui",
			Settings:       map[string]any{},
			ConnectionRefs: map[string]string{},
			Visible:        true,
		}},
	})
	layout.ActiveScreen = "music"

	if _, err := st.DashboardRepo.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}
	reloaded, err := st.DashboardRepo.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if reloaded.ActiveScreen != "music" || len(reloaded.Screens) != 2 {
		t.Fatalf("screen metadata did not persist: %+v", reloaded)
	}
	if got := reloaded.Screens[1].Widgets[0]; got.ScreenID != "music" || got.ID != "music-markets" {
		t.Fatalf("screen widget did not persist: %+v", got)
	}

	active, err := st.DashboardRepo.SetActiveScreen(context.Background(), "", "home")
	if err != nil {
		t.Fatalf("SetActiveScreen() error = %v", err)
	}
	if active.ActiveScreen != "home" {
		t.Fatalf("active screen was not updated: %+v", active)
	}
}

func TestSaveWidgetLayoutRejectsDuplicateWidgetIDsAcrossScreens(t *testing.T) {
	layout := DefaultWidgetLayout()
	duplicate := layout.Screens[0].Widgets[0]
	duplicate.ScreenID = "other"
	layout.Screens = append(layout.Screens, dashboard.DashboardScreen{
		ID:      "other",
		Label:   "Other",
		Widgets: []dashboard.WidgetInstance{duplicate},
	})
	if _, err := dashboard.NormalizeWidgetLayout(
		layout,
		repository.WidgetCatalogForSeed(),
	); !errors.Is(err, dashboard.ErrInvalidLayout) {
		t.Fatalf("NormalizeWidgetLayout() error = %v, want ErrInvalidLayout", err)
	}
}

func TestSaveWidgetLayoutRejectsDuplicateSingleInstanceKindsAcrossScreens(t *testing.T) {
	layout := DefaultWidgetLayout()
	duplicate := layout.Screens[0].Widgets[0]
	duplicate.ID += "-copy"
	duplicate.ScreenID = "other"
	layout.Screens = append(layout.Screens, dashboard.DashboardScreen{
		ID:      "other",
		Label:   "Other",
		Widgets: []dashboard.WidgetInstance{duplicate},
	})
	if _, err := dashboard.NormalizeWidgetLayout(
		layout,
		repository.WidgetCatalogForSeed(),
	); !errors.Is(err, dashboard.ErrInvalidLayout) {
		t.Fatalf("NormalizeWidgetLayout() error = %v, want ErrInvalidLayout", err)
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
