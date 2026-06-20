package app

import (
	"context"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/apps/hub/internal/app/repository"
	"jute-dash/apps/hub/internal/app/service"

	_ "jute-dash/widgets/chathistory/hub"
	_ "jute-dash/widgets/datetime/hub"
	_ "jute-dash/widgets/markets/hub"
	_ "jute-dash/widgets/rss/hub"
	_ "jute-dash/widgets/spotify/hub"
	_ "jute-dash/widgets/weather/hub"
)

type AgentConfig = model.AgentConfig
type SetupStatus = model.SetupStatus
type WidgetLayout = model.WidgetLayout
type MessageResponse = model.MessageResponse
type Conversation = model.Conversation
type ConversationDetail = model.ConversationDetail
type HouseholdSettings = model.HouseholdSettings
type RoomConfig = model.RoomConfig
type TileConfig = model.TileConfig
type AuthConfig = model.AuthConfig
type VoiceStatusResponse = model.StatusResponse
type VoiceProviderPack = model.ProviderPack
type TTSVoice = model.TTSVoice
type TTSActionResponse = service.TTSActionResponse
type WidgetInstance = model.WidgetInstance
type WidgetPlacement = model.WidgetPlacement
type WidgetCatalogItem = model.WidgetCatalogItem
type DashboardWidgetConfig = model.DashboardWidgetConfig
type DisplayBackground = model.DisplayBackground
type DisplayWidgetChrome = model.DisplayWidgetChrome

const defaultLayoutProfileID = model.DefaultLayoutProfileID

var ErrInvalidLayout = model.ErrInvalidLayout

func DefaultWidgetLayout() WidgetLayout {
	return repository.DefaultWidgetLayout()
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

func WidgetCatalog() []model.WidgetCatalogItem {
	return repository.WidgetCatalog()
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
