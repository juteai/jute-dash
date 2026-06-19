package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"jute-dash/apps/hub/internal/app/service/agents"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/pkg/widgetskills"
)

const (
	ProtocolVersion = "2025-11-25"
	jsonRPCVersion  = "2.0"

	callerAgentHeader = "X-Jute-Agent-Id"
)

// AuthConfig defines the bridge security model.
type AuthConfig struct {
	Mode     string `json:"mode"     yaml:"mode"`
	EnvToken string `json:"envToken" yaml:"env-token"`
}

// Config wraps local MCP bridge listener configuration.
type Config struct {
	Enabled       bool       `json:"enabled"       yaml:"enabled"`
	Transport     string     `json:"transport"     yaml:"transport"`
	ListenAddress string     `json:"listenAddress" yaml:"listen-address"`
	Path          string     `json:"path"          yaml:"path"`
	AllowLAN      bool       `json:"allowLan"      yaml:"allow-lan"`
	Auth          AuthConfig `json:"auth"          yaml:"auth"`
}

type SnapshotProvider interface {
	Snapshot(context.Context) (widgetskills.Snapshot, error)
}

type DisplayActions interface {
	Notify(message, severity string) (displayactions.Notification, error)
	FocusWidget(widgetInstanceID, reason string) (displayactions.FocusWidget, error)
}

type WidgetActionDispatcher interface {
	InvokeWidgetAction(
		ctx context.Context,
		widgetInstanceID string,
		actionID string,
		arguments map[string]any,
		actor string,
		confirmed bool,
	) (map[string]any, error)
}

type Handler struct {
	cfg     Config
	version string

	provider         SnapshotProvider
	display          DisplayActions
	actionDispatcher WidgetActionDispatcher
}

func NewHandler(
	cfg Config,
	version string,
	provider SnapshotProvider,
	display ...DisplayActions,
) http.Handler {
	var actionSink DisplayActions
	if len(display) > 0 {
		actionSink = display[0]
	}
	return &Handler{cfg: cfg, version: version, provider: provider, display: actionSink}
}

func NewHandlerWithActions(
	cfg Config,
	version string,
	provider SnapshotProvider,
	display DisplayActions,
	actionDispatcher WidgetActionDispatcher,
) http.Handler {
	return &Handler{
		cfg:              cfg,
		version:          version,
		provider:         provider,
		display:          display,
		actionDispatcher: actionDispatcher,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.validOrigin(r) {
		writeRPCError(w, http.StatusForbidden, nil, -32000, "origin is not allowed")
		return
	}
	if !h.authorized(r) {
		writeRPCError(w, http.StatusUnauthorized, nil, -32001, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "MCP SSE stream is not implemented", http.StatusMethodNotAllowed)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, nil, -32700, "invalid request")
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPCError(w, http.StatusBadRequest, nil, -32700, "parse error")
		return
	}
	if req.JSONRPC != jsonRPCVersion || strings.TrimSpace(req.Method) == "" {
		writeRPCError(w, http.StatusBadRequest, req.ID, -32600, "invalid request")
		return
	}
	if len(req.ID) == 0 {
		if strings.HasPrefix(req.Method, "notifications/") {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeRPCError(w, http.StatusBadRequest, nil, -32600, "request id is required")
		return
	}

	result, rpcErr := h.dispatch(r.Context(), r, req.Method, req.Params)
	if rpcErr != nil {
		writeRPCError(w, http.StatusOK, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}
	writeRPCResult(w, req.ID, result)
}

func (h *Handler) dispatch(
	ctx context.Context,
	r *http.Request,
	method string,
	params json.RawMessage,
) (any, *rpcError) {
	switch method {
	case "initialize":
		return h.initializeResult(), nil
	case "resources/list":
		snapshot, err := h.snapshot(ctx)
		if err != nil {
			return nil, internalError()
		}
		caller, rpcErr := h.callerForRequest(snapshot, r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"resources": resourcesList(snapshot, caller)}, nil
	case "resources/read":
		var req resourceReadParams
		if err := decodeParams(params, &req); err != nil {
			return nil, invalidParams(err)
		}
		return h.readResource(ctx, r, req.URI)
	case "tools/list":
		snapshot, err := h.snapshot(ctx)
		if err != nil {
			return nil, internalError()
		}
		caller, rpcErr := h.callerForRequest(snapshot, r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"tools": toolsList(caller)}, nil
	case "tools/call":
		var req toolCallParams
		if err := decodeParams(params, &req); err != nil {
			return nil, invalidParams(err)
		}
		return h.callTool(ctx, r, req)
	case "prompts/list":
		snapshot, err := h.snapshot(ctx)
		if err != nil {
			return nil, internalError()
		}
		caller, rpcErr := h.callerForRequest(snapshot, r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"prompts": promptsList(caller)}, nil
	case "prompts/get":
		var req promptGetParams
		if err := decodeParams(params, &req); err != nil {
			return nil, invalidParams(err)
		}
		return h.getPrompt(ctx, r, req.Name, req.Arguments)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func (h *Handler) initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities": map[string]any{
			"resources": map[string]any{"listChanged": false},
			"tools":     map[string]any{"listChanged": false},
			"prompts":   map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":        "jute-dash",
			"title":       "Jute Dash MCP Bridge",
			"version":     h.version,
			"description": "Local MCP bridge exposing hub-approved Jute dashboard context and Widget Skills.",
		},
	}
}

func (h *Handler) readResource(ctx context.Context, r *http.Request, uri string) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	for _, route := range ResourceRoutes {
		if route.Match(uri) {
			if !caller.has(route.Scope()) {
				return nil, missingScope(route.Scope())
			}
			routeCtx := RouteContext{
				Context:          ctx,
				Snapshot:         snapshot,
				Display:          h.display,
				ActionDispatcher: h.actionDispatcher,
			}
			val, err := route.Read(routeCtx, uri)
			if err != nil {
				if errors.Is(err, widgetskills.ErrNotFound) {
					return nil, notFound("resource not found")
				}
				return nil, internalError()
			}
			text, err := jsonText(val)
			if err != nil {
				return nil, internalError()
			}
			return map[string]any{
				"contents": []map[string]any{
					{
						"uri":      uri,
						"mimeType": "application/json",
						"text":     text,
					},
				},
			}, nil
		}
	}
	return nil, notFound("resource not found")
}

func (h *Handler) callTool(ctx context.Context, r *http.Request, req toolCallParams) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	for _, route := range ToolRoutes {
		if route.Name() == req.Name {
			if !caller.has(route.Scope()) {
				return nil, missingScope(route.Scope())
			}
			routeCtx := RouteContext{
				Context:          ctx,
				Snapshot:         snapshot,
				Display:          h.display,
				ActionDispatcher: h.actionDispatcher,
			}
			val, err := route.Call(routeCtx, req.Arguments)
			if err != nil {
				var rpcErr *rpcError
				if errors.As(err, &rpcErr) {
					return nil, rpcErr
				}
				if errors.Is(err, widgetskills.ErrNotFound) {
					return nil, notFound("skill or action not found")
				}
				return nil, invalidParams(err)
			}
			text, err := jsonText(val)
			if err != nil {
				return nil, internalError()
			}
			return map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": text},
				},
				"structuredContent": val,
				"isError":           false,
			}, nil
		}
	}
	return nil, notFound("tool not found")
}

func (h *Handler) getPrompt(
	ctx context.Context,
	r *http.Request,
	name string,
	arguments map[string]any,
) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	for _, route := range PromptRoutes {
		if route.Name() == name {
			if !caller.has(route.Scope()) {
				return nil, missingScope(route.Scope())
			}
			routeCtx := RouteContext{
				Context:          ctx,
				Snapshot:         snapshot,
				Display:          h.display,
				ActionDispatcher: h.actionDispatcher,
			}
			text, err := route.Get(routeCtx, name, arguments)
			if err != nil {
				if errors.Is(err, widgetskills.ErrNotFound) {
					return nil, notFound("prompt not found")
				}
				return nil, invalidParams(err)
			}
			return map[string]any{
				"description": "Jute MCP prompt guidance",
				"messages": []map[string]any{
					{
						"role": "user",
						"content": map[string]any{
							"type": "text",
							"text": text,
						},
					},
				},
			}, nil
		}
	}
	return nil, notFound("prompt not found")
}

func (h *Handler) snapshot(ctx context.Context) (widgetskills.Snapshot, error) {
	if h.provider == nil {
		return widgetskills.Snapshot{}, errors.New("mcp snapshot provider is not configured")
	}
	return h.provider.Snapshot(ctx)
}

type caller struct {
	AgentID   string
	Anonymous bool
	Scopes    map[string]struct{}
}

func (h *Handler) callerForRequest(snapshot widgetskills.Snapshot, r *http.Request) (caller, *rpcError) {
	agentID := strings.TrimSpace(r.Header.Get(callerAgentHeader))
	if agentID == "" {
		if h.cfg.Auth.Mode == "none" {
			return newCaller("", true, agents.DefaultMCPReadScopes()), nil
		}
		return caller{}, unauthorized("mcp caller identity is required")
	}
	for _, agent := range snapshot.Agents {
		if agent.ID != agentID {
			continue
		}
		if !agent.Enabled {
			return caller{}, unauthorized("mcp caller is not enabled")
		}
		return newCaller(agent.ID, false, agent.MCPScopes), nil
	}
	return caller{}, unauthorized("mcp caller is not authorized")
}

func newCaller(agentID string, anonymous bool, scopes []string) caller {
	if len(scopes) == 0 {
		scopes = agents.DefaultMCPReadScopes()
	}
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			scopeSet[scope] = struct{}{}
		}
	}
	return caller{AgentID: agentID, Anonymous: anonymous, Scopes: scopeSet}
}

func (c caller) has(scope string) bool {
	_, ok := c.Scopes[scope]
	return ok
}

func (h *Handler) validOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if h.cfg.AllowLAN {
		return true
	}
	return isLoopbackHost(host)
}

func (h *Handler) authorized(r *http.Request) bool {
	if h.cfg.Auth.Mode == "none" {
		return true
	}
	if h.cfg.Auth.Mode != "local-token" {
		return false
	}
	token := strings.TrimSpace(os.Getenv(h.cfg.Auth.EnvToken))
	if token == "" {
		return false
	}
	return r.Header.Get("Authorization") == "Bearer "+token
}

func resourcesList(snapshot widgetskills.Snapshot, caller caller) []map[string]any {
	resources := []map[string]any{}
	for _, route := range ResourceRoutes {
		if caller.has(route.Scope()) {
			resources = append(resources, route.List(snapshot)...)
		}
	}
	return resources
}

func toolsList(caller caller) []map[string]any {
	tools := []map[string]any{}
	for _, route := range ToolRoutes {
		if caller.has(route.Scope()) {
			tools = append(tools, tool(
				route.Name(),
				route.Title(),
				route.Description(),
				route.InputSchema(),
			))
		}
	}
	return tools
}

func tool(name, title, description string, inputSchema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"title":       title,
		"description": description,
		"inputSchema": inputSchema,
	}
}

func promptsList(caller caller) []map[string]any {
	prompts := []map[string]any{}
	for _, route := range PromptRoutes {
		if caller.has(route.Scope()) {
			p := map[string]any{
				"name":        route.Name(),
				"title":       route.Title(),
				"description": route.Description(),
			}
			if args := route.Arguments(); len(args) > 0 {
				p["arguments"] = args
			}
			prompts = append(prompts, p)
		}
	}
	return prompts
}

func emptySchema() map[string]any {
	return objectSchema(map[string]any{}, []string{})
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": true,
	}
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func stringArg(arguments map[string]any, key string) string {
	if arguments == nil {
		return ""
	}
	value, _ := arguments[key].(string)
	return strings.TrimSpace(value)
}

func jsonText(value any) (string, error) {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func writeRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	})
}

func writeRPCError(w http.ResponseWriter, status int, id json.RawMessage, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func invalidParams(err error) *rpcError {
	return &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)}
}

func unauthorized(message string) *rpcError {
	return &rpcError{Code: -32003, Message: message}
}

func missingScope(scope string) *rpcError {
	return unauthorized("missing MCP scope: " + scope)
}

func notFound(message string) *rpcError {
	return &rpcError{Code: -32004, Message: message}
}

func internalError() *rpcError {
	return &rpcError{Code: -32603, Message: "internal error"}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC Error %d: %s", e.Code, e.Message)
}

type resourceReadParams struct {
	URI string `json:"uri"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type promptGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type Status struct {
	Enabled       bool   `json:"enabled"`
	ServiceStatus string `json:"serviceStatus"`
	Transport     string `json:"transport"`
	ListenAddress string `json:"listenAddress"`
	Path          string `json:"path"`
	AuthMode      string `json:"authMode"`
	AllowLAN      bool   `json:"allowLAN"`
}

// DefaultConfig returns the default configuration for MCP.
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		Transport:     "streamable-http",
		ListenAddress: "127.0.0.1:8790",
		Path:          "/mcp",
		AllowLAN:      false,
		Auth: AuthConfig{
			Mode:     "local-token",
			EnvToken: strings.Join([]string{"JUTE", "MCP", "TOKEN"}, "_"),
		},
	}
}

func ApplyDefaults(cfg *Config) {
	defaults := DefaultConfig()
	if strings.TrimSpace(cfg.Transport) == "" {
		cfg.Transport = defaults.Transport
	}
	if strings.TrimSpace(cfg.ListenAddress) == "" {
		cfg.ListenAddress = defaults.ListenAddress
	}
	if strings.TrimSpace(cfg.Path) == "" {
		cfg.Path = defaults.Path
	}
	if strings.TrimSpace(cfg.Auth.Mode) == "" {
		cfg.Auth.Mode = defaults.Auth.Mode
	}
	if strings.TrimSpace(cfg.Auth.EnvToken) == "" {
		cfg.Auth.EnvToken = defaults.Auth.EnvToken
	}
}

func Validate(cfg Config) []string {
	var problems []string
	if strings.TrimSpace(cfg.Transport) != "streamable-http" {
		problems = append(problems, "mcp.transport must be streamable-http")
	}
	if strings.TrimSpace(cfg.ListenAddress) == "" {
		problems = append(problems, "mcp.listenAddress is required")
	}
	if strings.TrimSpace(cfg.Path) == "" || !strings.HasPrefix(cfg.Path, "/") {
		problems = append(problems, "mcp.path must start with /")
	}
	if cfg.Auth.Mode != "none" && cfg.Auth.Mode != "local-token" {
		problems = append(problems, "mcp.auth.mode must be none or local-token")
	}
	if cfg.Enabled {
		if !cfg.AllowLAN && !isLoopbackListenAddress(cfg.ListenAddress) {
			problems = append(problems, "mcp.listenAddress must be loopback unless mcp.allowLan is true")
		}
		if cfg.Auth.Mode == "local-token" && strings.TrimSpace(cfg.Auth.EnvToken) == "" {
			problems = append(problems, "mcp.auth.envToken is required when local-token auth is enabled")
		}
	}
	return problems
}

func isLoopbackListenAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return isLoopbackHost(host)
}

func StatusFromConfig(cfg Config) Status {
	status := Status{
		Enabled:       cfg.Enabled,
		ServiceStatus: "disabled",
		Transport:     cfg.Transport,
		ListenAddress: cfg.ListenAddress,
		Path:          cfg.Path,
		AuthMode:      cfg.Auth.Mode,
		AllowLAN:      cfg.AllowLAN,
	}
	if !cfg.Enabled {
		return status
	}
	if strings.TrimSpace(cfg.Transport) == "" || strings.TrimSpace(cfg.ListenAddress) == "" ||
		strings.TrimSpace(cfg.Path) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	if strings.TrimSpace(cfg.Auth.Mode) == "" {
		status.ServiceStatus = "misconfigured"
		return status
	}
	status.ServiceStatus = "enabled"
	return status
}
