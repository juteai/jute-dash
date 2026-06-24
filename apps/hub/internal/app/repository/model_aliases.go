package repository

import "jute-dash/apps/hub/internal/app/model"

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
type WidgetCatalogItem = model.WidgetCatalogItem
type WidgetInstance = model.WidgetInstance

const (
	DefaultHouseholdID = model.DefaultHouseholdID
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

var ErrInvalidSettings = model.ErrInvalidSettings

type Settings = model.Settings
type SettingsUpdateRequest = model.SettingsUpdateRequest
type Config = model.Config
type ProviderPack = model.ProviderPack
type ProviderCapabilities = model.ProviderCapabilities
type WakeWordProviderSummary = model.WakeWordProviderSummary
type WakeWordModelSummary = model.WakeWordModelSummary
type TTSVoicesResponse = model.TTSVoicesResponse
type TTSVoice = model.TTSVoice

func DefaultConfig() Config { return model.DefaultConfig() }
func Validate(cfg Config) []string {
	return model.Validate(cfg)
}
