package service

import (
	"context"
	"net/http"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

type ActionLayoutStore interface {
	WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error)
	SaveWidgetLayout(ctx context.Context, layout WidgetLayout) (WidgetLayout, error)
}

type SnapshotBuilder func(ctx context.Context, layout WidgetLayout) widgetskills.Snapshot

type WidgetActionDispatcher struct {
	layoutStore ActionLayoutStore
	resolver    widgets.ConnectionResolver
	snapshot    SnapshotBuilder
}

type WidgetActionRequest struct {
	WidgetInstanceID string
	ActionID         string
	Arguments        map[string]any
	Actor            string
	Confirmed        bool
}

type WidgetActionResult struct {
	Body       map[string]any
	Issue      *widgets.UserFacingIssue
	HTTPStatus int
}

func NewWidgetActionDispatcher(
	layoutStore ActionLayoutStore,
	resolver widgets.ConnectionResolver,
	snapshot SnapshotBuilder,
) *WidgetActionDispatcher {
	return &WidgetActionDispatcher{
		layoutStore: layoutStore,
		resolver:    resolver,
		snapshot:    snapshot,
	}
}

func (d *WidgetActionDispatcher) Dispatch(ctx context.Context, req WidgetActionRequest) WidgetActionResult {
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}
	layout, err := d.layoutStore.WidgetLayout(ctx, "")
	if err != nil {
		return issueResult(
			http.StatusInternalServerError,
			"widget.layout_unavailable",
			"Widget unavailable",
			"Jute cannot load the widget layout.",
			"error",
		)
	}
	inst := findWidget(layout, req.WidgetInstanceID)
	if inst == nil {
		return issueResult(
			http.StatusNotFound,
			"widget.not_found",
			"Widget not found",
			"This widget is no longer available.",
			"warning",
		)
	}
	provider, exists := widgets.Get(inst.Kind)
	if !exists {
		return issueResult(
			http.StatusNotFound,
			"widget.kind_not_registered",
			"Widget unavailable",
			"This widget is not available in the Hub.",
			"warning",
		)
	}
	action, ok := findWidgetAction(provider, req.ActionID)
	if !ok {
		return issueResult(
			http.StatusBadRequest,
			"widget.action_not_found",
			"Action unavailable",
			"This action is not supported by the widget.",
			"warning",
		)
	}
	if action.RequiresConfirmation && !req.Confirmed {
		return issueResult(
			http.StatusConflict,
			"widget.action_confirmation_required",
			"Confirmation needed",
			"Confirm this action before Jute runs it.",
			"warning",
		)
	}

	var snapshot widgetskills.Snapshot
	if d.snapshot != nil {
		snapshot = d.snapshot(ctx, layout)
	}
	if mutatingWidget, ok := provider.(widgets.SettingsMutatingActionWidget); ok {
		result, err := mutatingWidget.InvokeActionWithSettings(ctx, widgets.ActionInput{
			RuntimeInput: widgets.RuntimeInput{
				InstanceID:     inst.ID,
				Settings:       cloneSettings(inst.Settings),
				ConnectionRefs: cloneConnectionRefs(inst.ConnectionRefs),
			},
			Snapshot:  snapshot,
			ActionID:  req.ActionID,
			Arguments: req.Arguments,
			Actor:     req.Actor,
		})
		if err != nil {
			return issueResult(
				http.StatusInternalServerError,
				"widget.action_failed",
				"Action failed",
				"Jute could not complete this widget action.",
				"error",
			)
		}
		if result.Body == nil {
			result.Body = map[string]any{}
		}
		if result.Settings != nil {
			inst.Settings = result.Settings
			saved, err := d.layoutStore.SaveWidgetLayout(ctx, layout)
			if err != nil {
				return issueResult(
					http.StatusInternalServerError,
					"widget.action_save_failed",
					"Action not saved",
					"Jute completed the action but could not save the widget state.",
					"error",
				)
			}
			if savedInst := findWidget(saved, req.WidgetInstanceID); savedInst != nil {
				result.Body["settings"] = cloneSettings(savedInst.Settings)
			}
		}
		return WidgetActionResult{Body: result.Body, HTTPStatus: http.StatusOK}
	}
	if connectionWidget, ok := provider.(widgets.ConnectionAwareActionWidget); ok {
		resolution := widgets.ConnectionResolution{
			Connections: map[string]widgets.ResolvedConnection{},
		}
		if d.resolver != nil {
			resolution = d.resolver.ResolveWidgetConnections(
				ctx,
				connectionWidget.RequiredConnections(),
				inst.ConnectionRefs,
			)
		}
		if resolution.Issue != nil {
			return WidgetActionResult{Issue: resolution.Issue.Issue, HTTPStatus: http.StatusBadRequest}
		}
		result, err := connectionWidget.InvokeActionWithConnections(ctx, widgets.ActionInput{
			RuntimeInput: widgets.RuntimeInput{
				InstanceID:     inst.ID,
				Settings:       cloneSettings(inst.Settings),
				ConnectionRefs: cloneConnectionRefs(inst.ConnectionRefs),
				Connections:    resolution.Connections,
			},
			Snapshot:  snapshot,
			ActionID:  req.ActionID,
			Arguments: req.Arguments,
			Actor:     req.Actor,
		})
		if err != nil {
			return issueResult(
				http.StatusInternalServerError,
				"widget.action_failed",
				"Action failed",
				"Jute could not complete this widget action.",
				"error",
			)
		}
		return WidgetActionResult{Body: result, HTTPStatus: http.StatusOK}
	}
	actionWidget, ok := provider.(widgets.ActionWidget)
	if !ok {
		return issueResult(
			http.StatusBadRequest,
			"widget.action_unsupported",
			"Action unavailable",
			"This widget does not support actions.",
			"warning",
		)
	}
	result, err := actionWidget.InvokeAction(ctx, snapshot, req.WidgetInstanceID, req.ActionID, req.Arguments)
	if err != nil {
		return issueResult(
			http.StatusInternalServerError,
			"widget.action_failed",
			"Action failed",
			"Jute could not complete this widget action.",
			"error",
		)
	}
	return WidgetActionResult{Body: result, HTTPStatus: http.StatusOK}
}

func (d *WidgetActionDispatcher) InvokeWidgetAction(
	ctx context.Context,
	widgetInstanceID string,
	actionID string,
	arguments map[string]any,
	actor string,
	confirmed bool,
) (map[string]any, error) {
	result := d.Dispatch(ctx, WidgetActionRequest{
		WidgetInstanceID: widgetInstanceID,
		ActionID:         actionID,
		Arguments:        arguments,
		Actor:            actor,
		Confirmed:        confirmed,
	})
	if result.Issue != nil {
		return map[string]any{"status": "error", "issue": result.Issue}, nil
	}
	return result.Body, nil
}

func findWidget(layout WidgetLayout, instanceID string) *WidgetInstance {
	for i := range layout.Widgets {
		if layout.Widgets[i].ID == instanceID && layout.Widgets[i].Visible {
			return &layout.Widgets[i]
		}
	}
	return nil
}

func findWidgetAction(provider widgets.Widget, actionID string) (widgetskills.Action, bool) {
	skill := provider.Skill()
	if skill == nil {
		return widgetskills.Action{}, false
	}
	for _, action := range skill.Actions {
		if action.ID == actionID {
			return action, true
		}
	}
	return widgetskills.Action{}, false
}

func issueResult(
	status int,
	code string,
	title string,
	message string,
	severity string,
) WidgetActionResult {
	return WidgetActionResult{
		HTTPStatus: status,
		Issue: &widgets.UserFacingIssue{
			Code:     code,
			Severity: severity,
			Title:    title,
			Message:  message,
		},
	}
}

func cloneSettings(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneConnectionRefs(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
