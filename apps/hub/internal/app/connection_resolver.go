package app

import (
	"context"
	"os"
	"strings"

	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/widgets"
)

type adapterConnectionStore interface {
	AdapterConnection(ctx context.Context, id string) (homestate.AdapterConnection, error)
}

type connectionResolver struct {
	store adapterConnectionStore
}

func newConnectionResolver(store adapterConnectionStore) widgets.ConnectionResolver {
	return &connectionResolver{store: store}
}

func (r *connectionResolver) ResolveWidgetConnections(
	ctx context.Context,
	requirements []widgets.ConnectionRequirement,
	refs map[string]string,
) widgets.ConnectionResolution {
	connections := map[string]widgets.ResolvedConnection{}
	for _, req := range requirements {
		ref := strings.TrimSpace(refs[req.Slot])
		if ref == "" {
			if req.Required {
				return widgets.ConnectionResolution{Connections: connections, Issue: issuePtr(widgets.Unavailable(
					"connection.missing",
					"Connection needed",
					req.DisplayName+" is not connected yet.",
				))}
			}
			continue
		}
		if r.store == nil {
			return widgets.ConnectionResolution{Connections: connections, Issue: issuePtr(widgets.Unavailable(
				"connection.store_unavailable",
				"Connection unavailable",
				"Jute cannot read adapter connections right now.",
			))}
		}
		connection, err := r.store.AdapterConnection(ctx, ref)
		if err != nil {
			return widgets.ConnectionResolution{Connections: connections, Issue: issuePtr(widgets.Unavailable(
				"connection.not_found",
				"Connection not found",
				req.DisplayName+" is no longer available.",
			))}
		}
		if connection.Kind != req.Kind {
			return widgets.ConnectionResolution{Connections: connections, Issue: issuePtr(widgets.Unavailable(
				"connection.kind_mismatch",
				"Wrong connection type",
				req.DisplayName+" uses a connection with the wrong type.",
			))}
		}
		if !connection.Enabled {
			return widgets.ConnectionResolution{Connections: connections, Issue: issuePtr(widgets.Unavailable(
				"connection.disabled",
				"Connection disabled",
				req.DisplayName+" is disabled in settings.",
			))}
		}
		if issue := validateRequiredConnectionFields(req, connection); issue != nil {
			return widgets.ConnectionResolution{Connections: connections, Issue: issue}
		}
		resolved := widgets.ResolvedConnection{
			ID:       connection.ID,
			Kind:     connection.Kind,
			Name:     connection.Name,
			Settings: cloneAnyMap(connection.Settings),
			Secrets:  map[string]string{},
			Enabled:  connection.Enabled,
		}
		for key, ref := range connection.SecretRefs {
			value := resolveSecretRef(ref)
			if value == "" {
				return widgets.ConnectionResolution{Connections: connections, Issue: issuePtr(widgets.Unavailable(
					"connection.missing_credentials",
					"Credentials unavailable",
					req.DisplayName+" has missing credentials.",
				))}
			}
			resolved.Secrets[key] = value
		}
		connections[req.Slot] = resolved
	}
	return widgets.ConnectionResolution{Connections: connections}
}

func validateRequiredConnectionFields(
	req widgets.ConnectionRequirement,
	connection homestate.AdapterConnection,
) *widgets.RuntimePayload {
	for _, field := range req.Fields {
		if !field.Required {
			continue
		}
		if field.Secret {
			if strings.TrimSpace(connection.SecretRefs[field.ID]) == "" {
				payload := widgets.Unavailable(
					"connection.missing_credentials",
					"Credentials unavailable",
					req.DisplayName+" has missing credentials.",
				)
				return &payload
			}
			continue
		}
		if missingSetting(connection.Settings[field.ID]) {
			payload := widgets.Unavailable(
				"connection.missing_settings",
				"Connection incomplete",
				req.DisplayName+" is missing required setup details.",
			)
			return &payload
		}
	}
	return nil
}

func missingSetting(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	default:
		return false
	}
}

func issuePtr(payload widgets.RuntimePayload) *widgets.RuntimePayload {
	return &payload
}

func resolveSecretRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "env:") {
		return os.Getenv(strings.TrimPrefix(ref, "env:"))
	}
	return os.Getenv(ref)
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
