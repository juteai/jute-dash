package voice

import (
	"bufio"
	"context"
	"errors"
	"net"
	"time"
)

const (
	WyomingEventSynthesize        = "synthesize"
	WyomingEventSynthesizeStopped = "synthesize-stopped"
)

var errWyomingTTSProviderUnavailable = errors.New("wyoming TTS provider unavailable")

type TTSAudioResult struct {
	Audio        []byte        `json:"-"`
	ProviderID   string        `json:"providerId"`
	VoiceID      string        `json:"voiceId,omitempty"`
	Locale       string        `json:"locale,omitempty"`
	ContentType  string        `json:"contentType"`
	SampleRate   int           `json:"sampleRate,omitempty"`
	SampleWidth  int           `json:"sampleWidth,omitempty"`
	Channels     int           `json:"channels,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	PlaybackKind string        `json:"playbackKind"`
}

type WyomingTTSProvider struct {
	ProviderID  string
	Endpoint    string
	VoiceID     string
	Locale      string
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
}

func (p WyomingTTSProvider) Health(ctx context.Context) ProviderHealth {
	network, address, err := wyomingTCPAddress(p.Endpoint)
	if err != nil {
		return ProviderHealth{Status: "misconfigured", Reason: "invalid_endpoint"}
	}
	dial := p.DialContext
	if dial == nil {
		dial = (&net.Dialer{Timeout: 2 * time.Second}).DialContext
	}
	conn, err := dial(ctx, network, address)
	if err != nil {
		return ProviderHealth{Status: "offline", Reason: "unreachable"}
	}
	_ = conn.Close()
	return ProviderHealth{Status: "available"}
}

func (p WyomingTTSProvider) Synthesize(ctx context.Context, req TTSRequest) (TTSAudioResult, error) {
	text := req.Text
	if text == "" {
		return TTSAudioResult{}, errors.New("TTS text is required")
	}
	network, address, err := wyomingTCPAddress(p.Endpoint)
	if err != nil {
		return TTSAudioResult{}, err
	}
	dial := p.DialContext
	if dial == nil {
		dial = (&net.Dialer{Timeout: 3 * time.Second}).DialContext
	}
	conn, err := dial(ctx, network, address)
	if err != nil {
		return TTSAudioResult{}, errWyomingTTSProviderUnavailable
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	voiceID := safeIdentifier(req.VoiceID)
	if voiceID == "" {
		voiceID = safeIdentifier(p.VoiceID)
	}
	locale := safeIdentifier(req.Locale)
	if locale == "" {
		locale = safeIdentifier(p.Locale)
	}
	data := map[string]any{
		"text":        text,
		"text_format": "text",
	}
	voice := map[string]any{}
	if voiceID != "" {
		voice["name"] = voiceID
	}
	if locale != "" {
		voice["language"] = locale
	}
	if len(voice) > 0 {
		data["voice"] = voice
	}
	if err := writeWyomingEvent(conn, wyomingEvent{
		Type: WyomingEventSynthesize,
		Data: data,
	}); err != nil {
		return TTSAudioResult{}, errWyomingTTSProviderUnavailable
	}

	reader := bufio.NewReader(conn)
	var result TTSAudioResult
	result.ProviderID = safeIdentifier(p.ProviderID)
	result.VoiceID = voiceID
	result.Locale = locale
	result.ContentType = "audio/pcm"
	result.PlaybackKind = "audio"
	for {
		event, err := readWyomingEvent(reader)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return TTSAudioResult{}, ctx.Err()
			}
			return TTSAudioResult{}, errWyomingTTSProviderUnavailable
		}
		switch event.Type {
		case WyomingEventAudioStart:
			result.SampleRate = intNumber(event.Data["rate"])
			result.SampleWidth = intNumber(event.Data["width"])
			result.Channels = intNumber(event.Data["channels"])
		case WyomingEventAudioChunk:
			if result.SampleRate == 0 {
				result.SampleRate = intNumber(event.Data["rate"])
			}
			if result.SampleWidth == 0 {
				result.SampleWidth = intNumber(event.Data["width"])
			}
			if result.Channels == 0 {
				result.Channels = intNumber(event.Data["channels"])
			}
			result.Audio = append(result.Audio, event.Payload...)
		case WyomingEventAudioStop, WyomingEventSynthesizeStopped:
			if len(result.Audio) == 0 {
				return TTSAudioResult{}, errors.New("wyoming TTS returned no audio")
			}
			result.Duration = audioDuration(result.Audio, result.SampleRate, result.SampleWidth, result.Channels)
			return result, nil
		}
	}
}

func intNumber(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func audioDuration(audio []byte, rate, width, channels int) time.Duration {
	if rate <= 0 || width <= 0 || channels <= 0 {
		return 0
	}
	samples := len(audio) / (width * channels)
	return time.Duration(samples) * time.Second / time.Duration(rate)
}
