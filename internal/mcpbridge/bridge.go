package mcpbridge

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
	"time"

	"jute-dash/internal/config"
	"jute-dash/internal/displayactions"
	"jute-dash/internal/widgetskills"
)

const (
	ProtocolVersion = "2025-11-25"
	jsonRPCVersion  = "2.0"

	callerAgentHeader = "X-Jute-Agent-ID"
)

type SnapshotProvider interface {
	Snapshot(context.Context) (widgetskills.Snapshot, error)
}

type DisplayActions interface {
	Notify(message, severity string) (displayactions.Notification, error)
	FocusWidget(widgetInstanceID, reason string) (displayactions.FocusWidget, error)
}

type Handler struct {
	cfg     config.MCPConfig
	version string

	provider SnapshotProvider
	display  DisplayActions
}

func NewHandler(cfg config.MCPConfig, version string, provider SnapshotProvider, display ...DisplayActions) http.Handler {
	var actionSink DisplayActions
	if len(display) > 0 {
		actionSink = display[0]
	}
	return &Handler{cfg: cfg, version: version, provider: provider, display: actionSink}
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

func (h *Handler) dispatch(ctx context.Context, r *http.Request, method string, params json.RawMessage) (any, *rpcError) {
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
	if rpcErr := requireResourceScope(caller, uri); rpcErr != nil {
		return nil, rpcErr
	}
	var value any
	switch {
	case uri == "jute://dashboard/current":
		value = widgetskills.DashboardSnapshot(snapshot)
	case uri == "jute://home/state":
		generatedAt := snapshot.GeneratedAt
		if generatedAt.IsZero() {
			generatedAt = time.Now().UTC()
		}
		value = map[string]any{
			"schema":      "https://jute.dev/mcp/resources/home-state/v1",
			"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
			"home":        snapshot.Config.Home,
			"rooms":       snapshot.Config.Rooms,
			"tiles":       snapshot.Config.Tiles,
		}
	case uri == "jute://widgets/visible":
		value = widgetskills.VisibleWidgetsSnapshot(snapshot)
	case strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/context"):
		widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/context")
		value, err = widgetskills.WidgetContext(snapshot, widgetID)
	case uri == "jute://skills":
		value = widgetskills.SkillListSnapshot(snapshot)
	case strings.HasPrefix(uri, "jute://skills/") && strings.HasSuffix(uri, "/context"):
		skillID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://skills/"), "/context")
		value, err = widgetskills.SkillContext(snapshot, skillID, "")
	case strings.HasPrefix(uri, "jute://skills/"):
		skillID := strings.TrimPrefix(uri, "jute://skills/")
		value, err = widgetskills.SkillDefinition(snapshot, skillID)
	case strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/skill"):
		widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/skill")
		value, err = widgetskills.WidgetSkill(snapshot, widgetID)
	default:
		return nil, notFound("resource not found")
	}
	if err != nil {
		if errors.Is(err, widgetskills.ErrNotFound) {
			return nil, notFound("resource not found")
		}
		return nil, internalError()
	}
	text, err := jsonText(value)
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

func (h *Handler) callTool(ctx context.Context, r *http.Request, req toolCallParams) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if rpcErr := requireToolScope(caller, req.Name); rpcErr != nil {
		return nil, rpcErr
	}
	var value any
	switch req.Name {
	case "jute_dashboard_context_get":
		value = widgetskills.DashboardSnapshot(snapshot)
	case "jute_skill_list":
		value = widgetskills.SkillListSnapshot(snapshot)
	case "jute_skill_read_context":
		skillID, widgetID := stringArg(req.Arguments, "skillId"), stringArg(req.Arguments, "widgetInstanceId")
		value, err = widgetskills.SkillContext(snapshot, skillID, widgetID)
	case "jute_skill_invoke_action":
		value, err = widgetskills.InvokeAction(snapshot, stringArg(req.Arguments, "skillId"), stringArg(req.Arguments, "widgetInstanceId"), stringArg(req.Arguments, "actionId"), req.Arguments)
	case "jute_skill_prompt_get":
		text, promptErr := widgetskills.PromptText(snapshot, stringArg(req.Arguments, "skillId"), stringArg(req.Arguments, "promptId"))
		if promptErr != nil {
			err = promptErr
		} else {
			value = map[string]any{"text": text}
		}
	case "jute_display_notification":
		if h.display == nil {
			return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
		}
		notification, actionErr := h.display.Notify(stringArg(req.Arguments, "message"), stringArg(req.Arguments, "severity"))
		if actionErr != nil {
			err = actionErr
		} else {
			value = map[string]any{
				"status":       "queued",
				"eventType":    displayactions.EventNotification,
				"notification": notification,
			}
		}
	case "jute_display_focus_widget":
		if h.display == nil {
			return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
		}
		widgetID := stringArg(req.Arguments, "widgetInstanceId")
		if _, err = widgetskills.WidgetContext(snapshot, widgetID); err == nil {
			var focus displayactions.FocusWidget
			focus, err = h.display.FocusWidget(widgetID, stringArg(req.Arguments, "reason"))
			if err == nil {
				value = map[string]any{
					"status":    "queued",
					"eventType": displayactions.EventFocusWidget,
					"focus":     focus,
				}
			}
		}
	default:
		return nil, notFound("tool not found")
	}
	if err != nil {
		if errors.Is(err, widgetskills.ErrNotFound) {
			return nil, notFound("skill or action not found")
		}
		return nil, invalidParams(err)
	}
	text, err := jsonText(value)
	if err != nil {
		return nil, internalError()
	}
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"structuredContent": value,
		"isError":           false,
	}, nil
}

func (h *Handler) getPrompt(ctx context.Context, r *http.Request, name string, arguments map[string]any) (any, *rpcError) {
	snapshot, err := h.snapshot(ctx)
	if err != nil {
		return nil, internalError()
	}
	caller, rpcErr := h.callerForRequest(snapshot, r)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !caller.has(config.MCPScopeSkillsPromptRead) {
		return nil, missingScope(config.MCPScopeSkillsPromptRead)
	}
	var text string
	switch name {
	case "jute_home_assistant_guidance":
		text = widgetskills.HomeAssistantGuidance()
	case "jute_widget_skill_guidance":
		text, err = widgetskills.PromptText(snapshot, stringArg(arguments, "skillId"), stringArg(arguments, "promptId"))
	default:
		return nil, notFound("prompt not found")
	}
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
			return newCaller("", true, config.DefaultMCPReadScopes()), nil
		}
		return caller{}, unauthorized("mcp caller identity is required")
	}
	for _, agent := range snapshot.Config.Agents {
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
		scopes = config.DefaultMCPReadScopes()
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
	add := func(scope string, value map[string]any) {
		if caller.has(scope) {
			resources = append(resources, value)
		}
	}
	add(config.MCPScopeDashboardRead, resource("jute://dashboard/current", "dashboard-current", "Current Dashboard Context", "Safe current dashboard context and visible Widget Skills."))
	add(config.MCPScopeWidgetsRead, resource("jute://widgets/visible", "widgets-visible", "Visible Widgets", "Visible dashboard widgets and their Widget Skill mappings."))
	add(config.MCPScopeSkillsRead, resource("jute://skills", "widget-skills", "Widget Skills", "Available Widget Skills for this display."))
	add(config.MCPScopeDashboardRead, resource("jute://home/state", "home-state", "Home State", "Normalized non-secret home state summary."))
	for _, skill := range widgetskills.Available(snapshot) {
		if caller.has(config.MCPScopeSkillsRead) {
			resources = append(resources,
				resource("jute://skills/"+skill.SkillID, "skill-"+skill.SkillID, skill.DisplayName+" Skill", skill.Summary),
				resource("jute://widgets/"+skill.WidgetInstanceID+"/skill", "widget-"+skill.WidgetInstanceID+"-skill", skill.WidgetTitle+" Skill", "Widget instance to Widget Skill mapping."),
			)
		}
		if caller.has(config.MCPScopeSkillsContextRead) {
			resources = append(resources,
				resource("jute://skills/"+skill.SkillID+"/context", "skill-"+skill.SkillID+"-context", skill.DisplayName+" Context", "Current public context for "+skill.DisplayName+"."),
			)
			if caller.has(config.MCPScopeWidgetsRead) {
				resources = append(resources, resource("jute://widgets/"+skill.WidgetInstanceID+"/context", "widget-"+skill.WidgetInstanceID+"-context", skill.WidgetTitle+" Context", "Current public Widget Skill context for "+skill.WidgetTitle+"."))
			}
		}
	}
	return resources
}

func resource(uri, name, title, description string) map[string]any {
	return map[string]any{
		"uri":         uri,
		"name":        name,
		"title":       title,
		"description": description,
		"mimeType":    "application/json",
	}
}

func toolsList(caller caller) []map[string]any {
	tools := []map[string]any{}
	add := func(scope string, value map[string]any) {
		if caller.has(scope) {
			tools = append(tools, value)
		}
	}
	add(config.MCPScopeDashboardRead, tool("jute_dashboard_context_get", "Get Dashboard Context", "Return safe current Jute dashboard context.", emptySchema()))
	add(config.MCPScopeSkillsRead, tool("jute_skill_list", "List Widget Skills", "List available Jute Widget Skills.", emptySchema()))
	add(config.MCPScopeSkillsContextRead, tool("jute_skill_read_context", "Read Widget Skill Context", "Read public context for a Widget Skill.", objectSchema(map[string]any{
		"skillId":          map[string]any{"type": "string"},
		"widgetInstanceId": map[string]any{"type": "string"},
	}, []string{"skillId"})))
	add(config.MCPScopeSkillsActionInvoke, tool("jute_skill_invoke_action", "Invoke Widget Skill Action", "Invoke a declared low-risk Widget Skill action through the hub.", objectSchema(map[string]any{
		"skillId":          map[string]any{"type": "string"},
		"widgetInstanceId": map[string]any{"type": "string"},
		"actionId":         map[string]any{"type": "string"},
	}, []string{"skillId", "actionId"})))
	add(config.MCPScopeSkillsPromptRead, tool("jute_skill_prompt_get", "Get Widget Skill Prompt", "Get hub-approved prompt guidance for a Widget Skill.", objectSchema(map[string]any{
		"skillId":  map[string]any{"type": "string"},
		"promptId": map[string]any{"type": "string"},
	}, []string{"skillId", "promptId"})))
	add(config.MCPScopeDisplayWrite, tool("jute_display_notification", "Display Notification", "Show a short hub-sanitized notification on the Jute display.", objectSchema(map[string]any{
		"message":  map[string]any{"type": "string"},
		"severity": map[string]any{"type": "string", "enum": []string{"info", "success", "warning", "error"}},
	}, []string{"message"})))
	add(config.MCPScopeDisplayFocusWidget, tool("jute_display_focus_widget", "Focus Widget", "Ask the Jute display to highlight a visible widget instance.", objectSchema(map[string]any{
		"widgetInstanceId": map[string]any{"type": "string"},
		"reason":           map[string]any{"type": "string"},
	}, []string{"widgetInstanceId"})))
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
	if !caller.has(config.MCPScopeSkillsPromptRead) {
		return []map[string]any{}
	}
	return []map[string]any{
		{
			"name":        "jute_home_assistant_guidance",
			"title":       "Jute Home Assistant Guidance",
			"description": "Guidance for using Jute dashboard context and Widget Skills safely.",
		},
		{
			"name":        "jute_widget_skill_guidance",
			"title":       "Jute Widget Skill Guidance",
			"description": "Guidance for using a specific Widget Skill prompt.",
			"arguments": []map[string]any{
				{"name": "skillId", "description": "Widget Skill ID.", "required": true},
				{"name": "promptId", "description": "Skill prompt ID.", "required": true},
			},
		},
	}
}

func requireResourceScope(caller caller, uri string) *rpcError {
	switch {
	case uri == "jute://dashboard/current", uri == "jute://home/state":
		return requireScope(caller, config.MCPScopeDashboardRead)
	case uri == "jute://widgets/visible":
		return requireScope(caller, config.MCPScopeWidgetsRead)
	case strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/context"):
		if err := requireScope(caller, config.MCPScopeWidgetsRead); err != nil {
			return err
		}
		return requireScope(caller, config.MCPScopeSkillsContextRead)
	case strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/skill"):
		if err := requireScope(caller, config.MCPScopeWidgetsRead); err != nil {
			return err
		}
		return requireScope(caller, config.MCPScopeSkillsRead)
	case uri == "jute://skills":
		return requireScope(caller, config.MCPScopeSkillsRead)
	case strings.HasPrefix(uri, "jute://skills/") && strings.HasSuffix(uri, "/context"):
		return requireScope(caller, config.MCPScopeSkillsContextRead)
	case strings.HasPrefix(uri, "jute://skills/"):
		return requireScope(caller, config.MCPScopeSkillsRead)
	default:
		return nil
	}
}

func requireToolScope(caller caller, name string) *rpcError {
	switch name {
	case "jute_dashboard_context_get":
		return requireScope(caller, config.MCPScopeDashboardRead)
	case "jute_skill_list":
		return requireScope(caller, config.MCPScopeSkillsRead)
	case "jute_skill_read_context":
		return requireScope(caller, config.MCPScopeSkillsContextRead)
	case "jute_skill_invoke_action":
		return requireScope(caller, config.MCPScopeSkillsActionInvoke)
	case "jute_skill_prompt_get":
		return requireScope(caller, config.MCPScopeSkillsPromptRead)
	case "jute_display_notification":
		return requireScope(caller, config.MCPScopeDisplayWrite)
	case "jute_display_focus_widget":
		return requireScope(caller, config.MCPScopeDisplayFocusWidget)
	default:
		return nil
	}
}

func requireScope(caller caller, scope string) *rpcError {
	if caller.has(scope) {
		return nil
	}
	return missingScope(scope)
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
