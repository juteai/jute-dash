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

func TestWyomingSTTProviderSendsAudioAndReturnsTranscriptMetadata(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	start := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)
	provider := WyomingSTTProvider{
		ProviderID: "local-stt",
		Endpoint:   "tcp://127.0.0.1:10300",
		ModelID:    "small-en",
		Language:   "en-GB",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		for _, expected := range []string{
			`"type":"transcribe"`,
			`"type":"audio-start"`,
			`"type":"audio-chunk"`,
			`"type":"audio-chunk"`,
			`"type":"audio-stop"`,
		} {
			line, err := readLine(server)
			if err != nil {
				errc <- err
				return
			}
			if !strings.Contains(line, expected) {
				errc <- errors.New("unexpected wyoming event: " + line)
				return
			}
			if strings.Contains(line, "turn on the kitchen lights") {
				errc <- errors.New("transcript text leaked in audio request")
				return
			}
			if strings.Contains(line, `"payload_length":2`) {
				payload := make([]byte, 2)
				if _, err := io.ReadFull(server, payload); err != nil {
					errc <- err
					return
				}
			}
		}
		_, err := server.Write(
			[]byte(`{"type":"transcript","data":{"text":"turn on the kitchen lights","language":"en-GB"}}` + "\n"),
		)
		errc <- err
	}()

	result, err := provider.Transcribe(context.Background(), CapturedUtterance{
		StartedAt: start,
		EndedAt:   start.Add(200 * time.Millisecond),
		Frames: []AudioFrame{
			fixtureFrame(start, 0, 3),
			fixtureFrame(start, 100*time.Millisecond, 4),
		},
	})
	if err != nil {
		t.Fatalf("Transcribe failed: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
	if result.Text != "turn on the kitchen lights" ||
		result.ProviderID != "local-stt" ||
		result.ModelID != "small-en" ||
		result.Language != "en-GB" ||
		result.Duration != 200*time.Millisecond {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestWyomingSTTProviderSupportsStreamingTranscriptChunks(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	provider := WyomingSTTProvider{
		ProviderID: "local-stt",
		Endpoint:   "tcp://127.0.0.1:10300",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		for range 4 {
			line, err := readLine(server)
			if err != nil {
				errc <- err
				return
			}
			if strings.Contains(line, `"payload_length":2`) {
				payload := make([]byte, 2)
				if _, err := io.ReadFull(server, payload); err != nil {
					errc <- err
					return
				}
			}
		}
		_, err := server.Write([]byte(
			`{"type":"transcript-start","data":{"language":"en"}}` + "\n" +
				`{"type":"transcript-chunk","data":{"text":"turn on "}}` + "\n" +
				`{"type":"transcript-chunk","data":{"text":"the lights"}}` + "\n" +
				`{"type":"transcript-stop"}` + "\n",
		))
		errc <- err
	}()

	start := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)
	result, err := provider.Transcribe(context.Background(), CapturedUtterance{
		StartedAt: start,
		EndedAt:   start.Add(100 * time.Millisecond),
		Frames:    []AudioFrame{fixtureFrame(start, 0, 3)},
	})
	if err != nil {
		t.Fatalf("Transcribe failed: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("fixture server failed: %v", err)
	}
	if result.Text != "turn on the lights" || result.Language != "en" {
		t.Fatalf("unexpected streaming result: %+v", result)
	}
}

func TestWyomingSTTProviderMasksTransportFailureDetails(t *testing.T) {
	provider := WyomingSTTProvider{
		ProviderID: "local-stt",
		Endpoint:   "tcp://127.0.0.1:10300",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("dial tcp 127.0.0.1:10300: token=secret unavailable")
		},
	}

	_, err := provider.Transcribe(context.Background(), fixtureSTTTurnUtterance())

	if err == nil || err.Error() != "wyoming STT provider unavailable" {
		t.Fatalf("expected safe provider error, got %v", err)
	}
	if strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "token=secret") {
		t.Fatalf("STT provider error leaked transport details: %v", err)
	}
}

func TestWyomingSTTProviderMasksTranscriptReadFailureDetails(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	provider := WyomingSTTProvider{
		ProviderID: "local-stt",
		Endpoint:   "tcp://127.0.0.1:10300",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}
	errc := make(chan error, 1)
	go func() {
		defer server.Close()
		for range 4 {
			line, err := readLine(server)
			if err != nil {
				errc <- err
				return
			}
			if strings.Contains(line, `"payload_length":2`) {
				payload := make([]byte, 2)
				if _, err := io.ReadFull(server, payload); err != nil {
					errc <- err
					return
				}
			}
		}
		_, _ = server.Write([]byte(`{"type":"transcript","data":{"text":"token=secret"}`))
		errc <- nil
	}()

	_, err := provider.Transcribe(context.Background(), fixtureSTTTurnUtterance())

	if fixtureErr := <-errc; fixtureErr != nil {
		t.Fatalf("fixture server failed: %v", fixtureErr)
	}
	if err == nil || err.Error() != "wyoming STT provider unavailable" {
		t.Fatalf("expected safe provider error, got %v", err)
	}
	if strings.Contains(err.Error(), "token=secret") || strings.Contains(err.Error(), "transcript") {
		t.Fatalf("STT provider error leaked transcript details: %v", err)
	}
}

func TestWyomingSTTProviderHealthStates(t *testing.T) {
	if got := (WyomingSTTProvider{Endpoint: "https://example.com"}).Health(
		context.Background(),
	); got.Status != "misconfigured" {
		t.Fatalf("expected misconfigured health, got %+v", got)
	}
	if got := (WyomingSTTProvider{
		Endpoint: "tcp://127.0.0.1:10300",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("offline")
		},
	}).Health(context.Background()); got.Status != "offline" {
		t.Fatalf("expected offline health, got %+v", got)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	if got := (WyomingSTTProvider{
		Endpoint: "tcp://127.0.0.1:10300",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return client, nil
		},
	}).Health(context.Background()); got.Status != "available" {
		t.Fatalf("expected available health, got %+v", got)
	}
}
