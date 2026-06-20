package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardLayoutPutSavesAndEmitsUpdate(t *testing.T) {
	store := &fixtureDashboardStore{
		saveResult: WidgetLayout{ProfileID: "profile-1", Widgets: []WidgetInstance{}},
	}
	var updated WidgetLayout
	controller := NewDashboardController(store, func(layout WidgetLayout) {
		updated = layout
	})
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/widgets/layout",
		strings.NewReader(`{"profileId":"profile-1","widgets":[]}`),
	)
	rec := httptest.NewRecorder()

	controller.handleWidgetLayout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.saved.ProfileID != "profile-1" || updated.ProfileID != "profile-1" {
		t.Fatalf("layout was not saved/emitted: saved=%+v updated=%+v", store.saved, updated)
	}
}

func TestDashboardActiveScreenRejectsBadJSON(t *testing.T) {
	controller := NewDashboardController(&fixtureDashboardStore{}, nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/widgets/layout/active-screen", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	controller.handleWidgetLayoutActiveScreen(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDashboardResetDefaultsProfileID(t *testing.T) {
	store := &fixtureDashboardStore{
		resetResult: WidgetLayout{ProfileID: DefaultLayoutProfileID},
	}
	controller := NewDashboardController(store, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/widgets/layout/reset", nil)
	rec := httptest.NewRecorder()

	controller.handleWidgetLayoutReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.resetProfileID != DefaultLayoutProfileID {
		t.Fatalf("reset profile = %q, want %q", store.resetProfileID, DefaultLayoutProfileID)
	}
	var body WidgetLayout
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.ProfileID != DefaultLayoutProfileID {
		t.Fatalf("response profile = %q", body.ProfileID)
	}
}

type fixtureDashboardStore struct {
	layout         WidgetLayout
	saveResult     WidgetLayout
	resetResult    WidgetLayout
	activeResult   WidgetLayout
	saved          WidgetLayout
	resetProfileID string
	activeScreenID string
}

func (s *fixtureDashboardStore) WidgetLayout(context.Context, string) (WidgetLayout, error) {
	return s.layout, nil
}

func (s *fixtureDashboardStore) SaveWidgetLayout(_ context.Context, layout WidgetLayout) (WidgetLayout, error) {
	s.saved = layout
	return s.saveResult, nil
}

func (s *fixtureDashboardStore) ResetWidgetLayout(_ context.Context, profileID string) (WidgetLayout, error) {
	s.resetProfileID = profileID
	return s.resetResult, nil
}

func (s *fixtureDashboardStore) SetActiveScreen(
	_ context.Context,
	_ string,
	screenID string,
) (WidgetLayout, error) {
	s.activeScreenID = screenID
	return s.activeResult, nil
}
