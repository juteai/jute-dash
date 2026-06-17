package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var satelliteEnvRef = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

type SatelliteRuntimeConfig struct {
	HubURL         string        `json:"hubUrl"`
	SatelliteID    string        `json:"satelliteId"`
	AuthSecretEnv  string        `json:"authSecretEnv"`
	FixtureWAV     string        `json:"fixtureWav"`
	Transcript     string        `json:"transcript"`
	ConversationID string        `json:"conversationId,omitempty"`
	Version        string        `json:"version,omitempty"`
	WakeModelID    string        `json:"wakeModelId,omitempty"`
	Timeout        time.Duration `json:"-"`
}

type SatelliteRuntimeResult struct {
	SatelliteID       string   `json:"satelliteId"`
	ConversationID    string   `json:"conversationId,omitempty"`
	FollowupActive    bool     `json:"followupActive,omitempty"`
	FollowupExpiresAt string   `json:"followupExpiresAt,omitempty"`
	FollowupTurns     int      `json:"followupTurns,omitempty"`
	FollowupMaxTurns  int      `json:"followupMaxTurns,omitempty"`
	EventsSent        int      `json:"eventsSent"`
	TranscriptSent    bool     `json:"transcriptSent"`
	Diagnostics       []string `json:"diagnostics,omitempty"`
}

type SatelliteRuntime struct {
	HTTPClient *http.Client
	Now        func() time.Time
}

func DecodeSatelliteRuntimeConfig(raw []byte) (SatelliteRuntimeConfig, error) {
	var cfg SatelliteRuntimeConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return SatelliteRuntimeConfig{}, fmt.Errorf("decode satellite config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return SatelliteRuntimeConfig{}, errors.New("decode satellite config: trailing JSON data")
	}
	return cfg, nil
}

func LoadSatelliteRuntimeConfig(path string) (SatelliteRuntimeConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SatelliteRuntimeConfig{}, fmt.Errorf("read satellite config: %w", err)
	}
	return DecodeSatelliteRuntimeConfig(raw)
}

func (r SatelliteRuntime) RunFixture(ctx context.Context, cfg SatelliteRuntimeConfig) (SatelliteRuntimeResult, error) {
	result := SatelliteRuntimeResult{SatelliteID: strings.TrimSpace(cfg.SatelliteID)}
	cfg, authProof, err := normalizeSatelliteRuntimeConfig(cfg)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, satelliteDiagnostic(err))
		return result, err
	}
	result.ConversationID = cfg.ConversationID
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	utterance, err := loadSatelliteFixture(cfg.FixtureWAV)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, "microphone_unavailable")
		return result, err
	}
	if strings.TrimSpace(cfg.WakeModelID) == "" {
		result.Diagnostics = append(result.Diagnostics, "wake_provider_unavailable")
		return result, errors.New("wake_provider_unavailable")
	}

	client := satelliteRuntimeHubClient{
		baseURL:     cfg.HubURL,
		satelliteID: cfg.SatelliteID,
		authProof:   authProof,
		httpClient:  r.httpClient(),
		now:         r.now,
	}
	if err := client.sendEvent(ctx, SatelliteEventRequest{
		Type:        EventVoiceSatelliteHealthChanged,
		Health:      "ready",
		Version:     cfg.Version,
		WakeModelID: cfg.WakeModelID,
	}); err != nil {
		result.Diagnostics = append(result.Diagnostics, satelliteDiagnostic(err))
		return result, err
	}
	result.EventsSent++

	emitter := &satelliteRuntimeEmitter{ctx: ctx, client: client, result: &result}
	service := NewLocalVoiceService(
		VoiceServiceConfig{
			Enabled:         true,
			Muted:           false,
			DeviceID:        cfg.SatelliteID,
			PreRoll:         120 * time.Millisecond,
			SilenceDuration: 80 * time.Millisecond,
			MaxUtterance:    10 * time.Second,
		},
		satelliteFixtureCapture{frames: utterance.Frames},
		satelliteFixtureVAD{},
		emitter,
		func(CapturedUtterance) {
			if strings.TrimSpace(cfg.Transcript) == "" {
				return
			}
			response, err := client.sendFinalTranscript(ctx, cfg.Transcript, cfg.ConversationID)
			if err != nil {
				emitter.addDiagnostic(satelliteDiagnostic(err))
				return
			}
			result.applyFinalTranscriptResponse(response)
			result.TranscriptSent = true
		},
	)
	if err := service.Start(ctx); err != nil {
		result.Diagnostics = append(result.Diagnostics, satelliteDiagnostic(err))
		return result, err
	}
	waitLocalVoiceService(service)
	result.Diagnostics = append(result.Diagnostics, emitter.diagnostics...)
	if len(result.Diagnostics) > 0 && !result.TranscriptSent {
		return result, errors.New(result.Diagnostics[0])
	}
	return result, nil
}

func (r SatelliteRuntime) httpClient() *http.Client {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (r SatelliteRuntime) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func normalizeSatelliteRuntimeConfig(cfg SatelliteRuntimeConfig) (SatelliteRuntimeConfig, string, error) {
	cfg.HubURL = strings.TrimSpace(cfg.HubURL)
	cfg.SatelliteID = strings.TrimSpace(cfg.SatelliteID)
	cfg.AuthSecretEnv = strings.TrimSpace(cfg.AuthSecretEnv)
	cfg.FixtureWAV = strings.TrimSpace(cfg.FixtureWAV)
	cfg.Transcript = sanitizeText(cfg.Transcript)
	cfg.ConversationID = safeIdentifier(cfg.ConversationID)
	cfg.Version = safeIdentifier(cfg.Version)
	cfg.WakeModelID = safeIdentifier(cfg.WakeModelID)
	if cfg.HubURL == "" {
		return cfg, "", errors.New("hub_unreachable")
	}
	if _, err := url.ParseRequestURI(cfg.HubURL); err != nil {
		return cfg, "", errors.New("hub_unreachable")
	}
	if cfg.SatelliteID == "" {
		return cfg, "", errors.New("satellite_id_required")
	}
	if !satelliteEnvRef.MatchString(cfg.AuthSecretEnv) {
		return cfg, "", errors.New("credential_reference_required")
	}
	authProof, ok := os.LookupEnv(cfg.AuthSecretEnv)
	if !ok || strings.TrimSpace(authProof) == "" {
		return cfg, "", errors.New("credential_unavailable")
	}
	if cfg.FixtureWAV == "" {
		return cfg, "", errors.New("microphone_unavailable")
	}
	return cfg, strings.TrimSpace(authProof), nil
}

func (r *SatelliteRuntimeResult) applyFinalTranscriptResponse(response satelliteFinalTranscriptResponse) {
	conversationID := safeIdentifier(response.Conversation.Conversation.ID)
	if conversationID != "" {
		r.ConversationID = conversationID
	}
	r.FollowupActive = response.Followup.Active
	r.FollowupExpiresAt = safeIdentifier(response.Followup.ExpiresAt)
	r.FollowupTurns = response.Followup.Turns
	r.FollowupMaxTurns = response.Followup.MaxTurns
}

func loadSatelliteFixture(path string) (CapturedUtterance, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CapturedUtterance{}, fmt.Errorf("read fixture audio: %w", err)
	}
	utterance, err := DecodeBenchmarkWAV(raw, time.Time{})
	if err != nil {
		return CapturedUtterance{}, fmt.Errorf("decode fixture audio: %w", err)
	}
	return utterance, nil
}

type satelliteRuntimeHubClient struct {
	baseURL     string
	satelliteID string
	authProof   string
	httpClient  *http.Client
	now         func() time.Time
}

type satelliteFinalTranscriptRequest struct {
	Text           string `json:"text"`
	ConversationID string `json:"conversationId,omitempty"`
}

type satelliteFinalTranscriptResponse struct {
	Conversation struct {
		Conversation struct {
			ID string `json:"id"`
		} `json:"conversation"`
	} `json:"conversation"`
	Followup struct {
		Active    bool   `json:"active"`
		ExpiresAt string `json:"expiresAt,omitempty"`
		Turns     int    `json:"turns"`
		MaxTurns  int    `json:"maxTurns"`
	} `json:"followup"`
}

func (c satelliteRuntimeHubClient) sendEvent(ctx context.Context, req SatelliteEventRequest) error {
	_, err := c.postJSON(ctx, "/api/v1/voice/satellites/"+url.PathEscape(c.satelliteID)+"/events", req)
	return err
}

func (c satelliteRuntimeHubClient) sendFinalTranscript(
	ctx context.Context,
	text string,
	conversationID string,
) (satelliteFinalTranscriptResponse, error) {
	raw, err := c.postJSON(
		ctx,
		"/api/v1/voice/satellites/"+url.PathEscape(c.satelliteID)+"/transcripts/final",
		satelliteFinalTranscriptRequest{
			Text:           sanitizeText(text),
			ConversationID: safeIdentifier(conversationID),
		},
	)
	if err != nil {
		return satelliteFinalTranscriptResponse{}, err
	}
	if strings.TrimSpace(string(raw)) == "" {
		return satelliteFinalTranscriptResponse{}, nil
	}
	var response satelliteFinalTranscriptResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return satelliteFinalTranscriptResponse{}, errors.New("hub_response_invalid")
	}
	return response, nil
}

func (c satelliteRuntimeHubClient) postJSON(ctx context.Context, path string, body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+path,
		bytes.NewReader(raw),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Jute-Satellite-Auth", c.authProof)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("hub_unreachable")
	}
	defer resp.Body.Close()
	if clockSkewed(c.now, resp.Header.Get("Date")) {
		return nil, errors.New("clock_skew")
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return responseBody, nil
	case http.StatusForbidden, http.StatusUnauthorized:
		return nil, errors.New("auth_failed")
	default:
		return nil, fmt.Errorf("hub_status_%d", resp.StatusCode)
	}
}

func clockSkewed(now func() time.Time, rawDate string) bool {
	rawDate = strings.TrimSpace(rawDate)
	if rawDate == "" {
		return false
	}
	hubTime, err := http.ParseTime(rawDate)
	if err != nil {
		return false
	}
	if now == nil {
		now = time.Now
	}
	diff := now().UTC().Sub(hubTime.UTC())
	if diff < 0 {
		diff = -diff
	}
	return diff > 5*time.Minute
}

type satelliteRuntimeEmitter struct {
	ctx         context.Context
	client      satelliteRuntimeHubClient
	result      *SatelliteRuntimeResult
	diagnostics []string
}

func (e *satelliteRuntimeEmitter) EmitVoiceStateChanged(_ string, payload VoiceStatePayload) VoiceEvent {
	err := e.client.sendEvent(e.ctx, SatelliteEventRequest{
		Type:   EventVoiceSatelliteStateChanged,
		State:  payload.State,
		Health: payload.ServiceStatus,
	})
	if err != nil {
		e.addDiagnostic(satelliteDiagnostic(err))
		return VoiceEvent{}
	}
	e.result.EventsSent++
	return VoiceEvent{Type: EventVoiceSatelliteStateChanged, DeviceID: e.client.satelliteID}
}

func (e *satelliteRuntimeEmitter) EmitVoiceWakeDetected(string, string) VoiceEvent {
	err := e.client.sendEvent(e.ctx, SatelliteEventRequest{
		Type:  EventVoiceSatelliteWakeDetected,
		State: WakeStateDetected,
	})
	if err != nil {
		e.addDiagnostic(satelliteDiagnostic(err))
		return VoiceEvent{}
	}
	e.result.EventsSent++
	return VoiceEvent{Type: EventVoiceSatelliteWakeDetected, DeviceID: e.client.satelliteID}
}

func (e *satelliteRuntimeEmitter) addDiagnostic(code string) {
	if code == "" {
		return
	}
	for _, existing := range e.diagnostics {
		if existing == code {
			return
		}
	}
	e.diagnostics = append(e.diagnostics, code)
}

type satelliteFixtureCapture struct {
	frames []AudioFrame
}

func (c satelliteFixtureCapture) Capture(ctx context.Context) (<-chan AudioFrame, <-chan error) {
	frames := make(chan AudioFrame)
	errs := make(chan error)
	go func() {
		defer close(frames)
		defer close(errs)
		for _, frame := range c.frames {
			select {
			case <-ctx.Done():
				return
			case frames <- frame:
			}
		}
	}()
	return frames, errs
}

type satelliteFixtureVAD struct{}

func (satelliteFixtureVAD) Speech(frame AudioFrame) bool {
	for _, value := range frame.PCM {
		if value != 0 {
			return true
		}
	}
	return false
}

func waitLocalVoiceService(service *LocalVoiceService) {
	for {
		service.mu.RLock()
		done := service.finished == nil
		service.mu.RUnlock()
		if done {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func satelliteDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "auth_failed"):
		return "auth_failed"
	case strings.Contains(message, "hub_unreachable"):
		return "hub_unreachable"
	case strings.Contains(message, "credential"):
		return "credential_unavailable"
	case strings.Contains(message, "wake"):
		return "wake_provider_unavailable"
	case strings.Contains(message, "clock"):
		return "clock_skew"
	case strings.Contains(message, "audio"), strings.Contains(message, "microphone"):
		return "microphone_unavailable"
	default:
		return safeIdentifier(message)
	}
}
