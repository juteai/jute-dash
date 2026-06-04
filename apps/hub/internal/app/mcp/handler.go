//nolint:revive // allow unused parameters in MCP routers
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
	"time"

	"jute-dash/apps/hub/internal/app/agents"
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

type Handler struct {
	cfg     Config
	version string

	provider SnapshotProvider
	display  DisplayActions
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
	for _, router := range resourceRouters {
		if router.Match(uri) {
			if !caller.has(router.Scope) {
				return nil, missingScope(router.Scope)
			}
			val, err := router.Read(ctx, snapshot, uri)
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
	for _, router := range toolRouters {
		if router.Name == req.Name {
			if !caller.has(router.Scope) {
				return nil, missingScope(router.Scope)
			}
			val, err := router.Call(ctx, h, snapshot, req.Arguments)
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
	if !caller.has(agents.MCPScopeSkillsPromptRead) {
		return nil, missingScope(agents.MCPScopeSkillsPromptRead)
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

type ResourceRouter struct {
	Scope string
	List  func(snapshot widgetskills.Snapshot) []map[string]any
	Match func(uri string) bool
	Read  func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error)
}

type ToolRouter struct {
	Name        string
	Title       string
	Description string
	Scope       string
	InputSchema map[string]any
	Call        func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error)
}

//nolint:gochecknoglobals // static resource routers table
var resourceRouters = []ResourceRouter{
	{
		Scope: agents.MCPScopeDashboardRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource(
					"jute://dashboard/current",
					"dashboard-current",
					"Current Dashboard Context",
					"Safe current dashboard context and visible Widget Skills.",
				),
			}
		},
		Match: func(uri string) bool { return uri == "jute://dashboard/current" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			return widgetskills.DashboardSnapshot(snapshot), nil
		},
	},
	{
		Scope: agents.MCPScopeDashboardRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource("jute://home/state", "home-state", "Home State", "Normalized non-secret home state summary."),
			}
		},
		Match: func(uri string) bool { return uri == "jute://home/state" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			generatedAt := snapshot.GeneratedAt
			if generatedAt.IsZero() {
				generatedAt = time.Now().UTC()
			}
			return map[string]any{
				"schema":      "https://jute.dev/mcp/resources/home-state/v1",
				"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
				"home":        snapshot.Config.Home,
				"rooms":       snapshot.Config.Rooms,
				"tiles":       snapshot.Config.Tiles,
			}, nil
		},
	},
	{
		Scope: agents.MCPScopeWidgetsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource(
					"jute://widgets/visible",
					"widgets-visible",
					"Visible Widgets",
					"Visible dashboard widgets and their Widget Skill mappings.",
				),
			}
		},
		Match: func(uri string) bool { return uri == "jute://widgets/visible" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			return widgetskills.VisibleWidgetsSnapshot(snapshot), nil
		},
	},
	{
		Scope: agents.MCPScopeSkillsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			return []map[string]any{
				resource(
					"jute://skills",
					"widget-skills",
					"Widget Skills",
					"Available Widget Skills for this display.",
				),
			}
		},
		Match: func(uri string) bool { return uri == "jute://skills" },
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			return widgetskills.SkillListSnapshot(snapshot), nil
		},
	},
	{
		Scope: agents.MCPScopeSkillsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://skills/"+skill.SkillID,
					"skill-"+skill.SkillID,
					skill.DisplayName+" Skill",
					skill.Summary,
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			if !strings.HasPrefix(uri, "jute://skills/") {
				return false
			}
			rest := strings.TrimPrefix(uri, "jute://skills/")
			return !strings.Contains(rest, "/")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			skillID := strings.TrimPrefix(uri, "jute://skills/")
			skill, err := widgetskills.FindSkill(snapshot, skillID, "")
			if err != nil {
				return nil, err
			}
			generatedAt := snapshot.GeneratedAt
			if generatedAt.IsZero() {
				generatedAt = time.Now().UTC()
			}
			return map[string]any{
				"schema":      "https://jute.dev/mcp/resources/widget-skills/v1",
				"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
				"skill":       skill,
				"contextUri":  "jute://skills/" + skill.SkillID + "/context",
				"actions":     skill.Actions,
				"prompts":     skill.Prompts,
			}, nil
		},
	},
	{
		Scope: agents.MCPScopeSkillsRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://widgets/"+skill.WidgetInstanceID+"/skill",
					"widget-"+skill.WidgetInstanceID+"-skill",
					skill.WidgetTitle+" Skill",
					"Widget instance to Widget Skill mapping.",
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			return strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/skill")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/skill")
			skill, err := widgetskills.FindSkill(snapshot, "", widgetID)
			if err != nil {
				return nil, err
			}
			generatedAt := snapshot.GeneratedAt
			if generatedAt.IsZero() {
				generatedAt = time.Now().UTC()
			}
			return map[string]any{
				"schema":      "https://jute.dev/mcp/resources/widget-skills/v1",
				"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
				"skill":       skill,
				"contextUri":  "jute://skills/" + skill.SkillID + "/context",
				"actions":     skill.Actions,
				"prompts":     skill.Prompts,
			}, nil
		},
	},
	{
		Scope: agents.MCPScopeSkillsContextRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://skills/"+skill.SkillID+"/context",
					"skill-"+skill.SkillID+"-context",
					skill.DisplayName+" Context",
					"Current public context for "+skill.DisplayName+".",
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			return strings.HasPrefix(uri, "jute://skills/") && strings.HasSuffix(uri, "/context")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			skillID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://skills/"), "/context")
			return widgetskills.SkillContext(snapshot, skillID, "")
		},
	},
	{
		Scope: agents.MCPScopeSkillsContextRead,
		List: func(snapshot widgetskills.Snapshot) []map[string]any {
			resources := []map[string]any{}
			for _, skill := range widgetskills.Available(snapshot) {
				resources = append(resources, resource(
					"jute://widgets/"+skill.WidgetInstanceID+"/context",
					"widget-"+skill.WidgetInstanceID+"-context",
					skill.WidgetTitle+" Context",
					"Current public Widget Skill context for "+skill.WidgetTitle+".",
				))
			}
			return resources
		},
		Match: func(uri string) bool {
			return strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/context")
		},
		Read: func(ctx context.Context, snapshot widgetskills.Snapshot, uri string) (any, error) {
			widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/context")
			return widgetskills.WidgetContext(snapshot, widgetID)
		},
	},
}

//nolint:gochecknoglobals // static tool routers table
var toolRouters = []ToolRouter{
	{
		Name:        "jute_dashboard_context_get",
		Title:       "Get Dashboard Context",
		Description: "Return safe current Jute dashboard context.",
		Scope:       agents.MCPScopeDashboardRead,
		InputSchema: emptySchema(),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			for _, r := range resourceRouters {
				if r.Match("jute://dashboard/current") {
					return r.Read(ctx, snapshot, "jute://dashboard/current")
				}
			}
			return nil, errors.New("dashboard context resource not found")
		},
	},
	{
		Name:        "jute_skill_list",
		Title:       "List Widget Skills",
		Description: "List available Jute Widget Skills.",
		Scope:       agents.MCPScopeSkillsRead,
		InputSchema: emptySchema(),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			for _, r := range resourceRouters {
				if r.Match("jute://skills") {
					return r.Read(ctx, snapshot, "jute://skills")
				}
			}
			return nil, errors.New("skills resource not found")
		},
	},
	{
		Name:        "jute_skill_read_context",
		Title:       "Read Widget Skill Context",
		Description: "Read public context for a Widget Skill.",
		Scope:       agents.MCPScopeSkillsContextRead,
		InputSchema: objectSchema(map[string]any{
			"skillId":          map[string]any{"type": "string"},
			"widgetInstanceId": map[string]any{"type": "string"},
		}, []string{"skillId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			skillID, widgetID := stringArg(args, "skillId"), stringArg(args, "widgetInstanceId")
			return widgetskills.SkillContext(snapshot, skillID, widgetID)
		},
	},
	{
		Name:        "jute_skill_invoke_action",
		Title:       "Invoke Widget Skill Action",
		Description: "Invoke a declared low-risk Widget Skill action through the hub.",
		Scope:       agents.MCPScopeSkillsActionInvoke,
		InputSchema: objectSchema(map[string]any{
			"skillId":          map[string]any{"type": "string"},
			"widgetInstanceId": map[string]any{"type": "string"},
			"actionId":         map[string]any{"type": "string"},
		}, []string{"skillId", "actionId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			return widgetskills.InvokeAction(
				snapshot,
				stringArg(args, "skillId"),
				stringArg(args, "widgetInstanceId"),
				stringArg(args, "actionId"),
				args,
			)
		},
	},
	{
		Name:        "jute_skill_prompt_get",
		Title:       "Get Widget Skill Prompt",
		Description: "Get hub-approved prompt guidance for a Widget Skill.",
		Scope:       agents.MCPScopeSkillsPromptRead,
		InputSchema: objectSchema(map[string]any{
			"skillId":  map[string]any{"type": "string"},
			"promptId": map[string]any{"type": "string"},
		}, []string{"skillId", "promptId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			text, err := widgetskills.PromptText(
				snapshot,
				stringArg(args, "skillId"),
				stringArg(args, "promptId"),
			)
			if err != nil {
				return nil, err
			}
			return map[string]any{"text": text}, nil
		},
	},
	{
		Name:        "jute_display_notification",
		Title:       "Display Notification",
		Description: "Show a short hub-sanitized notification on the Jute display.",
		Scope:       agents.MCPScopeDisplayWrite,
		InputSchema: objectSchema(map[string]any{
			"message":  map[string]any{"type": "string"},
			"severity": map[string]any{"type": "string", "enum": []string{"info", "success", "warning", "error"}},
		}, []string{"message"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			if h.display == nil {
				return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
			}
			notification, err := h.display.Notify(
				stringArg(args, "message"),
				stringArg(args, "severity"),
			)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"status":       "queued",
				"eventType":    displayactions.EventNotification,
				"notification": notification,
			}, nil
		},
	},
	{
		Name:        "jute_display_focus_widget",
		Title:       "Focus Widget",
		Description: "Ask the Jute display to highlight a visible widget instance.",
		Scope:       agents.MCPScopeDisplayFocusWidget,
		InputSchema: objectSchema(map[string]any{
			"widgetInstanceId": map[string]any{"type": "string"},
			"reason":           map[string]any{"type": "string"},
		}, []string{"widgetInstanceId"}),
		Call: func(ctx context.Context, h *Handler, snapshot widgetskills.Snapshot, args map[string]any) (any, error) {
			if h.display == nil {
				return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
			}
			widgetID := stringArg(args, "widgetInstanceId")
			if _, err := widgetskills.WidgetContext(snapshot, widgetID); err != nil {
				return nil, err
			}
			focus, err := h.display.FocusWidget(widgetID, stringArg(args, "reason"))
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"status":    "queued",
				"eventType": displayactions.EventFocusWidget,
				"focus":     focus,
			}, nil
		},
	},
}

func resourcesList(snapshot widgetskills.Snapshot, caller caller) []map[string]any {
	resources := []map[string]any{}
	for _, router := range resourceRouters {
		if caller.has(router.Scope) {
			resources = append(resources, router.List(snapshot)...)
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
	for _, router := range toolRouters {
		if caller.has(router.Scope) {
			tools = append(tools, tool(
				router.Name,
				router.Title,
				router.Description,
				router.InputSchema,
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
	if !caller.has(agents.MCPScopeSkillsPromptRead) {
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
