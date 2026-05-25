package chathistory

import (
	"context"
	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

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
	return nil
}

func init() {
	widgets.Register(&ChatHistoryWidget{})
}
