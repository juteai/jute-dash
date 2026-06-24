package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	ProviderKindWakeWord = "wake-word"
	ProviderKindSTT      = "stt"
	ProviderKindTTS      = "tts"
)

type ProviderManifest struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Kind         string               `json:"kind"`
	Transport    TransportManifest    `json:"transport"`
	Capabilities ProviderCapabilities `json:"capabilities"`
	Credentials  []CredentialManifest `json:"credentials"`
	WakeWord     WakeWordManifest     `json:"wakeWord,omitempty"`
	TTS          TTSManifest          `json:"tts,omitempty"`
}

type TransportManifest struct {
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type CredentialManifest struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Source   string `json:"source"`
	Env      string `json:"env,omitempty"`
	Required bool   `json:"required"`
}

type WakeWordManifest struct {
	DefaultModelID string                  `json:"defaultModelId"`
	Phrase         string                  `json:"phrase,omitempty"`
	Languages      []string                `json:"languages,omitempty"`
	Sensitivity    float64                 `json:"sensitivity,omitempty"`
	Models         []WakeWordModelManifest `json:"models,omitempty"`
}

type WakeWordModelManifest struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Phrase      string   `json:"phrase,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	Sensitivity float64  `json:"sensitivity,omitempty"`
}

type TTSManifest struct {
	DefaultVoiceID string             `json:"defaultVoiceId,omitempty"`
	DefaultModelID string             `json:"defaultModelId,omitempty"`
	Voices         []TTSVoiceManifest `json:"voices,omitempty"`
}

type TTSVoiceManifest struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Locale  string `json:"locale"`
	ModelID string `json:"modelId,omitempty"`
}

func DecodeProviderManifest(raw string) (ProviderManifest, error) {
	var manifest ProviderManifest
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return ProviderManifest{}, fmt.Errorf("decode provider manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ProviderManifest{}, errors.New("decode provider manifest: trailing JSON data")
	}
	return manifest, nil
}

func ValidateProviderManifest(manifest ProviderManifest) []string {
	var problems []string
	if strings.TrimSpace(manifest.ID) == "" {
		problems = append(problems, "id is required")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		problems = append(problems, "name is required")
	}
	if strings.TrimSpace(manifest.Version) == "" {
		problems = append(problems, "version is required")
	}
	if !validProviderKind(manifest.Kind) {
		problems = append(problems, "kind must be wake-word, stt, or tts")
	}
	if strings.TrimSpace(manifest.Transport.Type) == "" {
		problems = append(problems, "transport.type is required")
	}
	problems = append(problems, validateTransport(manifest.Transport, manifest.Kind)...)
	problems = append(problems, validateCredentials(manifest.Credentials)...)
	if manifest.Kind == ProviderKindWakeWord {
		problems = append(problems, validateWakeWordManifest(manifest.WakeWord)...)
	}
	if manifest.Kind == ProviderKindTTS {
		problems = append(problems, validateTTSManifest(manifest.TTS)...)
	}
	return problems
}

func validateCredentials(credentials []CredentialManifest) []string {
	var problems []string
	seen := map[string]struct{}{}
	for i, credential := range credentials {
		location := fmt.Sprintf("credentials[%d]", i)
		id := strings.TrimSpace(credential.ID)
		if id == "" {
			problems = append(problems, location+".id is required")
		} else if _, ok := seen[id]; ok {
			problems = append(problems, location+".id must be unique")
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(credential.Label) == "" {
			problems = append(problems, location+".label is required")
		}
		if credential.Source != "env" {
			problems = append(problems, location+".source must be env")
		}
		if strings.TrimSpace(credential.Env) == "" {
			problems = append(problems, location+".env is required for env credentials")
		}
		if containsRawCredentialValue(credential.ID) ||
			containsRawCredentialValue(credential.Label) ||
			containsRawCredentialValue(credential.Env) {
			problems = append(problems, location+" must reference a secret without embedding credential values")
		}
	}
	return problems
}

func validProviderKind(kind string) bool {
	switch kind {
	case ProviderKindWakeWord, ProviderKindSTT, ProviderKindTTS:
		return true
	default:
		return false
	}
}

func validateTransport(transport TransportManifest, kind string) []string {
	var problems []string
	if transport.Type != "command" {
		return []string{"transport.type must be command"}
	}
	if strings.TrimSpace(transport.Command) == "" {
		problems = append(problems, "transport.command is required for command providers")
	}
	if !filepath.IsAbs(transport.Command) {
		problems = append(problems, "transport.command must be absolute for command providers")
	}
	if isSTTCapableProviderKind(kind) {
		if !hasCommandArg(transport.Args, "{modelId}") {
			problems = append(problems, "transport.args must include {modelId} for STT-capable command providers")
		}
		if !hasCommandArg(transport.Args, "{inputPath}") {
			problems = append(problems, "transport.args must include {inputPath} for STT-capable command providers")
		}
	}
	if kind == ProviderKindWakeWord && !hasCommandArg(transport.Args, "{inputPath}") {
		problems = append(problems, "transport.args must include {inputPath} for wake-word command providers")
	}
	return problems
}

func isSTTCapableProviderKind(kind string) bool {
	return kind == ProviderKindSTT
}

func hasCommandArg(args []string, want string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == want {
			return true
		}
	}
	return false
}

func validateWakeWordManifest(wake WakeWordManifest) []string {
	var problems []string
	if strings.TrimSpace(wake.DefaultModelID) == "" {
		problems = append(problems, "wakeWord.defaultModelId is required")
	}
	if wake.Sensitivity < 0 || wake.Sensitivity > 1 {
		problems = append(problems, "wakeWord.sensitivity must be between 0 and 1")
	}
	declaredModels := map[string]struct{}{}
	for i, model := range wake.Models {
		location := fmt.Sprintf("wakeWord.models[%d]", i)
		if strings.TrimSpace(model.ID) == "" {
			problems = append(problems, location+".id is required")
		}
		if strings.TrimSpace(model.Path) == "" {
			problems = append(problems, location+".path is required")
		}
		if unsafeModelPath(model.Path) {
			problems = append(problems, location+".path must be a relative provider-pack asset path")
		}
		if model.Sensitivity < 0 || model.Sensitivity > 1 {
			problems = append(problems, location+".sensitivity must be between 0 and 1")
		}
		modelID := strings.TrimSpace(model.ID)
		if modelID != "" {
			if _, ok := declaredModels[modelID]; ok {
				problems = append(problems, location+".id must be unique")
			}
			declaredModels[modelID] = struct{}{}
		}
	}
	if _, ok := declaredModels[wake.DefaultModelID]; strings.TrimSpace(wake.DefaultModelID) != "" && !ok {
		problems = append(problems, "wakeWord.defaultModelId must reference a declared wakeWord model")
	}
	return problems
}

func validateTTSManifest(tts TTSManifest) []string {
	var problems []string
	if len(tts.Voices) == 0 {
		problems = append(problems, "tts.voices must declare at least one voice")
	}
	declaredVoices := map[string]struct{}{}
	for i, voice := range tts.Voices {
		location := fmt.Sprintf("tts.voices[%d]", i)
		if strings.TrimSpace(voice.ID) == "" {
			problems = append(problems, location+".id is required")
		}
		if strings.TrimSpace(voice.Label) == "" {
			problems = append(problems, location+".label is required")
		}
		if strings.TrimSpace(voice.Locale) == "" {
			problems = append(problems, location+".locale is required")
		}
		voiceID := strings.TrimSpace(voice.ID)
		if voiceID != "" {
			if _, ok := declaredVoices[voiceID]; ok {
				problems = append(problems, location+".id must be unique")
			}
			declaredVoices[voiceID] = struct{}{}
		}
	}
	if _, ok := declaredVoices[tts.DefaultVoiceID]; strings.TrimSpace(tts.DefaultVoiceID) != "" && !ok {
		problems = append(problems, "tts.defaultVoiceId must reference a declared voice")
	}
	return problems
}

func unsafeModelPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, `\`) {
		return true
	}
	if u, err := url.Parse(path); err == nil && u.Scheme != "" {
		return true
	}
	clean := filepath.Clean(path)
	return clean == "." || clean == ".." || strings.HasPrefix(clean, "../")
}

func containsRawCredentialValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	for _, fragment := range []string{
		"bearer ",
		"token:",
		"token=",
		"secret:",
		"secret=",
		"password:",
		"password=",
		"api_key=",
		"api-key=",
		"apikey=",
		"sk-",
		"xoxb-",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func wakeWordSummary(manifest ProviderManifest) *WakeWordProviderSummary {
	if manifest.Kind != ProviderKindWakeWord {
		return nil
	}
	models := make([]WakeWordModelSummary, 0, len(manifest.WakeWord.Models))
	for _, model := range manifest.WakeWord.Models {
		models = append(models, WakeWordModelSummary{
			ID:          model.ID,
			Phrase:      model.Phrase,
			Languages:   append([]string(nil), model.Languages...),
			Sensitivity: model.Sensitivity,
		})
	}
	return &WakeWordProviderSummary{
		DefaultModelID: manifest.WakeWord.DefaultModelID,
		Phrase:         manifest.WakeWord.Phrase,
		Languages:      append([]string(nil), manifest.WakeWord.Languages...),
		Sensitivity:    manifest.WakeWord.Sensitivity,
		Models:         models,
	}
}

func WakeWordSummary(manifest ProviderManifest) *WakeWordProviderSummary {
	return wakeWordSummary(manifest)
}

func ttsVoicesFromManifest(manifest ProviderManifest) []TTSVoice {
	voices := make([]TTSVoice, 0, len(manifest.TTS.Voices))
	for _, voice := range manifest.TTS.Voices {
		voices = append(voices, TTSVoice(voice))
	}
	return voices
}

func ttsVoiceLocale(manifest ProviderManifest, voiceID string) string {
	voiceID = strings.TrimSpace(voiceID)
	for _, voice := range manifest.TTS.Voices {
		if strings.TrimSpace(voice.ID) == voiceID {
			return strings.TrimSpace(voice.Locale)
		}
	}
	return ""
}

func TTSVoiceLocale(manifest ProviderManifest, voiceID string) string {
	return ttsVoiceLocale(manifest, voiceID)
}

func TTSVoicesFromManifest(manifest ProviderManifest) []TTSVoice {
	return ttsVoicesFromManifest(manifest)
}

func ttsSelectedVoiceID(manifest ProviderManifest, selectedVoiceID string) string {
	selectedVoiceID = strings.TrimSpace(selectedVoiceID)
	for _, voice := range manifest.TTS.Voices {
		if strings.TrimSpace(voice.ID) == selectedVoiceID {
			return selectedVoiceID
		}
	}
	return strings.TrimSpace(manifest.TTS.DefaultVoiceID)
}

func TTSSelectedVoiceID(manifest ProviderManifest, selectedVoiceID string) string {
	return ttsSelectedVoiceID(manifest, selectedVoiceID)
}

func firstLanguage(languages []string) string {
	for _, language := range languages {
		if language = strings.TrimSpace(language); language != "" {
			return language
		}
	}
	return ""
}

func FirstLanguage(languages []string) string {
	return firstLanguage(languages)
}

func missingRequiredCredential(manifest ProviderManifest) bool {
	for _, credential := range manifest.Credentials {
		if !credential.Required {
			continue
		}
		if credential.Source != "env" || strings.TrimSpace(credential.Env) == "" {
			return true
		}
		if _, ok := os.LookupEnv(credential.Env); !ok {
			return true
		}
	}
	return false
}

func MissingRequiredCredential(manifest ProviderManifest) bool {
	return missingRequiredCredential(manifest)
}
