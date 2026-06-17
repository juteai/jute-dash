package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSatelliteRuntimeRunsFixtureAgainstHubAPIs(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	var eventBodies []string
	var transcriptBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Jute-Satellite-Auth") != "secret-ref-kitchen" {
			t.Fatalf("missing auth proof")
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/events"):
			var req SatelliteEventRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			raw, _ := json.Marshal(req)
			eventBodies = append(eventBodies, string(raw))
			w.WriteHeader(http.StatusAccepted)
		case strings.HasSuffix(r.URL.Path, "/transcripts/final"):
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode transcript: %v", err)
			}
			transcriptBody = req["text"]
			if req["conversationId"] != "" {
				t.Fatalf("new satellite turn should not send conversationId, got %q", req["conversationId"])
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"conversation":{"conversation":{"id":"voice-conversation-sat-1"}},
				"followup":{"active":true,"expiresAt":"2026-06-15T09:00:08Z","turns":1,"maxTurns":5}
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	fixturePath := writeSatelliteRuntimeFixture(t)
	result, err := (SatelliteRuntime{HTTPClient: server.Client()}).RunFixture(
		context.Background(),
		SatelliteRuntimeConfig{
			HubURL:        server.URL,
			SatelliteID:   "sat-kitchen",
			AuthSecretEnv: "JUTE_SATELLITE_AUTH",
			FixtureWAV:    fixturePath,
			Transcript:    "turn on the kitchen lights",
			Version:       "0.1.0",
			WakeModelID:   "fixture-wake",
		},
	)

	if err != nil {
		t.Fatalf("RunFixture() error = %v, result = %+v", err, result)
	}
	if !result.TranscriptSent || result.EventsSent < 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if transcriptBody != "turn on the kitchen lights" {
		t.Fatalf("unexpected transcript: %q", transcriptBody)
	}
	if result.ConversationID != "voice-conversation-sat-1" ||
		!result.FollowupActive ||
		result.FollowupExpiresAt != "2026-06-15T09:00:08Z" ||
		result.FollowupTurns != 1 ||
		result.FollowupMaxTurns != 5 {
		t.Fatalf("unexpected follow-up result: %+v", result)
	}
	joinedEvents := strings.Join(eventBodies, "\n")
	assertJSONOmits(t, joinedEvents, "secret-ref-kitchen", "pcm", "rawAudio", "preRoll", "token=")
}

func TestSatelliteRuntimeSendsConfiguredConversationIDForFollowup(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	var sawConversationID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/events"):
			w.WriteHeader(http.StatusAccepted)
		case strings.HasSuffix(r.URL.Path, "/transcripts/final"):
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode transcript: %v", err)
			}
			sawConversationID = req["conversationId"]
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"conversation":{"conversation":{"id":"voice-conversation-sat-existing"}},
				"followup":{"active":true,"turns":2,"maxTurns":5}
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := (SatelliteRuntime{HTTPClient: server.Client()}).RunFixture(
		context.Background(),
		SatelliteRuntimeConfig{
			HubURL:         server.URL,
			SatelliteID:    "sat-kitchen",
			AuthSecretEnv:  "JUTE_SATELLITE_AUTH",
			FixtureWAV:     writeSatelliteRuntimeFixture(t),
			Transcript:     "make it ten minutes",
			ConversationID: "voice-conversation-sat-existing",
			Version:        "0.1.0",
			WakeModelID:    "fixture-wake",
		},
	)

	if err != nil {
		t.Fatalf("RunFixture() error = %v, result = %+v", err, result)
	}
	if sawConversationID != "voice-conversation-sat-existing" {
		t.Fatalf("expected follow-up conversation ID, got %q", sawConversationID)
	}
	if result.ConversationID != "voice-conversation-sat-existing" ||
		!result.FollowupActive ||
		result.FollowupTurns != 2 ||
		result.FollowupMaxTurns != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSatelliteRuntimeRejectsPortableRawSecretConfig(t *testing.T) {
	_, err := DecodeSatelliteRuntimeConfig([]byte(`{
		"hubUrl":"http://127.0.0.1:8787",
		"satelliteId":"sat-kitchen",
		"authSecret":"secret-value",
		"fixtureWav":"fixture.wav",
		"transcript":"hello"
	}`))

	if err == nil {
		t.Fatal("expected raw secret config field to be rejected")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error leaked raw secret: %v", err)
	}
}

func TestSatelliteRuntimeRejectsTrailingConfigJSON(t *testing.T) {
	_, err := DecodeSatelliteRuntimeConfig([]byte(`{
		"hubUrl":"http://127.0.0.1:8787",
		"satelliteId":"sat-kitchen",
		"authSecretEnv":"JUTE_SATELLITE_AUTH",
		"fixtureWav":"fixture.wav",
		"transcript":"hello"
	}{"rawSecret":"secret-value"}`))

	if err == nil || !strings.Contains(err.Error(), "trailing JSON data") {
		t.Fatalf("expected trailing JSON decode error, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error leaked appended raw secret: %v", err)
	}
}

func TestSatelliteRuntimeUsesSafeDiagnostics(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	result, err := (SatelliteRuntime{}).RunFixture(context.Background(), SatelliteRuntimeConfig{
		HubURL:        "http://127.0.0.1:1",
		SatelliteID:   "sat-kitchen",
		AuthSecretEnv: "JUTE_SATELLITE_AUTH",
		FixtureWAV:    filepath.Join(t.TempDir(), "missing.wav"),
		Transcript:    "RAW_TRANSCRIPT_SHOULD_NOT_LOG",
	})

	if err == nil {
		t.Fatal("expected missing fixture to fail")
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0] != "microphone_unavailable" {
		t.Fatalf("unexpected diagnostics: %+v", result)
	}
	if strings.Contains(strings.Join(result.Diagnostics, ","), "RAW_TRANSCRIPT") ||
		strings.Contains(strings.Join(result.Diagnostics, ","), "secret-ref-kitchen") {
		t.Fatalf("diagnostics leaked unsafe material: %+v", result.Diagnostics)
	}
}

func TestSatelliteRuntimeReportsClockSkewFromHubDate(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", now.Add(10*time.Minute).Format(http.TimeFormat))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	result, err := (SatelliteRuntime{
		HTTPClient: server.Client(),
		Now:        func() time.Time { return now },
	}).RunFixture(context.Background(), SatelliteRuntimeConfig{
		HubURL:        server.URL,
		SatelliteID:   "sat-kitchen",
		AuthSecretEnv: "JUTE_SATELLITE_AUTH",
		FixtureWAV:    writeSatelliteRuntimeFixture(t),
		Transcript:    "RAW_TRANSCRIPT_SHOULD_NOT_LOG",
		WakeModelID:   "fixture-wake",
	})

	if err == nil {
		t.Fatal("expected clock skew to fail")
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0] != "clock_skew" {
		t.Fatalf("unexpected diagnostics: %+v", result)
	}
	if strings.Contains(strings.Join(result.Diagnostics, ","), "RAW_TRANSCRIPT") ||
		strings.Contains(strings.Join(result.Diagnostics, ","), "secret-ref-kitchen") {
		t.Fatalf("diagnostics leaked unsafe material: %+v", result.Diagnostics)
	}
}

func TestSatelliteRuntimeReportsMissingWakeProvider(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	result, err := (SatelliteRuntime{}).RunFixture(context.Background(), SatelliteRuntimeConfig{
		HubURL:        "http://127.0.0.1:1",
		SatelliteID:   "sat-kitchen",
		AuthSecretEnv: "JUTE_SATELLITE_AUTH",
		FixtureWAV:    writeSatelliteRuntimeFixture(t),
		Transcript:    "RAW_TRANSCRIPT_SHOULD_NOT_LOG",
	})

	if err == nil {
		t.Fatal("expected missing wake provider to fail")
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0] != "wake_provider_unavailable" {
		t.Fatalf("unexpected diagnostics: %+v", result)
	}
	if strings.Contains(strings.Join(result.Diagnostics, ","), "RAW_TRANSCRIPT") ||
		strings.Contains(strings.Join(result.Diagnostics, ","), "secret-ref-kitchen") {
		t.Fatalf("diagnostics leaked unsafe material: %+v", result.Diagnostics)
	}
}

func writeSatelliteRuntimeFixture(t *testing.T) string {
	t.Helper()
	utterance, err := NewBenchmarkToneFixture("satellite-command", "fixture", BenchmarkAudioSpec{
		Duration:  220 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.3,
		StartedAt: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	raw, err := EncodeBenchmarkWAV(utterance.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "fixture.wav")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}
