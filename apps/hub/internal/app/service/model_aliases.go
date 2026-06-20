package service

import "jute-dash/apps/hub/internal/app/model"

type AuthConfig = model.AuthConfig
type AgentConfig = model.AgentConfig
type PublicAgentConfig = model.PublicAgentConfig
type ConversationCreateRequest = model.ConversationCreateRequest
type ConversationTurnRequest = model.ConversationTurnRequest
type Conversation = model.Conversation
type ConversationMessage = model.ConversationMessage
type ConversationDetail = model.ConversationDetail
type AgentCardCache = model.AgentCardCache
type EventKind = model.EventKind
type Event = model.Event
type AgentStatusSummary = model.AgentStatusSummary
type AgentStatusResponse = model.AgentStatusResponse
type MessageRequest = model.MessageRequest
type MessageResponse = model.MessageResponse
type AgentCardCacheEntry = model.AgentCardCacheEntry

const (
	EventTurnStarted    = model.EventTurnStarted
	EventAssistantDelta = model.EventAssistantDelta
	EventStatusChanged  = model.EventStatusChanged
	EventTurnCompleted  = model.EventTurnCompleted
	EventTurnFailed     = model.EventTurnFailed

	MCPScopeDashboardRead      = model.MCPScopeDashboardRead
	MCPScopeWidgetsRead        = model.MCPScopeWidgetsRead
	MCPScopeSkillsRead         = model.MCPScopeSkillsRead
	MCPScopeSkillsContextRead  = model.MCPScopeSkillsContextRead
	MCPScopeSkillsPromptRead   = model.MCPScopeSkillsPromptRead
	MCPScopeSkillsActionInvoke = model.MCPScopeSkillsActionInvoke
	MCPScopeDisplayWrite       = model.MCPScopeDisplayWrite
	MCPScopeDisplayFocusWidget = model.MCPScopeDisplayFocusWidget
)

func DefaultMCPReadScopes() []string { return model.DefaultMCPReadScopes() }
func AllMCPScopes() []string         { return model.AllMCPScopes() }
func IsKnownMCPScope(scope string) bool {
	return model.IsKnownMCPScope(scope)
}

type DisplayConfig = model.DisplayConfig
type DisplayBackground = model.DisplayBackground
type DisplayWidgetChrome = model.DisplayWidgetChrome
type DashboardConfig = model.DashboardConfig
type DashboardScreenConfig = model.DashboardScreenConfig
type DashboardWidgetConfig = model.DashboardWidgetConfig
type WidgetLayout = model.WidgetLayout
type DashboardScreen = model.DashboardScreen
type LayoutVariant = model.LayoutVariant
type WidgetPlacement = model.WidgetPlacement
type SettingFieldType = model.SettingFieldType
type SettingField = model.SettingField
type ConnectionRequirement = model.ConnectionRequirement
type WidgetCatalogItem = model.WidgetCatalogItem
type WidgetInstance = model.WidgetInstance

const (
	BaseColumns        = model.BaseColumns
	LegacyColumnScale  = model.LegacyColumnScale
	WidgetModeUI       = model.WidgetModeUI
	WidgetModeHeadless = model.WidgetModeHeadless
)

var ErrInvalidLayout = model.ErrInvalidLayout

type HomeConfig = model.HomeConfig
type RoomConfig = model.RoomConfig
type TileConfig = model.TileConfig
type HouseholdSettings = model.HouseholdSettings
type SetupStatus = model.SetupStatus
type InitResult = model.InitResult
type AdapterConnection = model.AdapterConnection
type DisplaySettings = model.DisplaySettings

const (
	DefaultHouseholdID     = model.DefaultHouseholdID
	DefaultDeviceProfileID = model.DefaultDeviceProfileID
	DefaultLayoutProfileID = model.DefaultLayoutProfileID
)

var ErrInvalidSettings = model.ErrInvalidSettings

type Config = model.Config
type Settings = model.Settings
type SettingsUpdateRequest = model.SettingsUpdateRequest
type ProviderPack = model.ProviderPack
type ProviderCapabilities = model.ProviderCapabilities
type WakeWordProviderSummary = model.WakeWordProviderSummary
type WakeWordModelSummary = model.WakeWordModelSummary
type TTSVoicesResponse = model.TTSVoicesResponse
type TTSVoice = model.TTSVoice
type StatusResponse = model.StatusResponse

func VoiceState(enabled, muted bool) string { return model.State(enabled, muted) }
