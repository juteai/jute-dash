package voice

import (
	"strings"
	"testing"
)

func TestSpeechPolicyDefaultsSensitiveContentToVisualOnly(t *testing.T) {
	settings := Settings{SensitiveOutputPolicy: TTSPolicyVisualOnlySensitive}
	allowed, reason := speechPolicyAllows(TTSRequest{Text: "the door code is 1234"}, settings)

	if allowed || reason != "sensitive_output_visual_only" {
		t.Fatalf("expected visual-only sensitive policy, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestSpeechPolicyAllowsSpeakAll(t *testing.T) {
	settings := Settings{SensitiveOutputPolicy: TTSPolicySpeakAll}
	allowed, reason := speechPolicyAllows(TTSRequest{Sensitive: true, Text: "private"}, settings)

	if !allowed || reason != "" {
		t.Fatalf("expected speak-all to allow sensitive output, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestTTSCachePolicy(t *testing.T) {
	req := TTSRequest{Text: "hello", Cache: true}
	settings := Settings{TTSModelID: "model", TTSLocale: "en-GB"}

	eligible, key := cachePolicy(req, settings, "local-tts", "amy")
	if !eligible || key == "" {
		t.Fatalf("expected local non-sensitive speech to be cache eligible, got eligible=%v key=%q", eligible, key)
	}

	settings.CloudOptIn = true
	eligible, key = cachePolicy(req, settings, "cloud-tts", "amy")
	if eligible || key != "" {
		t.Fatalf("expected cloud opt-in speech to skip cache, got eligible=%v key=%q", eligible, key)
	}

	settings.CloudOptIn = false
	req.Text = "token=abc123"
	eligible, key = cachePolicy(req, settings, "local-tts", "amy")
	if eligible || key != "" {
		t.Fatalf("expected sensitive speech to skip cache, got eligible=%v key=%q", eligible, key)
	}
}

func TestTTSCachePolicyUsesEffectiveRequestLocale(t *testing.T) {
	settings := Settings{TTSModelID: "model", TTSLocale: "en-GB"}

	eligible, defaultLocaleKey := cachePolicy(
		TTSRequest{Text: "hello", Cache: true},
		settings,
		"local-tts",
		"amy",
	)
	if !eligible || defaultLocaleKey == "" {
		t.Fatalf("expected default locale cache key, got eligible=%v key=%q", eligible, defaultLocaleKey)
	}
	eligible, requestLocaleKey := cachePolicy(
		TTSRequest{Text: "hello", Cache: true, Locale: "cy-GB"},
		settings,
		"local-tts",
		"amy",
	)
	if !eligible || requestLocaleKey == "" {
		t.Fatalf("expected request locale cache key, got eligible=%v key=%q", eligible, requestLocaleKey)
	}
	if defaultLocaleKey == requestLocaleKey {
		t.Fatalf("expected request locale to change cache key, both were %q", defaultLocaleKey)
	}
	if strings.Contains(requestLocaleKey, "hello") ||
		strings.Contains(requestLocaleKey, "cy-GB") {
		t.Fatalf("cache key exposed raw text or locale: %q", requestLocaleKey)
	}
}

func TestTTSRuntimeBargeInStopsCurrentAction(t *testing.T) {
	runtime := NewTTSRuntime()
	started := runtime.Begin(
		TTSActionSpeak,
		TTSRequest{Text: "hello", ConversationID: "conversation-1", TurnID: "turn-1"},
		Settings{TTSProviderID: "local-tts", TTSVoiceID: "amy"},
	)

	stopped := runtime.Stop(TTSStopRequest{Reason: "barge_in"})

	if stopped.ID != started.ID ||
		stopped.State != TTSStateStopped ||
		stopped.Reason != "barge_in" ||
		stopped.ConversationID != "conversation-1" ||
		stopped.TurnID != "turn-1" {
		t.Fatalf("unexpected stopped state: %+v", stopped)
	}
}

func TestTTSRuntimeResponseRedactsCredentialLikeProviderIdentifiers(t *testing.T) {
	runtime := NewTTSRuntime()
	started := runtime.Begin(
		TTSActionSpeak,
		TTSRequest{
			Text:       "hello",
			ProviderID: "local-tts token=secret",
			VoiceID:    "amy apiKey=secret-value",
		},
		Settings{},
	)

	if strings.Contains(started.ProviderID, "secret") ||
		strings.Contains(started.VoiceID, "secret-value") {
		t.Fatalf("started response leaked provider metadata: %+v", started)
	}
	if started.ProviderID != "local-tts token=[redacted]" ||
		started.VoiceID != "amy apiKey=[redacted]" {
		t.Fatalf("unexpected sanitized response identifiers: %+v", started)
	}

	stopped := runtime.Stop(TTSStopRequest{Reason: "barge_in"})
	if strings.Contains(stopped.ProviderID, "secret") ||
		strings.Contains(stopped.VoiceID, "secret-value") {
		t.Fatalf("stopped response leaked provider metadata: %+v", stopped)
	}
}

func TestTTSRuntimeVisualOnlyClearsCacheAndCancel(t *testing.T) {
	runtime := NewTTSRuntime()
	cancelled := false
	started := runtime.Begin(
		TTSActionSpeak,
		TTSRequest{Text: "the door code is 1234", Cache: true},
		Settings{TTSProviderID: "local-tts", TTSVoiceID: "amy"},
		func() { cancelled = true },
	)

	visualOnly := runtime.VisualOnly(started.ID, "sensitive_output_visual_only")

	if visualOnly.ID != started.ID ||
		visualOnly.State != TTSStateVisualOnly ||
		!visualOnly.VisualOnly ||
		visualOnly.Reason != "sensitive_output_visual_only" ||
		visualOnly.CacheEligible ||
		visualOnly.CacheKey != "" {
		t.Fatalf("unexpected visual-only state: %+v", visualOnly)
	}
	if cancelled {
		t.Fatal("visual-only policy should not call a provider cancellation hook")
	}
}
