package widgetactions

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

const testWidgetKind = "dispatcher-test-widget"

var dispatchTestWidget = &testConnectionWidget{}

func registerDispatchTestWidget() {
	widgets.Register(dispatchTestWidget)
}

type fakeLayoutStore struct {
	layout dashboard.WidgetLayout
	err    error
}

func (s fakeLayoutStore) WidgetLayout(_ context.Context, _ string) (dashboard.WidgetLayout, error) {
	return s.layout, s.err
}

func (s fakeLayoutStore) SaveWidgetLayout(
	_ context.Context,
	layout dashboard.WidgetLayout,
) (dashboard.WidgetLayout, error) {
	return layout, s.err
}

type fakeConnectionResolver struct {
	result widgets.ConnectionResolution
}

func (r fakeConnectionResolver) ResolveWidgetConnections(
	_ context.Context,
	_ []widgets.ConnectionRequirement,
	_ map[string]string,
) widgets.ConnectionResolution {
	return r.result
}

type testConnectionWidget struct {
	lastInput widgets.ActionInput
	fail      bool
}

func (w *testConnectionWidget) Kind() string {
	return testWidgetKind
}

func (w *testConnectionWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:                   testWidgetKind,
		Name:                   "Dispatcher Test Widget",
		ConnectionRequirements: w.RequiredConnections(),
	}
}

func (w *testConnectionWidget) FetchData(context.Context, map[string]any) (any, error) {
	return widgets.Unavailable("test.connection_needed", "Connection needed", "Connect this widget."), nil
}

func (w *testConnectionWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:    "jute.dispatcher.test",
		WidgetKind: testWidgetKind,
		Actions: []widgetskills.Action{
			{ID: "do", Title: "Do", SideEffect: "write"},
			{ID: "confirm", Title: "Confirm", SideEffect: "write", RequiresConfirmation: true},
		},
	}
}

func (w *testConnectionWidget) RequiredConnections() []widgets.ConnectionRequirement {
	return []widgets.ConnectionRequirement{{
		Slot:        "account",
		Kind:        "test-account",
		DisplayName: "Test Account",
		Required:    true,
	}}
}

func (w *testConnectionWidget) FetchDataWithConnections(
	_ context.Context,
	_ widgets.RuntimeInput,
) (widgets.RuntimePayload, error) {
	return widgets.OK(map[string]any{"ready": true}), nil
}

func (w *testConnectionWidget) InvokeActionWithConnections(
	_ context.Context,
	input widgets.ActionInput,
) (map[string]any, error) {
	w.lastInput = input
	if w.fail {
		return nil, errors.New("provider failed")
	}
	return map[string]any{
		"status":       "ok",
		"actionId":     input.ActionID,
		"connectionId": input.Connections["account"].ID,
	}, nil
}

func TestDispatcherValidatesActionRequests(t *testing.T) {
	registerDispatchTestWidget()
	dispatchTestWidget.fail = false
	layout := dashboard.WidgetLayout{
		Widgets: []dashboard.WidgetInstance{{
			ID:             "widget-1",
			Kind:           testWidgetKind,
			Visible:        true,
			ConnectionRefs: map[string]string{"account": "account-1"},
		}},
	}
	successResolver := fakeConnectionResolver{
		result: widgets.ConnectionResolution{
			Connections: map[string]widgets.ResolvedConnection{
				"account": {ID: "account-1", Kind: "test-account", Enabled: true},
			},
		},
	}

	tests := []struct {
		name       string
		dispatcher *Dispatcher
		request    Request
		wantStatus int
		wantIssue  string
	}{
		{
			name:       "missing instance",
			dispatcher: NewDispatcher(fakeLayoutStore{layout: dashboard.WidgetLayout{}}, successResolver, nil),
			request:    Request{WidgetInstanceID: "missing", ActionID: "do"},
			wantStatus: http.StatusNotFound,
			wantIssue:  "widget.not_found",
		},
		{
			name: "removed instance",
			dispatcher: NewDispatcher(fakeLayoutStore{layout: dashboard.WidgetLayout{
				Widgets: []dashboard.WidgetInstance{{
					ID:      "widget-1",
					Kind:    testWidgetKind,
					Visible: false,
				}},
			}}, successResolver, nil),
			request:    Request{WidgetInstanceID: "widget-1", ActionID: "do"},
			wantStatus: http.StatusNotFound,
			wantIssue:  "widget.not_found",
		},
		{
			name:       "missing action",
			dispatcher: NewDispatcher(fakeLayoutStore{layout: layout}, successResolver, nil),
			request:    Request{WidgetInstanceID: "widget-1", ActionID: "missing"},
			wantStatus: http.StatusBadRequest,
			wantIssue:  "widget.action_not_found",
		},
		{
			name:       "confirmation required",
			dispatcher: NewDispatcher(fakeLayoutStore{layout: layout}, successResolver, nil),
			request:    Request{WidgetInstanceID: "widget-1", ActionID: "confirm"},
			wantStatus: http.StatusConflict,
			wantIssue:  "widget.action_confirmation_required",
		},
		{
			name: "connection issue",
			dispatcher: NewDispatcher(fakeLayoutStore{layout: layout}, fakeConnectionResolver{
				result: widgets.ConnectionResolution{
					Issue: issuePtr(widgets.Unavailable(
						"connection.missing",
						"Connection needed",
						"Connect this widget.",
					)),
				},
			}, nil),
			request:    Request{WidgetInstanceID: "widget-1", ActionID: "do"},
			wantStatus: http.StatusBadRequest,
			wantIssue:  "connection.missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dispatcher.Dispatch(context.Background(), tt.request)
			if result.HTTPStatus != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, result.HTTPStatus)
			}
			if result.Issue == nil {
				t.Fatal("expected issue")
			}
			if result.Issue.Code != tt.wantIssue {
				t.Fatalf("expected issue %q, got %q", tt.wantIssue, result.Issue.Code)
			}
		})
	}
}

func TestDispatcherInvokesConnectionAwareWidget(t *testing.T) {
	registerDispatchTestWidget()
	dispatchTestWidget.fail = false
	layout := dashboard.WidgetLayout{
		Widgets: []dashboard.WidgetInstance{{
			ID:             "widget-1",
			Kind:           testWidgetKind,
			Visible:        true,
			Settings:       map[string]any{"view": "compact"},
			ConnectionRefs: map[string]string{"account": "account-1"},
		}},
	}
	dispatcher := NewDispatcher(
		fakeLayoutStore{layout: layout},
		fakeConnectionResolver{
			result: widgets.ConnectionResolution{
				Connections: map[string]widgets.ResolvedConnection{
					"account": {ID: "account-1", Kind: "test-account", Enabled: true},
				},
			},
		},
		func(_ context.Context, layout dashboard.WidgetLayout) widgetskills.Snapshot {
			return widgetskills.Snapshot{Layout: widgetskills.WidgetLayout{ProfileID: layout.ProfileID}}
		},
	)

	result := dispatcher.Dispatch(context.Background(), Request{
		WidgetInstanceID: "widget-1",
		ActionID:         "do",
		Arguments:        map[string]any{"amount": 1},
		Actor:            "mcp",
	})

	if result.HTTPStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d issue=%#v", result.HTTPStatus, result.Issue)
	}
	if result.Body["status"] != "ok" {
		t.Fatalf("expected ok body, got %#v", result.Body)
	}
	if result.Body["connectionId"] != "account-1" {
		t.Fatalf("expected resolved connection id, got %#v", result.Body)
	}
	if dispatchTestWidget.lastInput.Actor != "mcp" {
		t.Fatalf("expected actor to be passed through, got %q", dispatchTestWidget.lastInput.Actor)
	}
	if dispatchTestWidget.lastInput.Settings["view"] != "compact" {
		t.Fatalf("expected non-secret settings to be passed through, got %#v", dispatchTestWidget.lastInput.Settings)
	}

	body, err := dispatcher.InvokeWidgetAction(context.Background(), "widget-1", "do", nil, "display", false)
	if err != nil {
		t.Fatalf("InvokeWidgetAction returned error: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok MCP-style body, got %#v", body)
	}
}

func issuePtr(payload widgets.RuntimePayload) *widgets.RuntimePayload {
	return &payload
}
