package service

import (
	"testing"
	"time"
)

func TestTTSAudioStoreExpiresAudio(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	store := NewTTSAudioStore(time.Second)
	store.now = func() time.Time { return now }

	if !store.Put("tts-1", TTSAudioResult{Audio: []byte{1}, ContentType: "audio/pcm"}) {
		t.Fatal("expected audio to be stored")
	}
	if got, ok := store.Get("tts-1"); !ok || got.ContentType != "audio/pcm" || len(got.Audio) != 1 {
		t.Fatalf("expected stored audio, got ok=%v item=%+v", ok, got)
	}

	now = now.Add(time.Second)
	if _, ok := store.Get("tts-1"); ok {
		t.Fatal("expected expired audio to be removed")
	}
}
