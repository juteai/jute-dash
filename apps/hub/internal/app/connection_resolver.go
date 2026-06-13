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
) (map[string]widgets.ResolvedConnection, map[string]widgets.RuntimePayload) {
	connections := map[string]widgets.ResolvedConnection{}
	issues := map[string]widgets.RuntimePayload{}
	for _, req := range requirements {
		ref := strings.TrimSpace(refs[req.Slot])
		if ref == "" {
			if req.Required {
				issues[req.Slot] = widgets.Unavailable(
					"connection.missing",
					"Connection needed",
					req.DisplayName+" is not connected yet.",
				)
			}
			continue
		}
		if r.store == nil {
			issues[req.Slot] = widgets.Unavailable(
				"connection.store_unavailable",
				"Connection unavailable",
				"Jute cannot read adapter connections right now.",
			)
			continue
		}
		connection, err := r.store.AdapterConnection(ctx, ref)
		if err != nil {
			issues[req.Slot] = widgets.Unavailable(
				"connection.not_found",
				"Connection not found",
				req.DisplayName+" is no longer available.",
			)
			continue
		}
		if connection.Kind != req.Kind {
			issues[req.Slot] = widgets.Unavailable(
				"connection.kind_mismatch",
				"Wrong connection type",
				req.DisplayName+" uses a connection with the wrong type.",
			)
			continue
		}
		if !connection.Enabled {
			issues[req.Slot] = widgets.Unavailable(
				"connection.disabled",
				"Connection disabled",
				req.DisplayName+" is disabled in settings.",
			)
			continue
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
				issues[req.Slot] = widgets.Unavailable(
					"connection.missing_credentials",
					"Credentials unavailable",
					req.DisplayName+" has missing credentials.",
				)
				break
			}
			resolved.Secrets[key] = value
		}
		if _, hasIssue := issues[req.Slot]; hasIssue {
			continue
		}
		connections[req.Slot] = resolved
	}
	return connections, issues
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
