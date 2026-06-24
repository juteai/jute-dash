package service

import (
	"sync"
	"time"
)

type StoredTTSAudio struct {
	Audio       []byte
	ContentType string
	ExpiresAt   time.Time
}

type TTSAudioStore struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]StoredTTSAudio
	now   func() time.Time
}

func NewTTSAudioStore(ttl time.Duration) *TTSAudioStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &TTSAudioStore{
		ttl:   ttl,
		items: map[string]StoredTTSAudio{},
		now:   time.Now,
	}
}

func (s *TTSAudioStore) Put(id string, audio TTSAudioResult) bool {
	if s == nil || id == "" || len(audio.Audio) == 0 {
		return false
	}
	contentType := audio.ContentType
	if contentType == "" {
		contentType = "audio/wav"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	s.items[id] = StoredTTSAudio{
		Audio:       append([]byte(nil), audio.Audio...),
		ContentType: contentType,
		ExpiresAt:   s.now().Add(s.ttl),
	}
	return true
}

func (s *TTSAudioStore) Get(id string) (StoredTTSAudio, bool) {
	if s == nil || id == "" {
		return StoredTTSAudio{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	item, ok := s.items[id]
	if !ok {
		return StoredTTSAudio{}, false
	}
	item.Audio = append([]byte(nil), item.Audio...)
	return item, true
}

func (s *TTSAudioStore) pruneLocked() {
	now := s.now()
	for id, item := range s.items {
		if !item.ExpiresAt.After(now) {
			delete(s.items, id)
		}
	}
}
