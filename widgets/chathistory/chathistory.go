package chathistory

import (
	"context"

	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

const SkillID = "jute.chat_history.current"

type ChatHistoryWidget struct{}

func (w *ChatHistoryWidget) Kind() string {
	return "chat-history"
}

func (w *ChatHistoryWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          "chat-history",
		Name:          "Chat History",
		Description:   "Recent multi-turn conversations and active assistant status.",
		DefaultTitle:  "Assistant Chat",
		DefaultW:      2,
		DefaultH:      2,
		MinW:          1,
		MinH:          1,
		DefaultSize:   "medium",
		Overflow:      "scroll",
		AllowMultiple: false,
	}
}

func (w *ChatHistoryWidget) FetchData(ctx context.Context, settings map[string]any) (any, error) {
	return map[string]any{}, nil
}

func (w *ChatHistoryWidget) Skill() *widgetskills.Definition {
	return chatHistorySkill()
}

func init() {
	widget := &ChatHistoryWidget{}
	widgets.Register(widget)
	widgetskills.Register(*widget.Skill(), chatHistoryContext)
}

func chatHistorySkill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          "chat-history",
		DisplayName:         "Chat History",
		Summary:             "Read available agents, selected agent preference, and conversation history availability.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "agentCount", Type: "integer", Description: "Number of configured agents.", Sensitivity: "public"},
			{Name: "enabledAgentCount", Type: "integer", Description: "Number of enabled agents.", Sensitivity: "public"},
			{Name: "preferredAgentId", Type: "string", Description: "Configured preferred agent ID when present.", Nullable: true, Sensitivity: "public"},
			{Name: "historySource", Type: "string", Description: "Current conversation history source.", Sensitivity: "public"},
			{Name: "agents", Type: "array", Description: "Safe summaries of configured agents.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("read", "Read chat status context", "Return public agent and chat history context."),
		},
		Prompts: []widgetskills.Prompt{{
			ID:      "conversation_status",
			Title:   "Use conversation availability",
			Purpose: "Guide an agent when explaining available agents and conversation state.",
		}},
		SupportedWidgetSizes: []string{"medium", "wide", "large"},
	}
}

func chatHistoryContext(snapshot widgetskills.Snapshot, instanceID string) map[string]any {
	agents := make([]map[string]any, 0, len(snapshot.Agents))
	enabled := 0
	for _, agent := range snapshot.Agents {
		if agent.Enabled {
			enabled++
		}
		agents = append(agents, map[string]any{
			"id":              agent.ID,
			"name":            agent.Name,
			"description":     agent.Description,
			"protocolBinding": agent.ProtocolBinding,
			"enabled":         agent.Enabled,
			"capabilities":    append([]string(nil), agent.Capabilities...),
			"authConfigured":  agent.AuthConfigured,
		})
	}
	preferredAgent := any(nil)
	if snapshot.Config.Voice.PreferredAgentID != "" {
		preferredAgent = snapshot.Config.Voice.PreferredAgentID
	}
	return map[string]any{
		"agentCount":        len(snapshot.Agents),
		"enabledAgentCount": enabled,
		"preferredAgentId":  preferredAgent,
		"historySource":     "agent_tasks",
		"agents":            agents,
	}
}
