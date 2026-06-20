package app

import (
	"context"
	"os"
	"strings"

	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/widgets"
)

type adapterConnectionStore interface {
	AdapterConnection(ctx context.Context, id string) (model.AdapterConnection, error)
}

type secretResolver interface {
	Resolve(ctx context.Context, id string) (string, error)
}

type connectionResolver struct {
	store   adapterConnectionStore
	secrets secretResolver
}

func newConnectionResolver(
	store adapterConnectionStore,
	resolvers ...secretResolver,
) widgets.ConnectionResolver {
	var resolver secretResolver
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	return &connectionResolver{store: store, secrets: resolver}
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
		requiredSecrets := requiredConnectionSecretFields(req)
		resolved := widgets.ResolvedConnection{
			ID:       connection.ID,
			Kind:     connection.Kind,
			Name:     connection.Name,
			Settings: cloneAnyMap(connection.Settings),
			Secrets:  map[string]string{},
			Enabled:  connection.Enabled,
		}
		for key, ref := range connection.SecretRefs {
			value := r.resolveSecretRef(ctx, ref)
			if value == "" {
				if !requiredSecrets[key] {
					continue
				}
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

func requiredConnectionSecretFields(req widgets.ConnectionRequirement) map[string]bool {
	required := map[string]bool{}
	for _, field := range req.Fields {
		if field.Secret && field.Required {
			required[field.ID] = true
		}
	}
	return required
}

func validateRequiredConnectionFields(
	req widgets.ConnectionRequirement,
	connection model.AdapterConnection,
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

func (r *connectionResolver) resolveSecretRef(ctx context.Context, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "env:") {
		return os.Getenv(strings.TrimPrefix(ref, "env:"))
	}
	if strings.HasPrefix(ref, "db:") {
		if r == nil || r.secrets == nil {
			return ""
		}
		value, err := r.secrets.Resolve(ctx, strings.TrimPrefix(ref, "db:"))
		if err != nil {
			return ""
		}
		return value
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
