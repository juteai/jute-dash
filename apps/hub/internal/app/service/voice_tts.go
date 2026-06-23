package service

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	TTSPolicyVisualOnlySensitive = "visual_only_sensitive"
	TTSPolicyAskBeforeSensitive  = "ask_before_sensitive"
	TTSPolicySpeakAll            = "speak_all"

	TTSActionSpeak = "speak"

	TTSStateIdle         = "idle"
	TTSStateSynthesizing = "synthesizing"
	TTSStatePlayback     = "playback"
	TTSStateStopped      = "stopped"
	TTSStateCompleted    = "completed"
	TTSStateVisualOnly   = "visual_only"
	TTSStateFailed       = "failed"
)

var (
	ttsFencePattern        = regexp.MustCompile("(?s)```.*?```")
	ttsImagePattern        = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	ttsLinkPattern         = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	ttsInlineCodePattern   = regexp.MustCompile("`([^`]*)`")
	ttsListMarkerPattern   = regexp.MustCompile(`(?m)^\s*(?:[-*+]\s+|\d+[.)]\s+)`)
	ttsHeadingQuotePattern = regexp.MustCompile(`(?m)^\s{0,3}(?:#{1,6}\s*|>\s*)`)
	ttsWhitespacePattern   = regexp.MustCompile(`\s+`)
	ttsURLPattern          = regexp.MustCompile(`https?://\S+`)
	ttsReasoningPattern    = regexp.MustCompile(
		"(?is)<(?:think|thinking|reasoning|scratchpad)>.*?</(?:think|thinking|reasoning|scratchpad)>" +
			"|```(?:thinking|reasoning|scratchpad)\\s+.*?```",
	)
)

type TTSRequest struct {
	Text           string `json:"text"`
	ProviderID     string `json:"providerId,omitempty"`
	VoiceID        string `json:"voiceId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	TurnID         string `json:"turnId,omitempty"`
	Locale         string `json:"locale,omitempty"`
	Sensitive      bool   `json:"sensitive,omitempty"`
}

type TTSStopRequest struct {
	ConversationID string `json:"conversationId,omitempty"`
	TurnID         string `json:"turnId,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

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

func DecodeTTSRequest(r io.Reader) (TTSRequest, error) {
	var req TTSRequest
	if err := decodeTTSJSON(r, &req); err != nil {
		return TTSRequest{}, err
	}
	return req, nil
}

func DecodeTTSStopRequest(r io.Reader) (TTSStopRequest, error) {
	var req TTSStopRequest
	if err := decodeTTSJSON(r, &req); err != nil {
		return TTSStopRequest{}, err
	}
	return req, nil
}

func decodeTTSJSON(r io.Reader, dst any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON data")
	}
	return nil
}

func effectiveTTSRequest(req TTSRequest, settings Settings) TTSRequest {
	if strings.TrimSpace(req.ProviderID) == "" {
		req.ProviderID = settings.TTSProviderID
	}
	if strings.TrimSpace(req.VoiceID) == "" {
		req.VoiceID = settings.TTSVoiceID
	}
	if strings.TrimSpace(req.Locale) == "" {
		req.Locale = settings.TTSLocale
	}
	return req
}

// ponytail: regex scrub, swap for a markdown AST only if speech needs full CommonMark fidelity.
func speechText(value string) string {
	value = stripReasoningForSpeech(value)
	value = ttsFencePattern.ReplaceAllString(value, " Code omitted. ")
	value = ttsImagePattern.ReplaceAllString(value, "$1")
	value = ttsLinkPattern.ReplaceAllString(value, "$1")
	value = ttsInlineCodePattern.ReplaceAllString(value, "$1")
	value = ttsURLPattern.ReplaceAllString(value, " link ")
	value = ttsHeadingQuotePattern.ReplaceAllString(value, "")
	value = ttsListMarkerPattern.ReplaceAllString(value, "")
	value = strings.NewReplacer(
		"**", "",
		"__", "",
		"*", "",
		"_", "",
		"~~", "",
		"`", "",
		"|", " ",
		"[ ]", "",
		"[x]", "",
		"[X]", "",
	).Replace(value)
	return strings.TrimSpace(ttsWhitespacePattern.ReplaceAllString(value, " "))
}

// SpeechText returns provider-safe text for spoken output.
func SpeechText(value string) string {
	return speechText(value)
}

func stripReasoningForSpeech(value string) string {
	value = strings.TrimSpace(value)
	value = ttsReasoningPattern.ReplaceAllString(value, "")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lines := ttsSplitLines(value)
	for len(lines) > 1 && looksLikeSpokenReasoning(lines[0]) {
		lines = lines[1:]
	}
	value = strings.TrimSpace(strings.Join(lines, "\n"))
	paragraphs := ttsSplitParagraphs(value)
	for len(paragraphs) > 1 && looksLikeSpokenReasoning(paragraphs[0]) {
		paragraphs = paragraphs[1:]
	}
	if len(paragraphs) == 1 && looksLikeSpokenReasoning(paragraphs[0]) {
		return ""
	}
	return strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
}

func ttsSplitLines(value string) []string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	raw := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func ttsSplitParagraphs(value string) []string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	raw := strings.Split(normalized, "\n\n")
	paragraphs := make([]string, 0, len(raw))
	for _, paragraph := range raw {
		if trimmed := strings.TrimSpace(paragraph); trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	return paragraphs
}

func looksLikeSpokenReasoning(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(lower, "okay, the user") ||
		strings.HasPrefix(lower, "the user ") ||
		strings.HasPrefix(lower, "we need ") ||
		strings.HasPrefix(lower, "i need ") ||
		strings.HasPrefix(lower, "i should ") ||
		strings.HasPrefix(lower, "let me ") ||
		strings.HasPrefix(lower, "everything seems covered") ||
		strings.HasPrefix(lower, "time to format") ||
		strings.HasPrefix(lower, "now i can ") ||
		strings.HasPrefix(lower, "i can now ") {
		return true
	}
	signals := 0
	for _, phrase := range []string{
		"the user",
		"i should",
		"i'll",
		"i will",
		"i can",
		"no need to",
		"need to call",
		"call any function",
		"call tools",
		"use the tool",
		"tool choice",
		"final answer",
		"format the response",
		"extracted data",
	} {
		if strings.Contains(lower, phrase) {
			signals++
		}
	}
	return signals >= 2
}

type TTSActionResponse struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	State          string `json:"state"`
	ProviderID     string `json:"providerId,omitempty"`
	VoiceID        string `json:"voiceId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	TurnID         string `json:"turnId,omitempty"`
	VisualOnly     bool   `json:"visualOnly"`
	Reason         string `json:"reason,omitempty"`
	PlaybackKind   string `json:"playbackKind,omitempty"`
	ContentType    string `json:"contentType,omitempty"`
	SampleRate     int    `json:"sampleRate,omitempty"`
	SampleWidth    int    `json:"sampleWidth,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	AudioBytes     int    `json:"audioBytes,omitempty"`
	DurationMs     int64  `json:"durationMs,omitempty"`
	AudioURL       string `json:"audioUrl,omitempty"`
}

type TTSRuntime struct {
	mu      sync.Mutex
	current TTSActionResponse
	cancel  func()
}

func NewTTSRuntime() *TTSRuntime {
	return &TTSRuntime{}
}

func (r *TTSRuntime) Begin(action string, req TTSRequest, settings Settings, cancels ...func()) TTSActionResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancel = nil
	if len(cancels) > 0 {
		r.cancel = cancels[0]
	}
	providerID := strings.TrimSpace(req.ProviderID)
	if providerID == "" {
		providerID = settings.TTSProviderID
	}
	voiceID := strings.TrimSpace(req.VoiceID)
	if voiceID == "" {
		voiceID = settings.TTSVoiceID
	}
	response := TTSActionResponse{
		ID:             newID("tts"),
		Action:         action,
		State:          TTSStateSynthesizing,
		ProviderID:     safeIdentifier(providerID),
		VoiceID:        safeIdentifier(voiceID),
		ConversationID: safeIdentifier(req.ConversationID),
		TurnID:         safeIdentifier(req.TurnID),
	}
	r.current = response
	return response
}

func (r *TTSRuntime) Complete(id string) TTSActionResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current.ID == id {
		if r.current.State == TTSStateStopped {
			return r.current
		}
		r.cancel = nil
		r.current.State = TTSStateCompleted
		return r.current
	}
	return TTSActionResponse{ID: id, State: TTSStateCompleted}
}

func (r *TTSRuntime) CompleteWithAudio(id string, audio TTSAudioResult) TTSActionResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	response := r.current
	if response.ID != id {
		response = TTSActionResponse{ID: id}
	} else if response.State == TTSStateStopped {
		return response
	}
	response.State = TTSStateCompleted
	applyTTSAudioResult(&response, audio)
	r.current = response
	r.cancel = nil
	return response
}

func (r *TTSRuntime) Fail(id, reason string) TTSActionResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current.ID == id {
		if r.current.State == TTSStateStopped {
			return r.current
		}
		r.cancel = nil
		r.current.State = TTSStateFailed
		r.current.Reason = sanitizeText(reason)
		return r.current
	}
	return TTSActionResponse{ID: id, State: TTSStateFailed, Reason: sanitizeText(reason)}
}

func (r *TTSRuntime) VisualOnly(id, reason string) TTSActionResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current.ID == id {
		r.cancel = nil
		r.current.State = TTSStateVisualOnly
		r.current.VisualOnly = true
		r.current.Reason = sanitizeText(reason)
		return r.current
	}
	return TTSActionResponse{
		ID:         id,
		State:      TTSStateVisualOnly,
		VisualOnly: true,
		Reason:     sanitizeText(reason),
	}
}

func (r *TTSRuntime) Stop(req TTSStopRequest) TTSActionResponse {
	r.mu.Lock()
	response := r.current
	if response.ID == "" {
		response = TTSActionResponse{
			ID:             newID("tts"),
			Action:         TTSActionSpeak,
			ConversationID: safeIdentifier(req.ConversationID),
			TurnID:         safeIdentifier(req.TurnID),
		}
	}
	response.State = TTSStateStopped
	response.Reason = normalizeStopReason(req.Reason)
	r.current = response
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return response
}

func sensitiveOutput(req TTSRequest) bool {
	if req.Sensitive {
		return true
	}
	text := strings.ToLower(req.Text)
	for _, marker := range []string{
		"password",
		"api key",
		"apikey",
		"secret",
		"token",
		"door code",
		"pin",
		"credential",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func applyTTSAudioResult(response *TTSActionResponse, audio TTSAudioResult) {
	if response.ProviderID == "" {
		response.ProviderID = safeIdentifier(audio.ProviderID)
	}
	if response.VoiceID == "" {
		response.VoiceID = safeIdentifier(audio.VoiceID)
	}
	response.PlaybackKind = safeIdentifier(audio.PlaybackKind)
	response.ContentType = safeIdentifier(audio.ContentType)
	response.SampleRate = audio.SampleRate
	response.SampleWidth = audio.SampleWidth
	response.Channels = audio.Channels
	response.AudioBytes = len(audio.Audio)
	response.DurationMs = audio.Duration.Milliseconds()
}

func speechPolicyAllows(req TTSRequest, settings Settings) (bool, string) {
	if !sensitiveOutput(req) {
		return true, ""
	}
	switch settings.SensitiveOutputPolicy {
	case TTSPolicySpeakAll:
		return true, ""
	case TTSPolicyAskBeforeSensitive:
		return false, "sensitive_output_requires_confirmation"
	default:
		return false, "sensitive_output_visual_only"
	}
}

func normalizeStopReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "user_stop"
	}
	switch reason {
	case "barge_in", "user_stop", "cancel", "timeout":
		return reason
	default:
		return "user_stop"
	}
}
