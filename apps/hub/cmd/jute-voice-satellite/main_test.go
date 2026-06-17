package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jute-dash/apps/hub/internal/app/voice"
)

func TestRunFixtureSatelliteCommand(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	var sawTranscript bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Jute-Satellite-Auth") != "secret-ref-kitchen" {
			t.Fatalf("missing auth")
		}
		if strings.HasSuffix(r.URL.Path, "/transcripts/final") {
			sawTranscript = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"conversation":{"conversation":{"id":"voice-conversation-cli-1"}},
				"followup":{"active":true,"turns":1,"maxTurns":5}
			}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	fixturePath := writeCommandFixture(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-hub-url", server.URL,
		"-satellite-id", "sat-kitchen",
		"-auth-secret-env", "JUTE_SATELLITE_AUTH",
		"-fixture-wav", fixturePath,
		"-transcript", "turn on the kitchen lights",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !sawTranscript {
		t.Fatal("expected final transcript post")
	}
	if !strings.Contains(stdout.String(), `"conversationId": "voice-conversation-cli-1"`) ||
		!strings.Contains(stdout.String(), `"followupActive": true`) {
		t.Fatalf("expected safe follow-up result, got stdout=%s", stdout.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "secret-ref-kitchen") ||
		strings.Contains(stdout.String()+stderr.String(), "pcm") {
		t.Fatalf("command output leaked unsafe material: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestRunFixtureSatelliteCommandSendsConversationID(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	var sawConversationID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/transcripts/final") {
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode transcript: %v", err)
			}
			sawConversationID = req["conversationId"]
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"conversation":{"conversation":{"id":"voice-conversation-cli-existing"}},
				"followup":{"active":true,"turns":2,"maxTurns":5}
			}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	fixturePath := writeCommandFixture(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-hub-url", server.URL,
		"-satellite-id", "sat-kitchen",
		"-auth-secret-env", "JUTE_SATELLITE_AUTH",
		"-fixture-wav", fixturePath,
		"-transcript", "make it ten minutes",
		"-conversation-id", "voice-conversation-cli-existing",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if sawConversationID != "voice-conversation-cli-existing" {
		t.Fatalf("expected command to send conversation ID, got %q", sawConversationID)
	}
	if !strings.Contains(stdout.String(), `"followupTurns": 2`) {
		t.Fatalf("expected follow-up turn result, got stdout=%s", stdout.String())
	}
}

func TestRunRejectsRawSecretConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "satellite.json")
	if err := os.WriteFile(configPath, []byte(`{
		"hubUrl":"http://127.0.0.1:8787",
		"satelliteId":"sat-kitchen",
		"authSecret":"secret-value",
		"fixtureWav":"fixture.wav",
		"transcript":"hello"
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"-config", configPath}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected config error code, got %d", code)
	}
	if strings.Contains(stderr.String(), "secret-value") {
		t.Fatalf("stderr leaked raw secret: %s", stderr.String())
	}
}

func TestRunRejectsTrailingConfigJSON(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "satellite.json")
	if err := os.WriteFile(configPath, []byte(`{
		"hubUrl":"http://127.0.0.1:8787",
		"satelliteId":"sat-kitchen",
		"authSecretEnv":"JUTE_SATELLITE_AUTH",
		"fixtureWav":"fixture.wav",
		"transcript":"hello"
	}
	{"rawSecret":"secret-value"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"-config", configPath}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected config error code, got %d", code)
	}
	if !strings.Contains(stderr.String(), "trailing JSON data") {
		t.Fatalf("expected trailing JSON error, got %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "secret-value") {
		t.Fatalf("stderr leaked appended secret: %s", stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected no stdout for config decode failure, got %s", stdout.String())
	}
}

func TestRunReportsAuthFailureWithoutLeakingProof(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	fixturePath := writeCommandFixture(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-hub-url", server.URL,
		"-satellite-id", "sat-kitchen",
		"-auth-secret-env", "JUTE_SATELLITE_AUTH",
		"-fixture-wav", fixturePath,
		"-transcript", "RAW_TRANSCRIPT_SHOULD_NOT_PRINT",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected auth failure exit code, got %d", code)
	}
	if !strings.Contains(stdout.String(), `"auth_failed"`) ||
		!strings.Contains(stderr.String(), "auth_failed") {
		t.Fatalf("expected auth_failed diagnostic, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	for _, leaked := range []string{
		"secret-ref-kitchen",
		"RAW_TRANSCRIPT_SHOULD_NOT_PRINT",
		"X-Jute-Satellite-Auth",
		"pcm",
	} {
		if strings.Contains(stdout.String()+stderr.String(), leaked) {
			t.Fatalf("command output leaked %q: stdout=%s stderr=%s", leaked, stdout.String(), stderr.String())
		}
	}
}

func TestRunReportsHubUnreachableWithoutLeakingInputs(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	fixturePath := writeCommandFixture(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-hub-url", "http://127.0.0.1:1",
		"-satellite-id", "sat-kitchen",
		"-auth-secret-env", "JUTE_SATELLITE_AUTH",
		"-fixture-wav", fixturePath,
		"-transcript", "RAW_TRANSCRIPT_SHOULD_NOT_PRINT",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected hub unreachable exit code, got %d", code)
	}
	if !strings.Contains(stdout.String(), `"hub_unreachable"`) ||
		!strings.Contains(stderr.String(), "hub_unreachable") {
		t.Fatalf("expected hub_unreachable diagnostic, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	for _, leaked := range []string{
		"secret-ref-kitchen",
		"RAW_TRANSCRIPT_SHOULD_NOT_PRINT",
		fixturePath,
		"pcm",
	} {
		if strings.Contains(stdout.String()+stderr.String(), leaked) {
			t.Fatalf("command output leaked %q: stdout=%s stderr=%s", leaked, stdout.String(), stderr.String())
		}
	}
}

func TestRunReportsMissingFixtureWithoutLeakingInputs(t *testing.T) {
	t.Setenv("JUTE_SATELLITE_AUTH", "secret-ref-kitchen")
	fixturePath := filepath.Join(t.TempDir(), "missing-fixture.wav")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"-hub-url", "http://127.0.0.1:8787",
		"-satellite-id", "sat-kitchen",
		"-auth-secret-env", "JUTE_SATELLITE_AUTH",
		"-fixture-wav", fixturePath,
		"-transcript", "RAW_TRANSCRIPT_SHOULD_NOT_PRINT",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected missing fixture exit code, got %d", code)
	}
	if !strings.Contains(stdout.String(), `"microphone_unavailable"`) ||
		!strings.Contains(stderr.String(), "microphone_unavailable") {
		t.Fatalf("expected microphone_unavailable diagnostic, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	for _, leaked := range []string{
		"secret-ref-kitchen",
		"RAW_TRANSCRIPT_SHOULD_NOT_PRINT",
		fixturePath,
		"missing-fixture.wav",
		"pcm",
		"read fixture audio",
	} {
		if strings.Contains(stdout.String()+stderr.String(), leaked) {
			t.Fatalf("command output leaked %q: stdout=%s stderr=%s", leaked, stdout.String(), stderr.String())
		}
	}
}

func writeCommandFixture(t *testing.T) string {
	t.Helper()
	utterance, err := voice.NewBenchmarkToneFixture("satellite-command", "fixture", voice.BenchmarkAudioSpec{
		Duration:  220 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.3,
		StartedAt: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(utterance.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "fixture.wav")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}
