package alerts

import "testing"

func TestNormalizeSoundUsesSupportedSounds(t *testing.T) {
	if got, want := NormalizeSound("BELL", DefaultSound), "bell"; got != want {
		t.Fatalf("NormalizeSound = %q, want %q", got, want)
	}
}

func TestNormalizeSoundFallsBackToConfiguredSound(t *testing.T) {
	if got, want := NormalizeSound("gong", "soft"), "soft"; got != want {
		t.Fatalf("NormalizeSound fallback = %q, want %q", got, want)
	}
}

func TestNormalizeSoundFallsBackToDefaultSound(t *testing.T) {
	if got, want := NormalizeSound("gong", ""), DefaultSound; got != want {
		t.Fatalf("NormalizeSound default = %q, want %q", got, want)
	}
}

func TestSupportedSoundsReturnsCopy(t *testing.T) {
	sounds := SupportedSounds()
	sounds[0] = "mutated"

	if got, want := SupportedSounds()[0], DefaultSound; got != want {
		t.Fatalf("SupportedSounds mutated shared list: got %q, want %q", got, want)
	}
}
