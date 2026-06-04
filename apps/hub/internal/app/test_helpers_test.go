package app

import (
	"context"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/voice"

	_ "jute-dash/widgets/chathistory"
	_ "jute-dash/widgets/datetime"
	_ "jute-dash/widgets/markets"
	_ "jute-dash/widgets/rss"
	_ "jute-dash/widgets/weather"
)

type AgentConfig = agents.AgentConfig
type SetupStatus = homestate.SetupStatus
type WidgetLayout = dashboard.WidgetLayout
type MessageResponse = agents.MessageResponse
type Conversation = agents.Conversation
type ConversationDetail = agents.ConversationDetail
type HouseholdSettings = homestate.HouseholdSettings
type RoomConfig = homestate.RoomConfig
type TileConfig = homestate.TileConfig
type AuthConfig = agents.AuthConfig
type VoiceStatusResponse = voice.StatusResponse
type VoiceProviderPack = voice.ProviderPack
type WidgetInstance = dashboard.WidgetInstance
type WidgetCatalogItem = dashboard.WidgetCatalogItem
type DisplayBackground = dashboard.DisplayBackground
type DisplayWidgetChrome = dashboard.DisplayWidgetChrome

const defaultLayoutProfileID = homestate.DefaultLayoutProfileID

var ErrInvalidLayout = dashboard.ErrInvalidLayout

func DefaultWidgetLayout() WidgetLayout {
	return dashboard.DefaultWidgetLayout()
}

func DefaultConfig() config.Config {
	return config.DefaultConfig()
}

func SaveYAML(path string, cfg config.Config) error {
	return config.SaveYAML(path, cfg)
}

func LoadConfig(path string) (config.Config, error) {
	return config.LoadConfig(path)
}

func WidgetCatalog() []dashboard.WidgetCatalogItem {
	return dashboard.WidgetCatalog()
}

func needsSeed(t interface{ Fatalf(string, ...any) }, st *Store) bool {
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	seeded, err := st.IsSeeded(context.Background())
	if err != nil {
		t.Fatalf("IsSeeded() error = %v", err)
	}
	return !seeded
}
