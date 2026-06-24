package controller

import "jute-dash/apps/hub/internal/app/model"

type AgentConfig = model.AgentConfig
type PublicAgentConfig = model.PublicAgentConfig
type ConversationCreateRequest = model.ConversationCreateRequest
type ConversationTurnRequest = model.ConversationTurnRequest
type Conversation = model.Conversation
type ConversationMessage = model.ConversationMessage
type ConversationDetail = model.ConversationDetail
type AgentStatusSummary = model.AgentStatusSummary
type AgentStatusResponse = model.AgentStatusResponse
type MessageRequest = model.MessageRequest
type MessageResponse = model.MessageResponse

type WidgetLayout = model.WidgetLayout
type DashboardScreen = model.DashboardScreen
type LayoutVariant = model.LayoutVariant
type WidgetPlacement = model.WidgetPlacement
type WidgetCatalogItem = model.WidgetCatalogItem
type WidgetInstance = model.WidgetInstance

const DefaultLayoutProfileID = model.DefaultLayoutProfileID

var ErrInvalidLayout = model.ErrInvalidLayout

type HomeConfig = model.HomeConfig
type RoomConfig = model.RoomConfig
type TileConfig = model.TileConfig
type HouseholdSettings = model.HouseholdSettings
type SetupStatus = model.SetupStatus
type AdapterConnection = model.AdapterConnection
type DisplaySettings = model.DisplaySettings

var ErrInvalidSettings = model.ErrInvalidSettings

type Settings = model.Settings
type SettingsUpdateRequest = model.SettingsUpdateRequest
type ProviderPack = model.ProviderPack
type TTSVoicesResponse = model.TTSVoicesResponse
type TTSVoice = model.TTSVoice
type StatusResponse = model.StatusResponse

func StatusFromSettings(settings Settings) StatusResponse {
	return model.StatusFromSettings(settings)
}
