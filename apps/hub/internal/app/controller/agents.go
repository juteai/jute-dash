package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"jute-dash/apps/hub/internal/app/service"
	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/httphelper"
)

type AgentControllerOptions struct {
	Manager             *service.AgentManager
	Messages            a2aclient.MessageSender
	TurnRunner          *service.Runner
	GetDashboardContext func(ctx context.Context) map[string]any
}

type AgentController struct {
	opts AgentControllerOptions
}

func NewAgentController(opts AgentControllerOptions) *AgentController {
	return &AgentController{opts: opts}
}

func (c *AgentController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/agents", c.handleAgents)
	mux.HandleFunc("/api/v1/agents/", c.handleAgentSubroutes)
	mux.HandleFunc("/api/v1/proxy/agents/", c.handleProxyAgent)
}

func (c *AgentController) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.writeJSON(w, http.StatusOK, map[string]any{
			"agents": c.opts.Manager.List(r.Context(), true),
		})
	case http.MethodPost:
		var req struct {
			CardURL string `json:"cardUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		agent, err := c.opts.Manager.Add(r.Context(), req.CardURL)
		if err != nil {
			c.writeAgentConfigError(w, err)
			return
		}
		c.writeJSON(w, http.StatusCreated, agent)
	default:
		c.writeMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (c *AgentController) handleAgentSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		c.writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	agentID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPatch:
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				c.writeError(w, http.StatusBadRequest, "invalid JSON request body")
				return
			}
			agent, err := c.opts.Manager.Patch(r.Context(), agentID, req.Enabled)
			if err != nil {
				c.writeAgentConfigError(w, err)
				return
			}
			c.writeJSON(w, http.StatusOK, agent)
		case http.MethodDelete:
			if err := c.opts.Manager.Delete(r.Context(), agentID); err != nil {
				c.writeAgentConfigError(w, err)
				return
			}
			c.writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		default:
			c.writeMethodNotAllowed(w, http.MethodPatch+", "+http.MethodDelete)
		}
		return
	}
	if len(parts) != 2 || parts[1] != "refresh-card" {
		c.writeError(w, http.StatusNotFound, "agent route not found")
		return
	}
	if r.Method != http.MethodPost {
		c.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	enriched, err := c.opts.Manager.RefreshCard(r.Context(), agentID)
	if err != nil {
		c.writeAgentConfigError(w, err)
		return
	}
	c.writeJSON(w, http.StatusOK, enriched)
}

func (c *AgentController) AgentStatusSummary(ctx context.Context) AgentStatusSummary {
	return c.opts.Manager.StatusSummary(ctx)
}

func (c *AgentController) writeAgentConfigError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrYAMLConfigRequired):
		c.writeError(w, http.StatusConflict, "YAML config file is required to add agents")
	case errors.Is(err, a2aclient.ErrAgentCardUnavailable):
		c.writeError(w, http.StatusBadGateway, "agent card could not be fetched")
	case errors.Is(err, a2aclient.ErrAgentCardURLNotAllowed):
		c.writeError(w, http.StatusBadRequest, "agent card URL is not allowed")
	case errors.Is(err, a2aclient.ErrNoSupportedInterface):
		c.writeError(w, http.StatusBadRequest, "agent card has no compatible A2A 1.0 JSON-RPC interface")
	case strings.Contains(err.Error(), "required"):
		c.writeError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "not found"):
		c.writeError(w, http.StatusNotFound, err.Error())
	default:
		c.writeError(w, http.StatusInternalServerError, "agent configuration could not be updated")
	}
}

func (c *AgentController) writeJSON(w http.ResponseWriter, status int, value any) {
	httphelper.WriteJSON(w, status, value)
}

func (c *AgentController) writeError(w http.ResponseWriter, status int, message string) {
	httphelper.WriteError(w, status, message)
}

func (c *AgentController) writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	httphelper.WriteMethodNotAllowed(w, allow)
}

func (c *AgentController) handleProxyAgent(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Minute))

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		c.writeError(w, http.StatusNotFound, "agent ID not found in proxy path")
		return
	}
	agentID := strings.TrimSpace(parts[0])

	publicAgent, ok := c.opts.Manager.Find(agentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent config not found")
		return
	}

	if !publicAgent.Enabled {
		c.writeError(w, http.StatusForbidden, "agent is disabled")
		return
	}

	agent, ok := c.opts.Manager.ConfiguredAgent(agentID)
	if !ok {
		c.writeError(w, http.StatusNotFound, "agent config not found")
		return
	}
	bearerToken, authAvailable := service.AgentBearerToken(agent)
	if !authAvailable {
		c.writeError(w, http.StatusServiceUnavailable, "agent credentials are not available")
		return
	}

	targetBase, err := url.Parse(publicAgent.EndpointURL)
	if err != nil {
		c.writeError(w, http.StatusInternalServerError, "invalid agent endpoint URL")
		return
	}

	prefix := "/api/v1/proxy/agents/" + agentID
	var subpath string
	if strings.HasPrefix(r.URL.Path, prefix) {
		subpath = r.URL.Path[len(prefix):]
	}

	targetURL := *targetBase
	targetURL.Path = singleJoiningSlash(targetURL.Path, subpath)
	targetURL.RawQuery = r.URL.RawQuery

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = &targetURL
			req.Host = targetBase.Host
			req.Header.Del("Authorization")
			if bearerToken != "" {
				req.Header.Set("Authorization", "Bearer "+bearerToken)
			}
		},
	}
	wrappedWriter := &idleTimeoutWriter{
		ResponseWriter: w,
		rc:             rc,
		idleTimeout:    5 * time.Minute,
	}
	proxy.ServeHTTP(wrappedWriter, r)
}

type idleTimeoutWriter struct {
	http.ResponseWriter

	rc          *http.ResponseController
	idleTimeout time.Duration
}

func (w *idleTimeoutWriter) Write(p []byte) (int, error) {
	_ = w.rc.SetWriteDeadline(time.Now().Add(w.idleTimeout))
	return w.ResponseWriter.Write(p)
}

func (w *idleTimeoutWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *idleTimeoutWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func singleJoiningSlash(a, b string) string {
	if b == "" || b == "/" {
		return a
	}
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
