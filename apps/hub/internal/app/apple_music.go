package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/apps/hub/internal/pkg/httphelper"
)

type appleMusicController struct {
	server *Server
}

type appleMusicUserTokenRequest struct {
	ConnectionID string `json:"connectionId"`
	UserToken    string `json:"userToken"`
}

func newAppleMusicController(server *Server) *appleMusicController {
	return &appleMusicController{server: server}
}

func (c *appleMusicController) handleMusicKitToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httphelper.WriteMethodNotAllowed(w, http.MethodGet)
		return
	}
	connection, ok := c.appleMusicConnection(r.Context(), w, r.URL.Query().Get("connectionId"))
	if !ok {
		return
	}
	developerToken := c.resolveSecretRef(r.Context(), connection.SecretRefs["developer_token"])
	userToken := c.resolveSecretRef(r.Context(), connection.SecretRefs["user_token"])
	if developerToken == "" {
		httphelper.WriteError(w, http.StatusUnauthorized, "Apple Music developer token is not configured")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, map[string]any{
		"developerToken": developerToken,
		"userToken":      userToken,
	})
}

func (c *appleMusicController) handleUserToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httphelper.WriteMethodNotAllowed(w, http.MethodPost)
		return
	}
	if c.server.secretStore == nil {
		httphelper.WriteError(w, http.StatusServiceUnavailable, "secret storage is unavailable")
		return
	}
	var req appleMusicUserTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	userToken := strings.TrimSpace(req.UserToken)
	if userToken == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "Apple Music user token is required")
		return
	}
	connection, ok := c.appleMusicConnection(r.Context(), w, req.ConnectionID)
	if !ok {
		return
	}
	secretID := "apple-music/" + connection.ID + "/user_token"
	if err := c.server.secretStore.Store(r.Context(), secretID, "apple-music", userToken); err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "Apple Music user token could not be saved")
		return
	}
	connection.SecretRefs["user_token"] = "db:" + secretID
	saved, err := c.server.settings.SaveAdapterConnection(r.Context(), connection)
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "Apple Music connection could not be saved")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, saved)
}

func (c *appleMusicController) appleMusicConnection(
	ctx context.Context,
	w http.ResponseWriter,
	connectionID string,
) (model.AdapterConnection, bool) {
	connectionID = strings.TrimSpace(connectionID)
	if connectionID == "" {
		httphelper.WriteError(w, http.StatusBadRequest, "connectionId is required")
		return model.AdapterConnection{}, false
	}
	if c.server.settings == nil {
		httphelper.WriteError(w, http.StatusServiceUnavailable, "settings storage is unavailable")
		return model.AdapterConnection{}, false
	}
	connection, err := c.server.settings.AdapterConnection(ctx, connectionID)
	if err != nil {
		httphelper.WriteError(w, http.StatusNotFound, "Apple Music connection was not found")
		return model.AdapterConnection{}, false
	}
	if connection.Kind != "apple-music" {
		httphelper.WriteError(w, http.StatusBadRequest, "connection is not an Apple Music account")
		return model.AdapterConnection{}, false
	}
	if connection.SecretRefs == nil {
		connection.SecretRefs = map[string]string{}
	}
	if connection.Settings == nil {
		connection.Settings = map[string]any{}
	}
	return connection, true
}

func (c *appleMusicController) resolveSecretRef(ctx context.Context, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || c.server.secretStore == nil {
		return ""
	}
	if strings.HasPrefix(ref, "db:") {
		value, err := c.server.secretStore.Resolve(ctx, strings.TrimPrefix(ref, "db:"))
		if err != nil {
			return ""
		}
		return value
	}
	return ref
}
