package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/apps/hub/internal/pkg/httphelper"
)

const spotifyOAuthScope = "streaming user-read-email user-read-private user-read-playback-state user-modify-playback-state user-read-currently-playing user-library-read playlist-read-private user-top-read"

//nolint:gochecknoglobals // test seams for OAuth endpoint integration tests.
var (
	spotifyAuthorizeURL = "https://accounts.spotify.com/authorize"
	spotifyTokenURL     = "https://accounts.spotify.com/api/token" //nolint:gosec // URL is not a secret credential.
)

type secretStore interface {
	Resolve(ctx context.Context, id string) (string, error)
	Store(ctx context.Context, id string, kind string, value string) error
}

type spotifyOAuthController struct {
	server *Server
	client *http.Client
	now    func() time.Time

	mu     sync.Mutex
	states map[string]spotifyOAuthState
}

type spotifyOAuthState struct {
	ConnectionID     string
	WidgetInstanceID string
	RedirectURI      string
	ReturnURI        string
	CodeVerifier     string
	ExpiresAt        time.Time
}

type spotifyTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func newSpotifyOAuthController(server *Server) *spotifyOAuthController {
	return &spotifyOAuthController{
		server: server,
		client: http.DefaultClient,
		now:    time.Now,
		states: map[string]spotifyOAuthState{},
	}
}

func (c *spotifyOAuthController) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httphelper.WriteMethodNotAllowed(w, http.MethodGet)
		return
	}
	if c.server.secretStore == nil {
		httphelper.WriteError(w, http.StatusServiceUnavailable, "secret storage is unavailable")
		return
	}
	connection, clientID, _, ok := c.spotifyClientConfig(r.Context(), w, r.URL.Query().Get("connectionId"))
	if !ok {
		return
	}
	state, err := randomOAuthState()
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "Spotify setup could not start")
		return
	}
	codeVerifier, err := randomPKCEVerifier()
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "Spotify setup could not start")
		return
	}
	redirectURI := spotifyRedirectURI(r)
	returnURI := spotifyReturnURI(r)
	c.mu.Lock()
	c.states[state] = spotifyOAuthState{
		ConnectionID:     connection.ID,
		WidgetInstanceID: strings.TrimSpace(r.URL.Query().Get("widgetInstanceId")),
		RedirectURI:      redirectURI,
		ReturnURI:        returnURI,
		CodeVerifier:     codeVerifier,
		ExpiresAt:        c.now().Add(10 * time.Minute),
	}
	c.mu.Unlock()

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", spotifyOAuthScope)
	params.Set("state", state)
	params.Set("code_challenge_method", "S256")
	params.Set("code_challenge", pkceChallenge(codeVerifier))
	http.Redirect(w, r, spotifyAuthorizeURL+"?"+params.Encode(), http.StatusFound)
}

func (c *spotifyOAuthController) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httphelper.WriteMethodNotAllowed(w, http.MethodGet)
		return
	}
	if errMsg := strings.TrimSpace(r.URL.Query().Get("error")); errMsg != "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Spotify did not authorize Jute Dash")
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	stateID := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || stateID == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Spotify callback is missing required parameters")
		return
	}
	state, ok := c.popState(stateID)
	if !ok || c.now().After(state.ExpiresAt) {
		httphelper.WriteError(w, http.StatusBadRequest, "Spotify setup session expired")
		return
	}

	connection, clientID, clientSecret, ok := c.spotifyClientConfig(r.Context(), w, state.ConnectionID)
	if !ok {
		return
	}
	token, err := c.exchangeCode(
		r.Context(),
		code,
		state.RedirectURI,
		clientID,
		clientSecret,
		state.CodeVerifier,
	)
	if err != nil {
		httphelper.WriteError(w, http.StatusBadGateway, "Spotify token exchange failed")
		return
	}
	if token.AccessToken == "" {
		httphelper.WriteError(w, http.StatusBadGateway, "Spotify token exchange returned no access token")
		return
	}
	if _, err := c.saveTokenResponse(r.Context(), connection, token); err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "Spotify connection could not be saved")
		return
	}
	if state.WidgetInstanceID != "" {
		_ = c.linkWidget(r.Context(), state.WidgetInstanceID, connection.ID)
	}
	if wantsJSONResponse(r) {
		httphelper.WriteJSON(w, http.StatusOK, map[string]any{
			"status":       "linked",
			"connectionId": connection.ID,
		})
		return
	}
	if state.ReturnURI != "" {
		if returnURL := spotifyRedirectDisplayURL(state.ReturnURI, "linked"); returnURL != "" {
			http.Redirect(w, r, returnURL, http.StatusSeeOther)
			return
		}
	}
	if state.ReturnURI != "" {
		http.Redirect(w, r, state.ReturnURI, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?spotify=linked", http.StatusSeeOther)
}

func (c *spotifyOAuthController) handleWebPlaybackToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httphelper.WriteMethodNotAllowed(w, http.MethodGet)
		return
	}
	if c.server.secretStore == nil {
		httphelper.WriteError(w, http.StatusServiceUnavailable, "secret storage is unavailable")
		return
	}
	connection, clientID, clientSecret, ok := c.spotifyClientConfig(
		r.Context(),
		w,
		r.URL.Query().Get("connectionId"),
	)
	if !ok {
		return
	}
	accessToken := c.resolveSecretRef(r.Context(), connection.SecretRefs["access_token"])
	if accessToken == "" || c.spotifyTokenExpired(connection) {
		refreshToken := c.resolveSecretRef(r.Context(), connection.SecretRefs["refresh_token"])
		if refreshToken == "" {
			httphelper.WriteError(w, http.StatusUnauthorized, "Spotify account is not linked")
			return
		}
		token, err := c.refreshAccessToken(r.Context(), clientID, clientSecret, refreshToken)
		if err != nil {
			httphelper.WriteError(w, http.StatusBadGateway, "Spotify token refresh failed")
			return
		}
		saved, err := c.saveTokenResponse(r.Context(), connection, token)
		if err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, "Spotify credentials could not be saved")
			return
		}
		connection = saved
		accessToken = c.resolveSecretRef(r.Context(), connection.SecretRefs["access_token"])
	}
	if accessToken == "" {
		httphelper.WriteError(w, http.StatusUnauthorized, "Spotify account is not linked")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, map[string]any{
		"accessToken": accessToken,
		"expiresAt":   connection.Settings["expires_at"],
		"scope":       spotifyOAuthScope,
	})
}

func (c *spotifyOAuthController) spotifyClientConfig(
	ctx context.Context,
	w http.ResponseWriter,
	connectionID string,
) (model.AdapterConnection, string, string, bool) {
	connectionID = strings.TrimSpace(connectionID)
	if connectionID == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Spotify connection ID is required")
		return model.AdapterConnection{}, "", "", false
	}
	connection, err := c.server.settings.AdapterConnection(ctx, connectionID)
	if err != nil {
		httphelper.WriteError(w, http.StatusNotFound, "Spotify connection was not found")
		return model.AdapterConnection{}, "", "", false
	}
	if connection.Kind != "spotify" {
		httphelper.WriteError(w, http.StatusBadRequest, "Connection is not a Spotify connection")
		return model.AdapterConnection{}, "", "", false
	}
	clientID := c.spotifyClientID(connection)
	clientSecret := c.resolveSecretRef(ctx, connection.SecretRefs["client_secret"])
	if clientID == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Spotify Client ID is required")
		return model.AdapterConnection{}, "", "", false
	}
	return connection, clientID, clientSecret, true
}

func (c *spotifyOAuthController) spotifyClientID(connection model.AdapterConnection) string {
	if value, _ := connection.Settings["client_id"].(string); strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if value := strings.TrimSpace(os.Getenv("JUTE_SPOTIFY_CLIENT_ID")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_ID"))
}

func (c *spotifyOAuthController) exchangeCode(
	ctx context.Context,
	code string,
	redirectURI string,
	clientID string,
	clientSecret string,
	codeVerifier string,
) (spotifyTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", codeVerifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return spotifyTokenResponse{}, err
	}
	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return spotifyTokenResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return spotifyTokenResponse{}, fmt.Errorf("spotify token status %d", resp.StatusCode)
	}
	var token spotifyTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return spotifyTokenResponse{}, err
	}
	return token, nil
}

func (c *spotifyOAuthController) refreshAccessToken(
	ctx context.Context,
	clientID string,
	clientSecret string,
	refreshToken string,
) (spotifyTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return spotifyTokenResponse{}, err
	}
	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return spotifyTokenResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return spotifyTokenResponse{}, fmt.Errorf("spotify refresh status %d", resp.StatusCode)
	}
	var token spotifyTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return spotifyTokenResponse{}, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

func (c *spotifyOAuthController) saveTokenResponse(
	ctx context.Context,
	connection model.AdapterConnection,
	token spotifyTokenResponse,
) (model.AdapterConnection, error) {
	if connection.Settings == nil {
		connection.Settings = map[string]any{}
	}
	if connection.SecretRefs == nil {
		connection.SecretRefs = map[string]string{}
	}
	if token.AccessToken != "" {
		accessID := "spotify/" + connection.ID + "/access_token"
		if err := c.server.secretStore.Store(ctx, accessID, "spotify", token.AccessToken); err != nil {
			return model.AdapterConnection{}, err
		}
		connection.SecretRefs["access_token"] = "db:" + accessID
	}
	if token.RefreshToken != "" {
		refreshID := "spotify/" + connection.ID + "/refresh_token"
		if err := c.server.secretStore.Store(ctx, refreshID, "spotify", token.RefreshToken); err != nil {
			return model.AdapterConnection{}, err
		}
		connection.SecretRefs["refresh_token"] = "db:" + refreshID
	}
	if token.ExpiresIn > 0 {
		connection.Settings["expires_at"] = c.now().Add(time.Duration(token.ExpiresIn) * time.Second).Unix()
	}
	return c.server.settings.SaveAdapterConnection(ctx, connection)
}

func (c *spotifyOAuthController) spotifyTokenExpired(connection model.AdapterConnection) bool {
	expiresAt := int64(0)
	switch v := connection.Settings["expires_at"].(type) {
	case int64:
		expiresAt = v
	case int:
		expiresAt = int64(v)
	case float64:
		expiresAt = int64(v)
	}
	return expiresAt > 0 && c.now().Unix() >= expiresAt-60
}

func (c *spotifyOAuthController) linkWidget(
	ctx context.Context,
	widgetInstanceID string,
	connectionID string,
) error {
	if c.server.layoutStore == nil {
		return nil
	}
	layout, err := c.server.layoutStore.WidgetLayout(ctx, "")
	if err != nil {
		return err
	}
	for i := range layout.Widgets {
		if layout.Widgets[i].ID != widgetInstanceID || layout.Widgets[i].Kind != "spotify" {
			continue
		}
		if layout.Widgets[i].ConnectionRefs == nil {
			layout.Widgets[i].ConnectionRefs = map[string]string{}
		}
		layout.Widgets[i].ConnectionRefs["account"] = connectionID
		_, err := c.server.layoutStore.SaveWidgetLayout(ctx, layout)
		if err == nil {
			c.server.mu.Lock()
			c.server.layout = layout
			c.server.mu.Unlock()
		}
		return err
	}
	return nil
}

func (c *spotifyOAuthController) popState(stateID string) (spotifyOAuthState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.states[stateID]
	if ok {
		delete(c.states, stateID)
	}
	return state, ok
}

func (c *spotifyOAuthController) resolveSecretRef(ctx context.Context, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "env:") {
		return os.Getenv(strings.TrimPrefix(ref, "env:"))
	}
	if strings.HasPrefix(ref, "db:") {
		if c.server.secretStore == nil {
			return ""
		}
		value, err := c.server.secretStore.Resolve(ctx, strings.TrimPrefix(ref, "db:"))
		if err != nil {
			return ""
		}
		return value
	}
	return os.Getenv(ref)
}

func spotifyRedirectURI(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	return scheme + "://" + spotifyLoopbackHost(r.Host) + "/api/v1/integrations/spotify/callback"
}

func spotifyReturnURI(r *http.Request) string {
	if returnURI := strings.TrimSpace(r.URL.Query().Get("returnUri")); safeLocalRedirectURI(returnURI) {
		return returnURI
	}
	if referer := strings.TrimSpace(r.Header.Get("Referer")); safeLocalRedirectURI(referer) {
		parsed, err := url.Parse(referer)
		if err == nil {
			return parsed.Scheme + "://" + parsed.Host
		}
	}
	return ""
}

func spotifyLoopbackHost(host string) string {
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		if strings.EqualFold(host, "localhost") {
			return "127.0.0.1"
		}
		return host
	}
	if strings.EqualFold(hostname, "localhost") {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return host
}

func spotifyRedirectDisplayURL(returnURI string, status string) string {
	parsed, err := url.Parse(returnURI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	query := parsed.Query()
	query.Set("spotify", status)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func safeLocalRedirectURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func wantsJSONResponse(r *http.Request) bool {
	return r.URL.Query().Get("response") == "json" ||
		strings.Contains(r.Header.Get("Accept"), "application/json")
}

func randomOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomPKCEVerifier() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
