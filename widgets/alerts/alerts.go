package alerts

import "strings"

const (
	DefaultSound      = "chime"
	DefaultSnoozeMins = 9
)

var supportedSounds = []string{"chime", "bell", "pulse", "soft", "none"}

func SupportedSounds() []string {
	return append([]string(nil), supportedSounds...)
}

func NormalizeSound(value string, fallback string) string {
	sound := strings.ToLower(strings.TrimSpace(value))
	for _, supported := range supportedSounds {
		if sound == supported {
			return sound
		}
	}
	if fallback != "" {
		return NormalizeSound(fallback, DefaultSound)
	}
	return DefaultSound
}

func SoundSchema() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": SupportedSounds(),
	}
}
