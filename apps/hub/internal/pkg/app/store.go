package app

import (
	"log/slog"

	"jute-dash/apps/hub/internal/app/repository"
)

type Store = repository.Store

func Open(dbPath string, log *slog.Logger) (*Store, error) {
	return repository.Open(dbPath, log)
}
