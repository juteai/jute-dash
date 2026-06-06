package dashboard_test

import (
	"context"
	"testing"

	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/mocks"

	"github.com/stretchr/testify/mock"
)

func TestYAMLRepositoryMigratesLegacyFourColumnConfig(t *testing.T) {
	cfg := dashboard.DashboardConfig{Widgets: []dashboard.DashboardWidgetConfig{
		{ID: "clock", Type: "date-time", Title: "Clock", X: 0, Y: 0, W: 2, H: 1, Visible: true},
		{ID: "weather", Type: "weather", Title: "Weather", X: 2, Y: 0, W: 2, H: 1, Visible: true},
	}}
	syncer := mocks.NewSyncer(t)
	syncer.EXPECT().DashboardConfig(mock.Anything).Return(cfg, nil)
	repo := dashboard.NewYAMLRepository(syncer)

	layout, err := repo.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}

	if layout.Widgets[0].W != 6 || layout.Widgets[1].X != 6 || layout.Widgets[1].W != 6 {
		t.Fatalf("legacy config was not scaled to base columns: %+v", layout.Widgets)
	}
	if layout.Widgets[0].MinW != 3 || layout.Widgets[0].Size != "wide" {
		t.Fatalf("catalog defaults were not applied: %+v", layout.Widgets[0])
	}
}

func TestYAMLRepositoryPersistsFrameLayoutMetadata(t *testing.T) {
	cfg := dashboard.DashboardConfig{Widgets: []dashboard.DashboardWidgetConfig{
		{ID: "clock", Type: "date-time", Title: "Clock", X: 0, Y: 0, W: 6, H: 1, Visible: true},
	}}
	var saved dashboard.DashboardConfig
	syncer := mocks.NewSyncer(t)
	syncer.EXPECT().DashboardConfig(mock.Anything).Return(cfg, nil)
	syncer.EXPECT().
		SyncDashboard(mock.Anything, mock.Anything).
		Run(func(_ context.Context, cfg dashboard.DashboardConfig) {
			saved = cfg
		}).
		Return(nil)
	repo := dashboard.NewYAMLRepository(syncer)
	layout := dashboard.WidgetLayout{
		ProfileID: dashboard.DefaultLayoutProfileID,
		Widgets: []dashboard.WidgetInstance{{
			ID:       "clock",
			Kind:     "date-time",
			Title:    "Clock",
			X:        0,
			Y:        0,
			W:        9,
			H:        3,
			MinW:     3,
			MinH:     1,
			Size:     "large",
			Mode:     dashboard.WidgetModeUI,
			Settings: map[string]any{},
			Visible:  true,
		}},
	}

	if _, err := repo.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}

	widget := saved.Widgets[0]
	if widget.W != 9 || widget.H != 3 || widget.MinW != 3 || widget.MinH != 1 || widget.Size != "large" {
		t.Fatalf("frame metadata was not persisted to config: %+v", widget)
	}
}
