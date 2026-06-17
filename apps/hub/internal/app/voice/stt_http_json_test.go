package voice

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPJSONSTTProviderPostsWAVAndReturnsTranscript(t *testing.T) {
	fixture, err := NewBenchmarkToneFixture("short-command", "fixture", BenchmarkAudioSpec{
		Duration: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	utterance := fixture.Utterance
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		var body struct {
			ModelID  string `json:"modelId"`
			Language string `json:"language"`
			Audio    struct {
				ContentType string `json:"contentType"`
				Data        string `json:"data"`
			} `json:"audio"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.ModelID != "tiny-en" || body.Language != "en-GB" || body.Audio.ContentType != "audio/wav" {
			t.Fatalf("unexpected body: %+v", body)
		}
		raw, err := base64.StdEncoding.DecodeString(body.Audio.Data)
		if err != nil || string(raw[:4]) != "RIFF" {
			t.Fatalf("expected WAV payload, len=%d err=%v", len(raw), err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(
				`{"text":"turn on the lights","providerId":"go-whisper","modelId":"tiny-en","language":"en-GB","durationMs":42}`,
			),
		)
	}))
	defer server.Close()

	result, err := (HTTPJSONSTTProvider{
		ProviderID: "local-stt",
		Endpoint:   server.URL,
		ModelID:    "tiny-en",
		Language:   "en-GB",
	}).Transcribe(t.Context(), utterance)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "turn on the lights" ||
		result.ProviderID != "go-whisper" ||
		result.ModelID != "tiny-en" ||
		result.Language != "en-GB" ||
		result.Duration != 42*time.Millisecond {
		t.Fatalf("unexpected result: %+v", result)
	}
}
