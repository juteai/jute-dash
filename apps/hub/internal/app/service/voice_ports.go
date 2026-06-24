package service

import "context"

type VoiceStore interface {
	VoiceSettings(ctx context.Context, deviceProfileID string) (Settings, error)
}

type VoiceDisplayEmitter interface {
	EmitVoiceStateChanged(deviceProfileID string, payload VoiceStatePayload) VoiceEvent
	EmitConversationEvent(eventType, deviceID, conversationID string, payload any) VoiceEvent
	EmitTTSEvent(eventType, deviceID string, response TTSActionResponse) VoiceEvent
}

type TTSProvider interface {
	Synthesize(ctx context.Context, req TTSRequest) (TTSAudioResult, error)
}

type CancelledConversation struct {
	ConversationID string
	DeviceID       string
}
