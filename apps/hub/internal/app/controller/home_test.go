package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jute-dash/apps/hub/internal/app/repository"
	_ "jute-dash/widgets/spotify/hub"
)

func TestConnectionKindsEndpointExposesTypedFields(t *testing.T) {
	store := repository.NewMemoryHomeRepository(SetupStatus{Complete: true})
	controller := NewHomeController(store, nil, nil, nil)
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
	if requiredSecrets["client_secret"] {
		t.Fatalf("expected client_secret to be optional for PKCE login; got %#v", spotifyFields)
	}
	for _, id := range []string{"access_token", "refresh_token"} {
		if requiredSecrets[id] {
			t.Fatalf("expected %s to be optional before OAuth linking; got %#v", id, spotifyFields)
		}
	}
}

func TestConnectionsEndpointValidatesKnownKindRequiredFields(t *testing.T) {
	store := repository.NewMemoryHomeRepository(SetupStatus{Complete: true})
	controller := NewHomeController(store, nil, nil, nil)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	body := `{
		"id": "spotify-main",
		"kind": "spotify",
		"name": "Spotify",
		"settings": {"client_id": "client"},
		"secretRefs": {},
		"enabled": true
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/connections", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initial OAuth setup record, got %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := store.AdapterConnection(context.Background(), "spotify-main"); err != nil {
		t.Fatalf("connection should be persisted before OAuth token refs exist: %v", err)
	}
}

func TestConnectionsEndpointAllowsSpotifyWithoutClientSecretForPKCE(t *testing.T) {
	store := repository.NewMemoryHomeRepository(SetupStatus{Complete: true})
	controller := NewHomeController(store, nil, nil, nil)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	body := `{
		"id": "spotify-main",
		"kind": "spotify",
		"name": "Spotify",
		"settings": {"client_id": "client"},
		"secretRefs": {},
		"enabled": true
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/connections", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without client secret for PKCE, got %d body=%s", rec.Code, rec.Body.String())
	}
}
