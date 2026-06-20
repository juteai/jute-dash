package service

import (
	"context"
	"testing"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
)

func TestRunnerRunSendsTrimmedTextAndEmitsTurnEvents(t *testing.T) {
	client := a2aclient.NewInMemoryClient()
	client.StubSendMessage(a2aclient.SendMessageResult{
		ConversationID: "ctx-1",
		TaskID:         "task-1",
		Status:         "completed",
		Text:           "hello back",
	}, nil)
	client.StubGetTask(a2aclient.TaskRecord{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status:    "completed",
		UpdatedAt: "2026-06-20T12:00:00Z",
		Messages: []a2aclient.TaskMessage{
			{Role: "user", Text: "hello"},
			{Role: "assistant", Text: "hello back"},
		},
	}, nil)

	runner := NewRunner(RunnerOptions{
		GetRegistry: func() registry.Registry {
			return registry.New([]registry.AgentConfig{{
				ID:              "agent-1",
				Name:            "Agent One",
				EndpointURL:     "http://agent.example/a2a",
				ProtocolBinding: a2aclient.ProtocolJSONRPC,
				Enabled:         true,
			}})
		},
		GetAgentConfig: func(id string) (AgentConfig, bool) {
			return AgentConfig{ID: id, Name: "Agent One"}, true
		},
		GetAgentCardCache: func(_ context.Context, _ registry.Agent) (AgentCardCache, bool) {
			return AgentCardCache{}, false
		},
		GetDashboardContext: func(_ context.Context) map[string]any { return nil },
		Messages:            client,
	})
	events := []Event{}

	detail, err := runner.Run(t.Context(), "ctx-1", ConversationTurnRequest{
		AgentID: "agent-1",
		Text:    "  hello  ",
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(client.SentMessages) != 1 || client.SentMessages[0].Text != "hello" {
		t.Fatalf("unexpected sent messages: %+v", client.SentMessages)
	}
	if detail.Conversation.LatestTaskID != "task-1" || detail.Messages[1].Content != "hello back" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if len(events) != 2 || events[0].Kind != EventTurnStarted || events[1].Kind != EventTurnCompleted {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestConversationFromTaskUsesFirstUserMessageAsTitle(t *testing.T) {
	task := a2aclient.TaskRecord{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status:    "completed",
		Text:      "fallback title",
		UpdatedAt: "2026-06-20T12:00:00Z",
		Messages: []a2aclient.TaskMessage{
			{Role: "assistant", Text: "ignored"},
			{Role: "user", Text: "  user title  "},
		},
	}

	conversation := ConversationFromTask(registry.Agent{ID: "agent-1", Name: "Agent"}, task)

	if conversation.ID != "ctx-1" || conversation.Title != "user title" {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}
}
