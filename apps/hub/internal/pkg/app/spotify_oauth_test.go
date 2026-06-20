package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/apps/hub/internal/app/repository"
)

type memorySecretStore map[string]string

func (s memorySecretStore) Resolve(_ context.Context, id string) (string, error) {
	return s[id], nil
}

func (s memorySecretStore) Store(_ context.Context, id string, _ string, value string) error {
	s[id] = value
	return nil
}

func TestSpotifyOAuthStoresTokensAsDBSecretRefs(t *testing.T) {
	oldTokenURL := spotifyTokenURL
	oldAuthorizeURL := spotifyAuthorizeURL
	defer func() {
		spotifyTokenURL = oldTokenURL
		spotifyAuthorizeURL = oldAuthorizeURL
	}()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST token exchange, got %s", r.Method)
		}
		if _, _, ok := r.BasicAuth(); ok {
			t.Fatal("PKCE token exchange should not require HTTP basic auth")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("client_id"); got != "client-id" {
			t.Fatalf("client_id = %q", got)
		}
		if got := r.Form.Get("code_verifier"); got == "" {
			t.Fatal("code_verifier missing")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()
	spotifyTokenURL = tokenServer.URL
	spotifyAuthorizeURL = "https://accounts.spotify.test/authorize"

	settings := repository.NewMemoryHomeRepository(model.SetupStatus{Complete: true})
	_, err := settings.SaveAdapterConnection(context.Background(), model.AdapterConnection{
		ID:         "spotify-main",
		Kind:       "spotify",
		Name:       "Spotify",
		Enabled:    true,
		Settings:   map[string]any{"client_id": "client-id"},
		SecretRefs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("save connection: %v", err)
	}
	secrets := memorySecretStore{}
	handler := NewServerWithSecrets(
		testConfig(),
		"test",
		model.SetupStatus{Complete: true},
		nil,
		settings,
		nil,
		"",
		nil,
		secrets,
	)

	authReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/integrations/spotify/auth?connectionId=spotify-main&returnUri=https%3A%2F%2Flocalhost%3A5173",
		nil,
	)
	authReq.Host = "127.0.0.1:8787"
	authRec := httptest.NewRecorder()
	handler.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusFound {
		t.Fatalf("auth status = %d: %s", authRec.Code, authRec.Body.String())
	}
	location := authRec.Header().Get("Location")
	if !strings.HasPrefix(location, "https://accounts.spotify.test/authorize?") {
		t.Fatalf("unexpected auth redirect: %s", location)
	}
	state := queryValue(t, location, "state")
	if got := queryValue(t, location, "redirect_uri"); got !=
		"http://127.0.0.1:8787/api/v1/integrations/spotify/callback" {
		t.Fatalf("redirect_uri = %q", got)
	}
	if got := queryValue(t, location, "code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method = %q", got)
	}
	if got := queryValue(t, location, "code_challenge"); got == "" {
		t.Fatal("code_challenge missing")
	}

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/integrations/spotify/callback?code=oauth-code&state="+state+"&response=json",
		nil,
	)
	callbackReq.Header.Set("Accept", "application/json")
	callbackReq.Host = "localhost:8787"
	callbackRec := httptest.NewRecorder()
	handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("callback status = %d: %s", callbackRec.Code, callbackRec.Body.String())
	}

	connection, err := settings.AdapterConnection(context.Background(), "spotify-main")
	if err != nil {
		t.Fatalf("load connection: %v", err)
	}
	if got := connection.SecretRefs["access_token"]; got != "db:spotify/spotify-main/access_token" {
		t.Fatalf("access token ref = %q", got)
	}
	if got := connection.SecretRefs["refresh_token"]; got != "db:spotify/spotify-main/refresh_token" {
		t.Fatalf("refresh token ref = %q", got)
	}
	if got := secrets["spotify/spotify-main/access_token"]; got != "access-token" {
		t.Fatalf("stored access token = %q", got)
	}
	if _, exists := connection.Settings["access_token"]; exists {
		t.Fatal("access token leaked into connection settings")
	}
}

func TestSpotifyWebPlaybackTokenRefreshesExpiredAccessToken(t *testing.T) {
	oldTokenURL := spotifyTokenURL
	defer func() {
		spotifyTokenURL = oldTokenURL
	}()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Fatalf("grant_type = %q", got)
		}
		if got := r.Form.Get("refresh_token"); got != "refresh-token" {
			t.Fatalf("refresh_token = %q", got)
		}
		if got := r.Form.Get("client_id"); got != "client-id" {
			t.Fatalf("client_id = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fresh-access-token",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()
	spotifyTokenURL = tokenServer.URL

	settings := repository.NewMemoryHomeRepository(model.SetupStatus{Complete: true})
	_, err := settings.SaveAdapterConnection(context.Background(), model.AdapterConnection{
		ID:       "spotify-main",
		Kind:     "spotify",
		Name:     "Spotify",
		Enabled:  true,
		Settings: map[string]any{"client_id": "client-id", "expires_at": int64(10)},
		SecretRefs: map[string]string{
			"access_token":  "db:spotify/spotify-main/access_token",
			"refresh_token": "db:spotify/spotify-main/refresh_token",
		},
	})
	if err != nil {
		t.Fatalf("save connection: %v", err)
	}
	secrets := memorySecretStore{
		"spotify/spotify-main/access_token":  "stale-access-token",
		"spotify/spotify-main/refresh_token": "refresh-token",
	}
	handler := NewServerWithSecrets(
		testConfig(),
		"test",
		model.SetupStatus{Complete: true},
		nil,
		settings,
		nil,
		"",
		nil,
		secrets,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/integrations/spotify/web-playback-token?connectionId=spotify-main",
		nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if body.AccessToken != "fresh-access-token" {
		t.Fatalf("access token = %q", body.AccessToken)
	}
	if got := secrets["spotify/spotify-main/access_token"]; got != "fresh-access-token" {
		t.Fatalf("stored access token = %q", got)
	}
}

func queryValue(t *testing.T, location string, key string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, location, nil)
	value := req.URL.Query().Get(key)
	if value == "" {
		t.Fatalf("%s missing from %s", key, location)
	}
	return value
}
