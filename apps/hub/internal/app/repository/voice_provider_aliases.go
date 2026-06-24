package repository

import "jute-dash/apps/hub/internal/app/service"

type WakeProvider = service.WakeProvider
type STTProvider = service.STTProvider
type TTSProvider = service.TTSProvider
type ProviderManifest = service.ProviderManifest
type TransportManifest = service.TransportManifest
type CredentialManifest = service.CredentialManifest
type WakeWordManifest = service.WakeWordManifest
type WakeWordModelManifest = service.WakeWordModelManifest
type TTSManifest = service.TTSManifest
type TTSVoiceManifest = service.TTSVoiceManifest

const (
	ProviderKindWakeWord = service.ProviderKindWakeWord
	ProviderKindSTT      = service.ProviderKindSTT
	ProviderKindTTS      = service.ProviderKindTTS
)

type CommandWakeProvider = service.CommandWakeProvider
type CommandSTTProvider = service.CommandSTTProvider
type CommandTTSProvider = service.CommandTTSProvider

func DecodeProviderManifest(raw string) (ProviderManifest, error) {
	return service.DecodeProviderManifest(raw)
}

func ValidateProviderManifest(manifest ProviderManifest) []string {
	return service.ValidateProviderManifest(manifest)
}

func missingRequiredCredential(manifest ProviderManifest) bool {
	return service.MissingRequiredCredential(manifest)
}

func firstLanguage(languages []string) string {
	return service.FirstLanguage(languages)
}

func ttsVoicesFromManifest(manifest ProviderManifest) []TTSVoice {
	return service.TTSVoicesFromManifest(manifest)
}

func ttsSelectedVoiceID(manifest ProviderManifest, selectedVoiceID string) string {
	return service.TTSSelectedVoiceID(manifest, selectedVoiceID)
}

func ttsVoiceLocale(manifest ProviderManifest, voiceID string) string {
	return service.TTSVoiceLocale(manifest, voiceID)
}

func wakeWordSummary(manifest ProviderManifest) *WakeWordProviderSummary {
	return service.WakeWordSummary(manifest)
}

func sanitizeText(value string) string {
	return service.SanitizeText(value)
}
