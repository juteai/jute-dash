package agents

import (
	"strings"

	"jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
)

// AuthConfig defines bearer authentication options for agent connections.
type AuthConfig struct {
	Type     string `json:"type"     yaml:"type"`
	EnvToken string `json:"envToken" yaml:"env-token"`
}

// AgentConfig represents configure agent connection parameters.
type AgentConfig struct {
	ID              string      `json:"id"              yaml:"id"`
	Name            string      `json:"name"            yaml:"name"`
	Description     string      `json:"description"     yaml:"description"`
	CardURL         string      `json:"cardUrl"         yaml:"card-url"`
	EndpointURL     string      `json:"endpointUrl"     yaml:"endpoint-url"`
	ProtocolBinding string      `json:"protocolBinding" yaml:"protocol-binding"`
	Enabled         bool        `json:"enabled"         yaml:"enabled"`
	Capabilities    []string    `json:"capabilities"    yaml:"capabilities"`
	MCPScopes       []string    `json:"mcpScopes"       yaml:"mcp-scopes"`
	Auth            *AuthConfig `json:"auth,omitempty"  yaml:"auth,omitempty"`
}

// PublicAgentConfig is a safe, redacted version of AgentConfig for client consumption.
type PublicAgentConfig struct {
	ID              string   `json:"id"              yaml:"id"`
	Name            string   `json:"name"            yaml:"name"`
	Description     string   `json:"description"     yaml:"description"`
	CardURL         string   `json:"cardUrl"         yaml:"card-url"`
	EndpointURL     string   `json:"endpointUrl"     yaml:"endpoint-url"`
	ProtocolBinding string   `json:"protocolBinding" yaml:"protocol-binding"`
	Enabled         bool     `json:"enabled"         yaml:"enabled"`
	Capabilities    []string `json:"capabilities"    yaml:"capabilities"`
	MCPScopes       []string `json:"mcpScopes"       yaml:"mcp-scopes"`
	AuthConfigured  bool     `json:"authConfigured"  yaml:"auth-configured"`
	AuthAvailable   bool     `json:"authAvailable"   yaml:"auth-available"`
}

// ConversationCreateRequest is the payload to start a conversation.
type ConversationCreateRequest struct {
	AgentID     string `json:"agentId"`
	Title       string `json:"title,omitempty"`
	InitialText string `json:"initialText,omitempty"`
}

// ConversationTurnRequest is the payload to post a turn to an agent.
type ConversationTurnRequest struct {
	AgentID string `json:"agentId"`
	Text    string `json:"text"`
}

// Conversation represents a thread with a single agent.
type Conversation struct {
	ID                 string `json:"id"`
	AgentID            string `json:"agentId"`
	Title              string `json:"title"`
	Status             string `json:"status"`
	A2AContextID       string `json:"a2aContextId"`
	LatestTaskID       string `json:"latestTaskId"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	HistoryUnsupported bool   `json:"historyUnsupported,omitempty"`
}

// ConversationMessage represents a single message in the thread.
type ConversationMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	AgentID        string `json:"agentId"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Status         string `json:"status"`
	A2AMessageID   string `json:"a2aMessageId"`
	A2ATaskID      string `json:"a2aTaskId"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// ConversationDetail wraps a conversation and its messages.
type ConversationDetail struct {
	Conversation Conversation          `json:"conversation"`
	Messages     []ConversationMessage `json:"messages"`
}

// AgentCardCache represents discovery settings loaded from the card.
type AgentCardCache struct {
	SelectedEndpointURL       string
	SelectedProtocolBinding   string
	SelectedProtocolVersion   string
	Streaming                 bool
	DashboardContextSupported bool
}

// EventKind represents the type of turn stream events.
type EventKind string

const (
	EventTurnStarted    EventKind = "turn_started"
	EventAssistantDelta EventKind = "assistant_delta"
	EventStatusChanged  EventKind = "status_changed"
	EventTurnCompleted  EventKind = "turn_completed"
	EventTurnFailed     EventKind = "turn_failed"
)

// Event is the turn stream event payload.
type Event struct {
	Kind           EventKind           `json:"kind"`
	ConversationID string              `json:"conversationId"`
	AgentID        string              `json:"agentId"`
	TaskID         string              `json:"taskId,omitempty"`
	Status         string              `json:"status,omitempty"`
	Text           string              `json:"text,omitempty"`
	Append         bool                `json:"append,omitempty"`
	Terminal       bool                `json:"terminal,omitempty"`
	Detail         *ConversationDetail `json:"detail,omitempty"`
	Message        string              `json:"message,omitempty"`
}

// AgentStatusSummary reports overall agent health.
type AgentStatusSummary struct {
	Total                     int `json:"total"`
	Enabled                   int `json:"enabled"`
	Disabled                  int `json:"disabled"`
	Available                 int `json:"available"`
	Unavailable               int `json:"unavailable"`
	DashboardContextSupported int `json:"dashboardContextSupported"`
	MCPScoped                 int `json:"mcpScoped"`
}

// AgentStatusResponse wraps an agent configuration for client status checks.
type AgentStatusResponse struct {
	Agent registry.Agent `json:"agent"`
}

// MessageRequest is the direct API message payload.
type MessageRequest struct {
	AgentID        string `json:"agentId"`
	Text           string `json:"text"`
	ConversationID string `json:"conversationId,omitempty"`
}

// MessageResponse is the response to a direct message.
type MessageResponse struct {
	ConversationID string `json:"conversationId"`
	TaskID         string `json:"taskId,omitempty"`
	AgentID        string `json:"agentId"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

type selectedAgentInterface struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	Streaming       bool
	Extensions      []string
	Metadata        map[string]any
}

type AgentCardCacheEntry struct {
	AgentID                   string
	CardJSON                  string
	CardStatus                string
	CardError                 string
	SelectedEndpointURL       string
	SelectedProtocolBinding   string
	SelectedProtocolVersion   string
	Streaming                 bool
	DashboardContextSupported bool
	Skills                    []a2a.AgentSkill
	FetchedAt                 string
	ExpiresAt                 string
}

// MCP scope constants

const (
	MCPScopeDashboardRead      = "dashboard:read"
	MCPScopeWidgetsRead        = "widgets:read"
	MCPScopeSkillsRead         = "skills:read"
	MCPScopeSkillsContextRead  = "skills:context_read"
	MCPScopeSkillsPromptRead   = "skills:prompt_read"
	MCPScopeSkillsActionInvoke = "skills:action_invoke"
	MCPScopeDisplayWrite       = "display:write_ephemeral"
	MCPScopeDisplayFocusWidget = "display:focus_widget"
)

func DefaultMCPReadScopes() []string {
	return []string{
		MCPScopeDashboardRead,
		MCPScopeWidgetsRead,
		MCPScopeSkillsRead,
		MCPScopeSkillsContextRead,
	}
}

func IsKnownMCPScope(scope string) bool {
	switch strings.TrimSpace(scope) {
	case MCPScopeDashboardRead,
		MCPScopeWidgetsRead,
		MCPScopeSkillsRead,
		MCPScopeSkillsContextRead,
		MCPScopeSkillsPromptRead,
		MCPScopeSkillsActionInvoke,
		MCPScopeDisplayWrite,
		MCPScopeDisplayFocusWidget:
		return true
	default:
		return false
	}
}
