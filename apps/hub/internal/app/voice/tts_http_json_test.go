package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPJSONTTSProviderSynthesizesAudio(t *testing.T) {
	requests := make(chan struct {
		Text    string `json:"text"`
		ModelID string `json:"modelId"`
		VoiceID string `json:"voiceId"`
		Locale  string `json:"locale"`
	}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Text    string `json:"text"`
			ModelID string `json:"modelId"`
			VoiceID string `json:"voiceId"`
			Locale  string `json:"locale"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		requests <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"providerId": "local-http-tts",
			"voiceId": "amy",
			"locale": "en-GB",
			"sampleRate": 16000,
			"sampleWidth": 2,
			"channels": 1,
			"durationMs": 12,
			"audio": {"contentType": "audio/wav", "data": "AQIDBA=="}
		}`))
	}))
	defer server.Close()
	provider := HTTPJSONTTSProvider{
		ProviderID: "local-http-tts",
		Endpoint:   server.URL,
		ModelID:    "piper-en",
		VoiceID:    "amy",
		Locale:     "en-GB",
	}

	result, err := provider.Synthesize(context.Background(), TTSRequest{Text: "hello kitchen"})
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}
	body := <-requests
	if body.Text != "hello kitchen" ||
		body.ModelID != "piper-en" ||
		body.VoiceID != "amy" ||
		body.Locale != "en-GB" {
		t.Fatalf("unexpected request body: %+v", body)
	}
	if result.ProviderID != "local-http-tts" ||
		result.VoiceID != "amy" ||
		result.Locale != "en-GB" ||
		result.ContentType != "audio/wav" ||
		result.SampleRate != 16000 ||
		result.SampleWidth != 2 ||
		result.Channels != 1 ||
		result.Duration.Milliseconds() != 12 ||
		result.PlaybackKind != "audio" ||
		string(result.Audio) != string([]byte{1, 2, 3, 4}) {
		t.Fatalf("unexpected TTS result: %+v", result)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if bytes.Contains(raw, []byte("AQIDBA")) || bytes.Contains(raw, []byte{1, 2, 3, 4}) {
		t.Fatalf("TTS result JSON leaked raw audio: %s", raw)
	}
}

func TestHTTPJSONTTSProviderRejectsEmptyAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"audio":{"contentType":"audio/wav","data":""}}`))
	}))
	defer server.Close()

	_, err := (HTTPJSONTTSProvider{Endpoint: server.URL}).Synthesize(
		context.Background(),
		TTSRequest{Text: "hello"},
	)
	if err == nil {
		t.Fatal("expected empty audio error")
	}
}
