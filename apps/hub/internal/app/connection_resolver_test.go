package app

import (
	"context"
	"errors"
	"testing"

	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/widgets"
)

type fakeAdapterConnectionStore map[string]homestate.AdapterConnection

func (s fakeAdapterConnectionStore) AdapterConnection(
	_ context.Context,
	id string,
) (homestate.AdapterConnection, error) {
	connection, ok := s[id]
	if !ok {
		return homestate.AdapterConnection{}, errors.New("not found")
	}
	return connection, nil
}

type fakeSecretResolver map[string]string

func (s fakeSecretResolver) Resolve(_ context.Context, id string) (string, error) {
	value, ok := s[id]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func TestConnectionResolverReturnsFirstSafeIssue(t *testing.T) {
	requirements := []widgets.ConnectionRequirement{
		{
			Slot:        "first",
			Kind:        "philips-hue",
			DisplayName: "Hue Bridge",
			Required:    true,
			Fields: []widgets.ConnectionField{
				{ID: "bridge_ip", Type: widgets.ConnectionFieldString, Label: "Bridge IP", Required: true},
			},
		},
		{
			Slot:        "second",
			Kind:        "spotify",
			DisplayName: "Spotify Account",
			Required:    true,
		},
	}
	resolver := newConnectionResolver(fakeAdapterConnectionStore{})

	resolution := resolver.ResolveWidgetConnections(context.Background(), requirements, map[string]string{
		"second": "available",
	})

	if resolution.Issue == nil {
		t.Fatal("expected issue")
	}
	if got := resolution.Issue.Issue.Code; got != "connection.missing" {
		t.Fatalf("expected first requirement issue, got %q", got)
	}
}

func TestConnectionResolverResolvesDBSecretRefs(t *testing.T) {
	requirement := widgets.ConnectionRequirement{
		Slot:        "account",
		Kind:        "spotify",
		DisplayName: "Spotify Account",
		Required:    true,
		Fields: []widgets.ConnectionField{
			{ID: "client_id", Type: widgets.ConnectionFieldString, Label: "Client ID", Required: true},
			{
				ID:       "access_token",
				Type:     widgets.ConnectionFieldString,
				Label:    "Access token",
				Required: true,
				Secret:   true,
			},
		},
	}
	resolver := newConnectionResolver(
		fakeAdapterConnectionStore{
			"spotify-main": {
				ID:         "spotify-main",
				Kind:       "spotify",
				Enabled:    true,
				Settings:   map[string]any{"client_id": "client"},
				SecretRefs: map[string]string{"access_token": "db:spotify/main/access_token"},
			},
		},
		fakeSecretResolver{"spotify/main/access_token": "resolved-access-token"},
	)

	resolution := resolver.ResolveWidgetConnections(
		context.Background(),
		[]widgets.ConnectionRequirement{requirement},
		map[string]string{"account": "spotify-main"},
	)

	if resolution.Issue != nil {
		t.Fatalf("unexpected issue: %#v", resolution.Issue.Issue)
	}
	if got := resolution.Connections["account"].Secrets["access_token"]; got != "resolved-access-token" {
		t.Fatalf("expected db secret to resolve, got %q", got)
	}
}

func TestConnectionResolverValidatesConnectionRecords(t *testing.T) {
	requirement := widgets.ConnectionRequirement{
		Slot:        "bridge",
		Kind:        "philips-hue",
		DisplayName: "Hue Bridge",
		Required:    true,
		Fields: []widgets.ConnectionField{
			{ID: "bridge_ip", Type: widgets.ConnectionFieldString, Label: "Bridge IP", Required: true},
			{
				ID:       "username",
				Type:     widgets.ConnectionFieldString,
				Label:    "Username",
				Required: true,
				Secret:   true,
			},
			{
				ID:     "diagnostic_token",
				Type:   widgets.ConnectionFieldString,
				Label:  "Diagnostic token",
				Secret: true,
			},
		},
	}

	tests := []struct {
		name       string
		store      fakeAdapterConnectionStore
		wantIssue  string
		wantSecret string
	}{
		{
			name:      "missing ref",
			store:     fakeAdapterConnectionStore{},
			wantIssue: "connection.not_found",
		},
		{
			name: "wrong kind",
			store: fakeAdapterConnectionStore{
				"hue": {ID: "hue", Kind: "spotify", Enabled: true},
			},
			wantIssue: "connection.kind_mismatch",
		},
		{
			name: "disabled",
			store: fakeAdapterConnectionStore{
				"hue": {ID: "hue", Kind: "philips-hue"},
			},
			wantIssue: "connection.disabled",
		},
		{
			name: "missing required setting",
			store: fakeAdapterConnectionStore{
				"hue": {
					ID:         "hue",
					Kind:       "philips-hue",
					Enabled:    true,
					SecretRefs: map[string]string{"username": "env:HUE_USER"},
				},
			},
			wantIssue: "connection.missing_settings",
		},
		{
			name: "missing required secret ref",
			store: fakeAdapterConnectionStore{
				"hue": {
					ID:       "hue",
					Kind:     "philips-hue",
					Enabled:  true,
					Settings: map[string]any{"bridge_ip": "192.0.2.10"},
				},
			},
			wantIssue: "connection.missing_credentials",
		},
		{
			name: "optional secret omitted",
			store: fakeAdapterConnectionStore{
				"hue": {
					ID:         "hue",
					Kind:       "philips-hue",
					Enabled:    true,
					Settings:   map[string]any{"bridge_ip": "192.0.2.10"},
					SecretRefs: map[string]string{"username": "env:HUE_USER"},
				},
			},
			wantSecret: "resolved-user",
		},
		{
			name: "optional secret ref missing",
			store: fakeAdapterConnectionStore{
				"hue": {
					ID:       "hue",
					Kind:     "philips-hue",
					Enabled:  true,
					Settings: map[string]any{"bridge_ip": "192.0.2.10"},
					SecretRefs: map[string]string{
						"username":         "env:HUE_USER",
						"diagnostic_token": "env:MISSING_OPTIONAL_TOKEN",
					},
				},
			},
			wantSecret: "resolved-user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HUE_USER", "resolved-user")
			resolver := newConnectionResolver(tt.store)
			resolution := resolver.ResolveWidgetConnections(
				context.Background(),
				[]widgets.ConnectionRequirement{requirement},
				map[string]string{"bridge": "hue"},
			)
			if tt.wantIssue != "" {
				if resolution.Issue == nil {
					t.Fatalf("expected issue %q", tt.wantIssue)
				}
				if got := resolution.Issue.Issue.Code; got != tt.wantIssue {
					t.Fatalf("expected issue %q, got %q", tt.wantIssue, got)
				}
				return
			}
			if resolution.Issue != nil {
				t.Fatalf("unexpected issue: %#v", resolution.Issue.Issue)
			}
			resolved := resolution.Connections["bridge"]
			if got := resolved.Secrets["username"]; got != tt.wantSecret {
				t.Fatalf("expected resolved secret %q, got %q", tt.wantSecret, got)
			}
			if _, exists := resolved.Secrets["diagnostic_token"]; exists {
				t.Fatal("optional omitted secret should not be resolved or exposed")
			}
		})
	}
}
