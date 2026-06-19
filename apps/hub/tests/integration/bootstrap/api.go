package bootstrap

import (
	"net/http"
	"os"
	"time"
)

func BaseURL() string {
	if value := os.Getenv("JUTE_HUB_BASE_URL"); value != "" {
		return value
	}
	return "http://localhost:8787"
}

func Client() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}
