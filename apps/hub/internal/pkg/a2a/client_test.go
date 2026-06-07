package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJSONRPCClientSendsA2A10SendMessageRequest(t *testing.T) {
	var got struct {
		Method string `json:"method"`
		Params struct {
			Message struct {
				ContextID string         `json:"contextId"`
				Role      string         `json:"role"`
				Metadata  map[string]any `json:"metadata"`
				Parts     []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"message"`
			Configuration struct {
				ReturnImmediately *bool `json:"returnImmediately"`
			} `json:"configuration"`
		} `json:"params"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer dev-token" {
			t.Fatalf("unexpected auth header %q", auth)
		}
		if version := r.Header.Get("A2a-Version"); version != "1.0" {
			t.Fatalf("unexpected A2A-Version %q", version)
		}
		if extensions := r.Header.Get("A2a-Extensions"); extensions != DashboardContextExtensionURI {
			t.Fatalf("unexpected A2A-Extensions %q", extensions)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRPCResult(
			t,
			w,
			`{"message":{"messageId":"msg-1","contextId":"ctx-1","role":"ROLE_AGENT","parts":[{"text":"Hello from A2A"}]}}`,
		)
	}))
	defer server.Close()

	client := NewJSONRPCClient()
	result, err := client.SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
		BearerToken:     "dev-token",
		ConversationID:  "ctx-existing",
		Extensions:      []string{DashboardContextExtensionURI},
		Metadata: map[string]any{
			DashboardContextExtensionURI: map[string]any{"dashboard": "safe"},
		},
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if got.Method != "message/send" {
		t.Fatalf("method = %q, want message/send", got.Method)
	}
	if got.Params.Configuration.ReturnImmediately == nil || *got.Params.Configuration.ReturnImmediately {
		t.Fatal("expected blocking request with returnImmediately=false")
	}
	if got.Params.Message.ContextID != "ctx-existing" {
		t.Fatalf("unexpected contextId %q", got.Params.Message.ContextID)
	}
	if got.Params.Message.Role != "ROLE_USER" || len(got.Params.Message.Parts) != 1 ||
		got.Params.Message.Parts[0].Text != "Hello" {
		t.Fatalf("unexpected message payload: %+v", got.Params.Message)
	}
	if got.Params.Message.Metadata[DashboardContextExtensionURI] == nil {
		t.Fatalf("expected dashboard metadata, got %+v", got.Params.Message.Metadata)
	}
	if result.ConversationID != "ctx-1" || result.Status != "completed" || result.Text != "Hello from A2A" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientDoesNotRetryLegacyMethodNames(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		method = req.Method
		writeRPCError(t, w, -32601, "not found")
	}))
	defer server.Close()

	_, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	})
	if err == nil {
		t.Fatal("expected method-not-found error")
	}
	if method != "message/send" {
		t.Fatalf("method = %q, want message/send", method)
	}
}

func TestJSONRPCClientExtractsTaskStatusText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"task":{
				"id":"task-1",
				"contextId":"ctx-1",
				"status":{
					"state":"completed",
					"message":{"messageId":"msg-3","role":"ROLE_AGENT","parts":[{"text":"Task complete"}]}
				}
			}
		}`)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Status",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.ConversationID != "ctx-1" || result.TaskID != "task-1" || result.Status != "completed" ||
		result.Text != "Task complete" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientStripsReasoningFromMessageResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(
			t,
			w,
			`{"message":{"messageId":"msg-1","contextId":"ctx-1","role":"ROLE_AGENT","parts":[{"text":"Okay, the user just said \"Hello\". I should respond politely. Since there's no specific request, I'll greet them and offer help. No need to call any functions here because the conversation isn't about the tools provided. Just a simple, friendly reply.\n\nHello! How can I assist you today?"}]}}`,
		)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.Text != "Hello! How can I assist you today?" {
		t.Fatalf("unexpected sanitized text: %q", result.Text)
	}
}

func TestJSONRPCClientStripsTaggedReasoningFromTaskHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"kind":"task",
			"id":"task-think",
			"contextId":"ctx-think",
			"status":{"state":"completed"},
			"history":[
				{"kind":"message","messageId":"msg-1","role":"ROLE_USER","parts":[{"text":"Hello"}]},
				{"kind":"message","messageId":"msg-2","role":"ROLE_AGENT","parts":[{"text":"<think>I should not show this.</think>\n\nHi there."}]}
			]
		}`)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.Text != "Hi there." {
		t.Fatalf("unexpected sanitized text: %q", result.Text)
	}
}

func TestJSONRPCClientExtractsLatestTaskHistoryText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"kind":"task",
			"id":"task-2",
			"contextId":"ctx-2",
			"status":{"state":"completed"},
			"history":[
				{"kind":"message","messageId":"msg-4","role":"ROLE_USER","parts":[{"text":"Question"}]},
				{"kind":"message","messageId":"msg-5","role":"ROLE_AGENT","parts":[{"text":"Latest answer"}]}
			]
		}`)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "History",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.Text != "Latest answer" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientNormalizesA2A10TaskStates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"task":{
				"id":"task-9",
				"contextId":"ctx-9",
				"status":{"state":"TASK_STATE_COMPLETED"},
				"artifacts":[{"parts":[{"text":"Hi there"}]}]
			}
		}`)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Status",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed (normalized from TASK_STATE_COMPLETED)", result.Status)
	}
	if result.Text != "Hi there" {
		t.Fatalf("text = %q, want artifact-derived reply", result.Text)
	}
}

func TestJSONRPCClientGetTaskIncludesArtifactReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{"task":{
			"id":"task-art",
			"contextId":"ctx-art",
			"status":{"state":"TASK_STATE_COMPLETED"},
			"history":[
				{"kind":"message","messageId":"msg-user","role":"ROLE_USER","parts":[{"text":"hello"}]}
			],
			"artifacts":[{"parts":[{"text":"Hello back"}]}]
		}}`)
	}))
	defer server.Close()

	task, err := NewJSONRPCClient().GetTask(t.Context(), GetTaskRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		TaskID:          "task-art",
	})
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.Status != "completed" {
		t.Fatalf("task.Status = %q, want completed", task.Status)
	}
	if len(task.Messages) != 2 {
		t.Fatalf("expected 2 messages (user + agent from artifact), got %+v", task.Messages)
	}
	if task.Messages[0].Role != "user" || task.Messages[0].Text != "hello" {
		t.Fatalf("unexpected user message: %+v", task.Messages[0])
	}
	if task.Messages[1].Role != "assistant" || task.Messages[1].Text != "Hello back" {
		t.Fatalf("unexpected synthesised assistant message: %+v", task.Messages[1])
	}
}

func TestJSONRPCClientGetTaskSanitizesAssistantHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{"task":{
			"id":"task-sanitize",
			"contextId":"ctx-sanitize",
			"status":{"state":"TASK_STATE_COMPLETED"},
			"history":[
				{"kind":"message","messageId":"msg-user","role":"ROLE_USER","parts":[{"text":"hello"}]},
				{"kind":"message","messageId":"msg-agent","role":"ROLE_AGENT","parts":[{"text":"Okay, the user said hello. I should greet them. No need to call tools.\n\nHello back."}]}
			]
		}}`)
	}))
	defer server.Close()

	task, err := NewJSONRPCClient().GetTask(t.Context(), GetTaskRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		TaskID:          "task-sanitize",
	})
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if len(task.Messages) != 2 || task.Messages[1].Text != "Hello back." {
		t.Fatalf("unexpected sanitized history: %+v", task.Messages)
	}
	if task.Text != "Hello back." {
		t.Fatalf("unexpected sanitized task text: %q", task.Text)
	}
}

func TestJSONRPCClientStreamNormalizesA2A10States(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeRPCSSE(
			t,
			w,
			`{"statusUpdate":{"taskId":"task-x","contextId":"ctx-x","status":{"state":"TASK_STATE_WORKING"}}}`,
		)
		writeRPCSSE(
			t,
			w,
			`{"statusUpdate":{"taskId":"task-x","contextId":"ctx-x","status":{"state":"TASK_STATE_COMPLETED"},"final":true}}`,
		)
	}))
	defer server.Close()

	var events []StreamEvent
	err := NewJSONRPCClient().StreamMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMessage() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %+v", events)
	}
	if events[0].Status != "working" || events[0].Terminal {
		t.Fatalf("first event = %+v, want working/non-terminal", events[0])
	}
	if events[1].Status != "completed" || !events[1].Terminal {
		t.Fatalf("second event = %+v, want completed/terminal", events[1])
	}
}

func TestJSONRPCClientStreamSanitizesReasoningDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeRPCSSE(
			t,
			w,
			`{"artifactUpdate":{"taskId":"task-1","contextId":"ctx-1","artifact":{"parts":[{"text":"<think>private tool plan</think>\n\nVisible reply"}]},"append":true,"lastChunk":true}}`,
		)
	}))
	defer server.Close()

	var events []StreamEvent
	err := NewJSONRPCClient().StreamMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMessage() error = %v", err)
	}
	if len(events) != 1 || events[0].Text != "Visible reply" {
		t.Fatalf("unexpected sanitized stream events: %+v", events)
	}
}

func TestJSONRPCClientMapsRPCErrorSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCError(t, w, -32000, "secret backend stack trace")
	}))
	defer server.Close()

	_, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret backend stack trace") {
		t.Fatalf("error leaked remote detail: %v", err)
	}
}

func TestJSONRPCClientListTasksSendsExpectedRequest(t *testing.T) {
	var got struct {
		Method string `json:"method"`
		Params struct {
			ContextID string `json:"contextId"`
			PageSize  int    `json:"pageSize"`
		} `json:"params"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRPCResult(
			t,
			w,
			`{"tasks":[{"id":"task-1","contextId":"ctx-1","status":{"state":"completed"},"history":[{"messageId":"user-1","role":"ROLE_USER","parts":[{"text":"Hello"}]},{"messageId":"agent-1","role":"ROLE_AGENT","parts":[{"text":"Hi"}]}]}]}`,
		)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().ListTasks(t.Context(), ListTasksRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		ContextID:       "ctx-1",
		PageSize:        10,
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if got.Method != "ListTasks" || got.Params.ContextID != "ctx-1" || got.Params.PageSize != 10 {
		t.Fatalf("unexpected request: %+v", got)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].ID != "task-1" || len(result.Tasks[0].Messages) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Tasks[0].Messages[0].Role != "user" || result.Tasks[0].Messages[1].Text != "Hi" {
		t.Fatalf("unexpected task messages: %+v", result.Tasks[0].Messages)
	}
}

func TestJSONRPCClientGetTaskSendsExpectedRequest(t *testing.T) {
	var got struct {
		Method string `json:"method"`
		Params struct {
			ID            string `json:"id"`
			HistoryLength int    `json:"historyLength"`
		} `json:"params"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRPCResult(
			t,
			w,
			`{"task":{"id":"task-2","contextId":"ctx-2","status":{"state":"completed","message":{"role":"ROLE_AGENT","parts":[{"text":"Done"}]}}}}`,
		)
	}))
	defer server.Close()

	task, err := NewJSONRPCClient().GetTask(t.Context(), GetTaskRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		TaskID:          "task-2",
		HistoryLength:   25,
	})
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Method != "tasks/get" || got.Params.ID != "task-2" || got.Params.HistoryLength != 25 {
		t.Fatalf("unexpected request: %+v", got)
	}
	if task.ID != "task-2" || task.ContextID != "ctx-2" || len(task.Messages) != 1 || task.Messages[0].Text != "Done" {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func TestJSONRPCClientStreamsA2AEvents(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		method = req.Method
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("ResponseWriter is not a Flusher")
		}
		writeRPCSSE(t, w, `{"task":{"id":"task-1","contextId":"ctx-1","status":{"state":"working"}}}`)
		flusher.Flush()
		writeRPCSSE(
			t,
			w,
			`{"artifactUpdate":{"taskId":"task-1","contextId":"ctx-1","artifact":{"parts":[{"text":"Hel"}]},"append":true}}`,
		)
		flusher.Flush()
		writeRPCSSE(
			t,
			w,
			`{"artifactUpdate":{"taskId":"task-1","contextId":"ctx-1","artifact":{"parts":[{"text":"lo"}]},"append":true,"lastChunk":true}}`,
		)
		flusher.Flush()
		writeRPCSSE(
			t,
			w,
			`{"statusUpdate":{"taskId":"task-1","contextId":"ctx-1","status":{"state":"completed"},"final":true}}`,
		)
		flusher.Flush()
		// Block the handler. Once the client terminates scanning, it closes the response
		// body, which cancels the request context and lets this handler exit.
		select {
		case <-r.Context().Done():
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	var events []StreamEvent
	err := NewJSONRPCClient().StreamMessage(ctx, SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMessage() error = %v", err)
	}
	if method != "message/stream" {
		t.Fatalf("method = %q, want message/stream", method)
	}
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %+v", events)
	}
	if events[0].Kind != "task" || events[1].Kind != "artifact" || events[2].Kind != "artifact" ||
		events[3].Kind != "status" {
		t.Fatalf("unexpected stream event kinds: %+v", events)
	}
	if events[1].Text != "Hel" || !events[1].Append || events[2].Text != "lo" || events[2].Terminal ||
		!events[3].Terminal {
		t.Fatalf("unexpected stream events: %+v", events)
	}
}

func writeRPCResult(t *testing.T, w http.ResponseWriter, result string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"test","result":` + result + `}`))
}

func writeRPCSSE(t *testing.T, w http.ResponseWriter, result string) {
	t.Helper()
	_, _ = w.Write([]byte(`data: {"jsonrpc":"2.0","id":"test","result":` + result + "}\n\n"))
}

func writeRPCError(t *testing.T, w http.ResponseWriter, code int, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      "test",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
