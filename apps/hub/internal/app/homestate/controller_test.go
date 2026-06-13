package homestate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "jute-dash/widgets/spotify/hub"
)

func TestConnectionKindsEndpointExposesTypedFields(t *testing.T) {
	store := NewMemoryRepository(SetupStatus{Complete: true})
	controller := NewController(store, nil, nil, nil)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/connection-kinds", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Kinds []struct {
			Kind   string `json:"kind"`
			Fields []struct {
				ID       string `json:"id"`
				Required bool   `json:"required"`
				Secret   bool   `json:"secret"`
			} `json:"fields"`
		} `json:"kinds"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var spotifyFields []struct {
		ID       string `json:"id"`
		Required bool   `json:"required"`
		Secret   bool   `json:"secret"`
	}
	for _, kind := range body.Kinds {
		if kind.Kind == "spotify" {
			spotifyFields = kind.Fields
			break
		}
	}
	if len(spotifyFields) == 0 {
		t.Fatal("expected spotify connection kind fields")
	}
	requiredSecrets := map[string]bool{}
	for _, field := range spotifyFields {
		if field.Required && field.Secret {
			requiredSecrets[field.ID] = true
		}
	}
	for _, id := range []string{"client_secret", "access_token", "refresh_token"} {
		if !requiredSecrets[id] {
			t.Fatalf("expected %s to be a required secret field; got %#v", id, spotifyFields)
		}
	}
}

func TestConnectionsEndpointValidatesKnownKindRequiredFields(t *testing.T) {
	store := NewMemoryRepository(SetupStatus{Complete: true})
	controller := NewController(store, nil, nil, nil)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	body := `{
		"id": "spotify-main",
		"kind": "spotify",
		"name": "Spotify",
		"settings": {"client_id": "client"},
		"secretRefs": {"client_secret": "env:SPOTIFY_CLIENT_SECRET"},
		"enabled": true
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/connections", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token refs, got %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := store.AdapterConnection(context.Background(), "spotify-main"); err == nil {
		t.Fatal("invalid connection should not be persisted")
	}
}
