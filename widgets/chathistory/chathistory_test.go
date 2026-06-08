package chathistory

import (
	"context"
	"testing"

	"jute-dash/apps/hub/pkg/widgetskills"
)

func TestChatHistoryWidget_FetchData(t *testing.T) {
	w := &ChatHistoryWidget{}
	data, err := w.FetchData(context.Background(), nil)
	if err != nil {
		t.Fatalf("FetchData error: %v", err)
	}
	if data == nil {
		t.Fatalf("expected non-nil data")
	}
}

func TestChatHistoryWidget_ChatHistoryContext(t *testing.T) {
	snapshot := widgetskills.Snapshot{
		Agents: []widgetskills.Agent{
			{
				ID:              "agent1",
				Name:            "Agent 1",
				Description:     "Test Agent",
				ProtocolBinding: "HTTP+JSON",
				Enabled:         true,
				Capabilities:    []string{"chat"},
				AuthConfigured:  true,
			},
		},
		Config: widgetskills.Config{
			Voice: struct {
				PreferredAgentID string `json:"preferredAgentId"`
			}{
				PreferredAgentID: "agent1",
			},
		},
	}

	ctx := chatHistoryContext(snapshot, "")
	if ctx["agentCount"] != 1 {
		t.Errorf("expected agentCount 1, got %v", ctx["agentCount"])
	}
	if ctx["enabledAgentCount"] != 1 {
		t.Errorf("expected enabledAgentCount 1, got %v", ctx["enabledAgentCount"])
	}
	if ctx["preferredAgentId"] != "agent1" {
		t.Errorf("expected preferredAgentId agent1, got %v", ctx["preferredAgentId"])
	}

	agentsList, ok := ctx["agents"].([]map[string]any)
	if !ok {
		t.Fatalf("expected agents to be []map[string]any, got %T", ctx["agents"])
	}
	if len(agentsList) != 1 {
		t.Fatalf("expected 1 agent in list, got %d", len(agentsList))
	}
	if agentsList[0]["id"] != "agent1" || agentsList[0]["name"] != "Agent 1" {
		t.Errorf("unexpected agent values: %+v", agentsList[0])
	}
}
