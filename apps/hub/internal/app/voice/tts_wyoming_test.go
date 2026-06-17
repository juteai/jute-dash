package voice

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestWyomingTTSProviderSynthesizesPlayableAudio(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	provider := WyomingTTSProvider{
		ProviderID: "local-tts",
		Endpoint:   "tcp://127.0.0.1:10200",
		VoiceID:    "amy",
		Locale:     "en-GB",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}

	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		line, err := readLine(server)
		if err != nil {
			errc <- err
			return
		}
		if !strings.Contains(line, `"type":"synthesize"`) ||
			!strings.Contains(line, `"text":"The kitchen lights are on."`) ||
			!strings.Contains(line, `"name":"amy"`) ||
			!strings.Contains(line, `"language":"en-GB"`) {
			errc <- errors.New("unexpected synthesize request: " + line)
			return
		}
		if _, err := server.Write(
			[]byte(`{"type":"audio-start","data":{"rate":16000,"width":2,"channels":1}}` + "\n"),
		); err != nil {
			errc <- err
			return
		}
		if err := writeWyomingEvent(server, wyomingEvent{
			Type: WyomingEventAudioChunk,
			Data: map[string]any{
				"rate":     16000,
				"width":    2,
				"channels": 1,
			},
			Payload: []byte{1, 2, 3, 4},
		}); err != nil {
			errc <- err
			return
		}
		_, err = server.Write([]byte(`{"type":"audio-stop"}` + "\n"))
		errc <- err
	}()

	result, err := provider.Synthesize(context.Background(), TTSRequest{
		Text: "The kitchen lights are on.",
	})
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
	if string(result.Audio) != string([]byte{1, 2, 3, 4}) ||
		result.ProviderID != "local-tts" ||
		result.VoiceID != "amy" ||
		result.Locale != "en-GB" ||
		result.ContentType != "audio/pcm" ||
		result.SampleRate != 16000 ||
		result.SampleWidth != 2 ||
		result.Channels != 1 ||
		result.PlaybackKind != "audio" ||
		result.Duration != 125*time.Microsecond {
		t.Fatalf("unexpected TTS result: %+v", result)
	}
}

func TestWyomingTTSProviderRejectsEmptyAudio(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	provider := WyomingTTSProvider{
		Endpoint: "tcp://127.0.0.1:10200",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		if _, err := readLine(server); err != nil {
			errc <- err
			return
		}
		_, err := server.Write([]byte(`{"type":"audio-stop"}` + "\n"))
		errc <- err
	}()

	if _, err := provider.Synthesize(context.Background(), TTSRequest{Text: "hello"}); err == nil {
		t.Fatalf("expected empty audio error")
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
}

func TestWyomingTTSProviderMasksTransportFailureDetails(t *testing.T) {
	provider := WyomingTTSProvider{
		ProviderID: "local-tts",
		Endpoint:   "tcp://127.0.0.1:10200",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("dial tcp 127.0.0.1:10200: token=secret unavailable")
		},
	}

	_, err := provider.Synthesize(context.Background(), TTSRequest{Text: "hello"})

	if err == nil || err.Error() != "wyoming TTS provider unavailable" {
		t.Fatalf("expected safe provider error, got %v", err)
	}
	if strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "token=secret") {
		t.Fatalf("TTS provider error leaked transport details: %v", err)
	}
}

func TestWyomingTTSProviderMasksAudioReadFailureDetails(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	provider := WyomingTTSProvider{
		ProviderID: "local-tts",
		Endpoint:   "tcp://127.0.0.1:10200",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		if _, err := readLine(server); err != nil {
			errc <- err
			return
		}
		_, _ = server.Write([]byte(`{"type":"audio-start","data":{"secret":"token=secret"}`))
		errc <- nil
	}()

	_, err := provider.Synthesize(context.Background(), TTSRequest{Text: "hello"})

	if fixtureErr := <-errc; fixtureErr != nil {
		t.Fatalf("fixture server failed: %v", fixtureErr)
	}
	if err == nil || err.Error() != "wyoming TTS provider unavailable" {
		t.Fatalf("expected safe provider error, got %v", err)
	}
	if strings.Contains(err.Error(), "token=secret") || strings.Contains(err.Error(), "audio-start") {
		t.Fatalf("TTS provider error leaked audio response details: %v", err)
	}
}

func TestWyomingTTSProviderHealthStates(t *testing.T) {
	if got := (WyomingTTSProvider{Endpoint: "https://example.com"}).Health(
		context.Background(),
	); got.Status != "misconfigured" {
		t.Fatalf("expected misconfigured health, got %+v", got)
	}
	if got := (WyomingTTSProvider{
		Endpoint: "tcp://127.0.0.1:10200",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("offline")
		},
	}).Health(context.Background()); got.Status != "offline" {
		t.Fatalf("expected offline health, got %+v", got)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	if got := (WyomingTTSProvider{
		Endpoint: "tcp://127.0.0.1:10200",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}).Health(context.Background()); got.Status != "available" {
		t.Fatalf("expected available health, got %+v", got)
	}
}

func TestWyomingTTSProviderDoesNotWriteAudioBeforeRequest(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	provider := WyomingTTSProvider{
		Endpoint: "tcp://127.0.0.1:10200",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		line, err := readLine(server)
		if err != nil {
			errc <- err
			return
		}
		if strings.Contains(line, "payload_length") {
			errc <- errors.New("synthesize request unexpectedly included audio payload")
			return
		}
		_, err = server.Write([]byte(`{"type":"audio-stop"}` + "\n"))
		errc <- err
	}()
	_, _ = provider.Synthesize(context.Background(), TTSRequest{Text: "hello"})
	if err := <-errc; err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("fixture server failed: %v", err)
	}
}
