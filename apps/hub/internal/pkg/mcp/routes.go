//nolint:revive // allow unused parameters in MCP routes
package mcp

import (
	"context"
	"strings"
	"time"

	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/pkg/widgetskills"
)

// RouteContext encapsulates context and Jute-specific display dependencies for MCP handlers.
type RouteContext struct {
	Context          context.Context
	Snapshot         widgetskills.Snapshot
	Display          DisplayActions
	ActionDispatcher WidgetActionDispatcher
}

// ResourceRoute defines the interface for an MCP resource route.
type ResourceRoute interface {
	Scope() string
	List(snapshot widgetskills.Snapshot) []map[string]any
	Match(uri string) bool
	Read(ctx RouteContext, uri string) (any, error)
}

// ToolRoute defines the interface for an MCP tool route.
type ToolRoute interface {
	Name() string
	Title() string
	Description() string
	Scope() string
	InputSchema() map[string]any
	Call(ctx RouteContext, args map[string]any) (any, error)
}

// PromptRoute defines the interface for an MCP prompt route.
type PromptRoute interface {
	Name() string
	Title() string
	Description() string
	Arguments() []map[string]any
	Scope() string
	Get(ctx RouteContext, name string, args map[string]any) (string, error)
}

// Helper to construct MCP resource list entries.
func resource(uri, name, title, description string) map[string]any {
	return map[string]any{
		"uri":         uri,
		"name":        name,
		"title":       title,
		"description": description,
		"mimeType":    "application/json",
	}
}

// Concrete Jute Resource Routes

type juteDashboardCurrentRoute struct{}

func (juteDashboardCurrentRoute) Scope() string { return model.MCPScopeDashboardRead }
func (juteDashboardCurrentRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
	return []map[string]any{
		resource(
			"jute://dashboard/current",
			"dashboard-current",
			"Current Dashboard Context",
			"Safe current dashboard context and visible Widget Skills.",
		),
	}
}
func (juteDashboardCurrentRoute) Match(uri string) bool { return uri == "jute://dashboard/current" }
func (juteDashboardCurrentRoute) Read(ctx RouteContext, uri string) (any, error) {
	return widgetskills.DashboardSnapshot(ctx.Snapshot), nil
}

type juteHomeStateRoute struct{}

func (juteHomeStateRoute) Scope() string { return model.MCPScopeDashboardRead }
func (juteHomeStateRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
	return []map[string]any{
		resource("jute://home/state", "home-state", "Home State", "Normalized non-secret home state summary."),
	}
}
func (juteHomeStateRoute) Match(uri string) bool { return uri == "jute://home/state" }
func (juteHomeStateRoute) Read(ctx RouteContext, uri string) (any, error) {
	generatedAt := ctx.Snapshot.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	return map[string]any{
		"schema":      "https://jute.dev/mcp/resources/home-state/v1",
		"generatedAt": generatedAt.UTC().Format(time.RFC3339Nano),
		"home":        ctx.Snapshot.Config.Home,
		"rooms":       ctx.Snapshot.Config.Rooms,
		"tiles":       ctx.Snapshot.Config.Tiles,
	}, nil
}

type juteWidgetsVisibleRoute struct{}

func (juteWidgetsVisibleRoute) Scope() string { return model.MCPScopeWidgetsRead }
func (juteWidgetsVisibleRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
	return []map[string]any{
		resource(
			"jute://widgets/visible",
			"widgets-visible",
			"Visible Widgets",
			"Visible dashboard widgets and their Widget Skill mappings.",
		),
	}
}
func (juteWidgetsVisibleRoute) Match(uri string) bool { return uri == "jute://widgets/visible" }
func (juteWidgetsVisibleRoute) Read(ctx RouteContext, uri string) (any, error) {
	return widgetskills.VisibleWidgetsSnapshot(ctx.Snapshot), nil
}

type juteSkillsRoute struct{}

func (juteSkillsRoute) Scope() string { return model.MCPScopeSkillsRead }
func (juteSkillsRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
	return []map[string]any{
		resource(
			"jute://skills",
			"widget-skills",
			"Widget Skills",
			"Available Widget Skills for this display.",
		),
	}
}
func (juteSkillsRoute) Match(uri string) bool { return uri == "jute://skills" }
func (juteSkillsRoute) Read(ctx RouteContext, uri string) (any, error) {
	return widgetskills.SkillListSnapshot(ctx.Snapshot), nil
}

type juteSkillDetailRoute struct{}

func (juteSkillDetailRoute) Scope() string { return model.MCPScopeSkillsRead }
func (juteSkillDetailRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
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
}
func (juteSkillDetailRoute) Match(uri string) bool {
	if !strings.HasPrefix(uri, "jute://skills/") {
		return false
	}
	rest := strings.TrimPrefix(uri, "jute://skills/")
	return !strings.Contains(rest, "/")
}
func (juteSkillDetailRoute) Read(ctx RouteContext, uri string) (any, error) {
	skillID := strings.TrimPrefix(uri, "jute://skills/")
	skill, err := widgetskills.FindSkill(ctx.Snapshot, skillID, "")
	if err != nil {
		return nil, err
	}
	generatedAt := ctx.Snapshot.GeneratedAt
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
}

type juteWidgetSkillRoute struct{}

func (juteWidgetSkillRoute) Scope() string { return model.MCPScopeSkillsRead }
func (juteWidgetSkillRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
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
}
func (juteWidgetSkillRoute) Match(uri string) bool {
	return strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/skill")
}
func (juteWidgetSkillRoute) Read(ctx RouteContext, uri string) (any, error) {
	widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/skill")
	skill, err := widgetskills.FindSkill(ctx.Snapshot, "", widgetID)
	if err != nil {
		return nil, err
	}
	generatedAt := ctx.Snapshot.GeneratedAt
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
}

type juteSkillContextRoute struct{}

func (juteSkillContextRoute) Scope() string { return model.MCPScopeSkillsContextRead }
func (juteSkillContextRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
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
}
func (juteSkillContextRoute) Match(uri string) bool {
	return strings.HasPrefix(uri, "jute://skills/") && strings.HasSuffix(uri, "/context")
}
func (juteSkillContextRoute) Read(ctx RouteContext, uri string) (any, error) {
	skillID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://skills/"), "/context")
	return widgetskills.SkillContext(ctx.Snapshot, skillID, "")
}

type juteWidgetContextRoute struct{}

func (juteWidgetContextRoute) Scope() string { return model.MCPScopeSkillsContextRead }
func (juteWidgetContextRoute) List(snapshot widgetskills.Snapshot) []map[string]any {
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
}
func (juteWidgetContextRoute) Match(uri string) bool {
	return strings.HasPrefix(uri, "jute://widgets/") && strings.HasSuffix(uri, "/context")
}
func (juteWidgetContextRoute) Read(ctx RouteContext, uri string) (any, error) {
	widgetID := strings.TrimSuffix(strings.TrimPrefix(uri, "jute://widgets/"), "/context")
	return widgetskills.WidgetContext(ctx.Snapshot, widgetID)
}

// Concrete Jute Tool Routes

type juteDashboardContextGetTool struct{}

func (juteDashboardContextGetTool) Name() string  { return "jute_dashboard_context_get" }
func (juteDashboardContextGetTool) Title() string { return "Get Dashboard Context" }
func (juteDashboardContextGetTool) Description() string {
	return "Return safe current Jute dashboard context."
}
func (juteDashboardContextGetTool) Scope() string { return model.MCPScopeDashboardRead }
func (juteDashboardContextGetTool) InputSchema() map[string]any {
	return emptySchema()
}
func (juteDashboardContextGetTool) Call(ctx RouteContext, args map[string]any) (any, error) {
	route := juteDashboardCurrentRoute{}
	return route.Read(ctx, "jute://dashboard/current")
}

type juteSkillListTool struct{}

func (juteSkillListTool) Name() string        { return "jute_skill_list" }
func (juteSkillListTool) Title() string       { return "List Widget Skills" }
func (juteSkillListTool) Description() string { return "List available Jute Widget Skills." }
func (juteSkillListTool) Scope() string       { return model.MCPScopeSkillsRead }
func (juteSkillListTool) InputSchema() map[string]any {
	return emptySchema()
}
func (juteSkillListTool) Call(ctx RouteContext, args map[string]any) (any, error) {
	route := juteSkillsRoute{}
	return route.Read(ctx, "jute://skills")
}

type juteSkillReadContextTool struct{}

func (juteSkillReadContextTool) Name() string  { return "jute_skill_read_context" }
func (juteSkillReadContextTool) Title() string { return "Read Widget Skill Context" }
func (juteSkillReadContextTool) Description() string {
	return "Read public context for an exact Widget Skill ID returned by jute_skill_list."
}
func (juteSkillReadContextTool) Scope() string { return model.MCPScopeSkillsContextRead }
func (juteSkillReadContextTool) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"skillId": map[string]any{
			"type":        "string",
			"description": "Exact Widget Skill ID returned by jute_skill_list.",
		},
		"widgetInstanceId": map[string]any{
			"type":        "string",
			"description": "Optional exact widget instance ID returned by jute_skill_list.",
		},
	}, []string{"skillId"})
}
func (juteSkillReadContextTool) Call(ctx RouteContext, args map[string]any) (any, error) {
	skillID, widgetID := stringArg(args, "skillId"), stringArg(args, "widgetInstanceId")
	return widgetskills.SkillContext(ctx.Snapshot, skillID, widgetID)
}

type juteSkillInvokeActionTool struct{}

func (juteSkillInvokeActionTool) Name() string  { return "jute_skill_invoke_action" }
func (juteSkillInvokeActionTool) Title() string { return "Invoke Widget Skill Action" }
func (juteSkillInvokeActionTool) Description() string {
	return "Invoke an exact declared Widget Skill action through the hub. Call jute_skill_list first; do not invent generic skill IDs such as music_player, stocks, shares, or market_prices."
}
func (juteSkillInvokeActionTool) Scope() string { return model.MCPScopeSkillsActionInvoke }
func (juteSkillInvokeActionTool) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"skillId": map[string]any{
			"type":        "string",
			"description": "Exact Widget Skill ID returned by jute_skill_list, for example jute.spotify.control or jute.markets.current.",
		},
		"widgetInstanceId": map[string]any{
			"type":        "string",
			"description": "Optional exact widget instance ID returned by jute_skill_list.",
		},
		"actionId": map[string]any{
			"type":        "string",
			"description": "Exact action ID declared by the selected Widget Skill.",
		},
		"arguments": map[string]any{
			"type":                 "object",
			"description":          "Action-specific arguments declared by the Widget Skill actionDetails/inputSchema returned by jute_skill_list.",
			"additionalProperties": true,
		},
		"confirmed": map[string]any{"type": "boolean"},
	}, []string{"skillId", "actionId"})
}
func (juteSkillInvokeActionTool) Call(ctx RouteContext, args map[string]any) (any, error) {
	skillID := stringArg(args, "skillId")
	widgetID := stringArg(args, "widgetInstanceId")
	actionID := stringArg(args, "actionId")
	actionArgs := skillActionArguments(args)
	if ctx.ActionDispatcher != nil {
		skill, err := widgetskills.FindSkill(ctx.Snapshot, skillID, widgetID)
		if err != nil {
			return nil, err
		}
		confirmed, _ := args["confirmed"].(bool)
		return ctx.ActionDispatcher.InvokeWidgetAction(
			ctx.Context,
			skill.WidgetInstanceID,
			actionID,
			actionArgs,
			"mcp",
			confirmed,
		)
	}
	return widgetskills.InvokeAction(
		ctx.Context,
		ctx.Snapshot,
		skillID,
		widgetID,
		actionID,
		actionArgs,
	)
}

func skillActionArguments(args map[string]any) map[string]any {
	if raw, ok := args["arguments"].(map[string]any); ok {
		return raw
	}
	cleaned := make(map[string]any, len(args))
	for key, value := range args {
		switch key {
		case "skillId", "widgetInstanceId", "actionId", "confirmed":
			continue
		default:
			cleaned[key] = value
		}
	}
	return cleaned
}

type juteSkillPromptGetTool struct{}

func (juteSkillPromptGetTool) Name() string  { return "jute_skill_prompt_get" }
func (juteSkillPromptGetTool) Title() string { return "Get Widget Skill Prompt" }
func (juteSkillPromptGetTool) Description() string {
	return "Get hub-approved prompt guidance for a Widget Skill."
}
func (juteSkillPromptGetTool) Scope() string { return model.MCPScopeSkillsPromptRead }
func (juteSkillPromptGetTool) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"skillId":  map[string]any{"type": "string"},
		"promptId": map[string]any{"type": "string"},
	}, []string{"skillId", "promptId"})
}
func (juteSkillPromptGetTool) Call(ctx RouteContext, args map[string]any) (any, error) {
	text, err := widgetskills.PromptText(
		ctx.Snapshot,
		stringArg(args, "skillId"),
		stringArg(args, "promptId"),
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"text": text}, nil
}

type juteDisplayNotificationTool struct{}

func (juteDisplayNotificationTool) Name() string  { return "jute_display_notification" }
func (juteDisplayNotificationTool) Title() string { return "Display Notification" }
func (juteDisplayNotificationTool) Description() string {
	return "Show a short hub-sanitized notification on the Jute display."
}
func (juteDisplayNotificationTool) Scope() string { return model.MCPScopeDisplayWrite }
func (juteDisplayNotificationTool) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"message":  map[string]any{"type": "string"},
		"severity": map[string]any{"type": "string", "enum": []string{"info", "success", "warning", "error"}},
	}, []string{"message"})
}
func (juteDisplayNotificationTool) Call(ctx RouteContext, args map[string]any) (any, error) {
	if ctx.Display == nil {
		return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
	}
	notification, err := ctx.Display.Notify(
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
}

type juteDisplayFocusWidgetTool struct{}

func (juteDisplayFocusWidgetTool) Name() string  { return "jute_display_focus_widget" }
func (juteDisplayFocusWidgetTool) Title() string { return "Focus Widget" }
func (juteDisplayFocusWidgetTool) Description() string {
	return "Ask the Jute display to highlight a visible widget instance."
}
func (juteDisplayFocusWidgetTool) Scope() string { return model.MCPScopeDisplayFocusWidget }
func (juteDisplayFocusWidgetTool) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"widgetInstanceId": map[string]any{"type": "string"},
		"reason":           map[string]any{"type": "string"},
	}, []string{"widgetInstanceId"})
}
func (juteDisplayFocusWidgetTool) Call(ctx RouteContext, args map[string]any) (any, error) {
	if ctx.Display == nil {
		return nil, &rpcError{Code: -32005, Message: "display actions are unavailable"}
	}
	widgetID := stringArg(args, "widgetInstanceId")
	if _, err := widgetskills.WidgetContext(ctx.Snapshot, widgetID); err != nil {
		return nil, err
	}
	focus, err := ctx.Display.FocusWidget(widgetID, stringArg(args, "reason"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":    "queued",
		"eventType": displayactions.EventFocusWidget,
		"focus":     focus,
	}, nil
}

// Concrete Jute Prompt Routes

type juteHomeAssistantGuidancePrompt struct{}

func (juteHomeAssistantGuidancePrompt) Name() string  { return "jute_home_assistant_guidance" }
func (juteHomeAssistantGuidancePrompt) Title() string { return "Jute Home Assistant Guidance" }
func (juteHomeAssistantGuidancePrompt) Description() string {
	return "Guidance for using Jute dashboard context and Widget Skills safely."
}
func (juteHomeAssistantGuidancePrompt) Scope() string { return model.MCPScopeSkillsPromptRead }
func (juteHomeAssistantGuidancePrompt) Arguments() []map[string]any {
	return nil
}
func (juteHomeAssistantGuidancePrompt) Get(ctx RouteContext, name string, args map[string]any) (string, error) {
	return widgetskills.HomeAssistantGuidance(), nil
}

type juteWidgetSkillGuidancePrompt struct{}

func (juteWidgetSkillGuidancePrompt) Name() string  { return "jute_widget_skill_guidance" }
func (juteWidgetSkillGuidancePrompt) Title() string { return "Jute Widget Skill Guidance" }
func (juteWidgetSkillGuidancePrompt) Description() string {
	return "Guidance for using a specific Widget Skill prompt."
}
func (juteWidgetSkillGuidancePrompt) Scope() string { return model.MCPScopeSkillsPromptRead }
func (juteWidgetSkillGuidancePrompt) Arguments() []map[string]any {
	return []map[string]any{
		{"name": "skillId", "description": "Widget Skill ID.", "required": true},
		{"name": "promptId", "description": "Skill prompt ID.", "required": true},
	}
}
func (juteWidgetSkillGuidancePrompt) Get(ctx RouteContext, name string, args map[string]any) (string, error) {
	return widgetskills.PromptText(ctx.Snapshot, stringArg(args, "skillId"), stringArg(args, "promptId"))
}

//nolint:gochecknoglobals // static MCP routes tables
var (
	ResourceRoutes = []ResourceRoute{
		juteDashboardCurrentRoute{},
		juteHomeStateRoute{},
		juteWidgetsVisibleRoute{},
		juteSkillsRoute{},
		juteSkillDetailRoute{},
		juteWidgetSkillRoute{},
		juteSkillContextRoute{},
		juteWidgetContextRoute{},
	}

	ToolRoutes = []ToolRoute{
		juteDashboardContextGetTool{},
		juteSkillListTool{},
		juteSkillReadContextTool{},
		juteSkillInvokeActionTool{},
		juteSkillPromptGetTool{},
		juteDisplayNotificationTool{},
		juteDisplayFocusWidgetTool{},
	}

	PromptRoutes = []PromptRoute{
		juteHomeAssistantGuidancePrompt{},
		juteWidgetSkillGuidancePrompt{},
	}
)
