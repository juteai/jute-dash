package main

import (
	"bytes"
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestKronkA2AServerPublishesA2A10Card(t *testing.T) {
	server := newTestA2AServer(t)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()

	server.handleAgentCard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var card struct {
		SupportedInterfaces []struct {
			URL             string `json:"url"`
			ProtocolBinding string `json:"protocolBinding"`
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"supportedInterfaces"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&card); err != nil {
		t.Fatalf("decode card: %v", err)
	}
	if len(card.SupportedInterfaces) != 1 {
		t.Fatalf("supportedInterfaces len = %d, want 1", len(card.SupportedInterfaces))
	}
	if got := card.SupportedInterfaces[0].ProtocolVersion; got != "1.0" {
		t.Fatalf("protocolVersion = %q, want 1.0", got)
	}
	if got := card.SupportedInterfaces[0].ProtocolBinding; got != "JSONRPC" {
		t.Fatalf("protocolBinding = %q, want JSONRPC", got)
	}
}

func TestKronkA2AServerSendMessage(t *testing.T) {
	server := newTestA2AServer(t)
	resp := invokeA2A(t, server, "SendMessage", `{"message":{"messageId":"msg-1","contextId":"ctx-1","role":"ROLE_USER","parts":[{"text":"hello"}]}}`)
	if got := textFromA2AResult(t, resp.Result); got != "fake Kronk reply" {
		t.Fatalf("result text = %q, want fake Kronk reply", got)
	}
}

func TestKronkA2AServerListsAndGetsTasks(t *testing.T) {
	server := newTestA2AServer(t)
	sendResp := invokeA2A(t, server, "SendMessage", `{"message":{"messageId":"msg-1","contextId":"ctx-1","role":"ROLE_USER","parts":[{"text":"hello"}]}}`)
	taskID := taskIDFromA2AResult(t, sendResp.Result)
	if taskID == "" {
		t.Fatalf("send result did not include a task id: %s", string(sendResp.Result))
	}

	listResp := invokeA2A(t, server, "ListTasks", `{"contextId":"ctx-1"}`)
	var listResult struct {
		Tasks []struct {
			ID        string `json:"id"`
			ContextID string `json:"contextId"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(listResp.Result, &listResult); err != nil {
		t.Fatalf("decode list result: %v", err)
	}
	if len(listResult.Tasks) != 1 {
		t.Fatalf("listed tasks = %d, want 1: %s", len(listResult.Tasks), string(listResp.Result))
	}
	if got := listResult.Tasks[0].ID; got != taskID {
		t.Fatalf("listed task id = %q, want %q", got, taskID)
	}

	getResp := invokeA2A(t, server, "GetTask", `{"id":"`+taskID+`"}`)
	var task struct {
		ID        string `json:"id"`
		ContextID string `json:"contextId"`
		Status    struct {
			State string `json:"state"`
		} `json:"status"`
	}
	if err := json.Unmarshal(getResp.Result, &task); err != nil {
		t.Fatalf("decode get result: %v", err)
	}
	if task.ID != taskID {
		t.Fatalf("got task id = %q, want %q", task.ID, taskID)
	}
	if task.ContextID != "ctx-1" {
		t.Fatalf("got task context = %q, want ctx-1", task.ContextID)
	}
	if task.Status.State != "TASK_STATE_COMPLETED" {
		t.Fatalf("got task state = %q, want TASK_STATE_COMPLETED", task.Status.State)
	}
}

func invokeA2A(t *testing.T, server *kronkA2AServer, method, params string) rpcTestResponse {
	t.Helper()
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":"1","method":"` + method + `","params":` + params + `}`)
	req := httptest.NewRequest(http.MethodPost, "/invoke", body)
	req.Header.Set("A2A-Version", "1.0")
	rec := httptest.NewRecorder()

	server.handleInvoke(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", method, rec.Code, http.StatusOK)
	}
	var resp rpcTestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode %s response: %v", method, err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected %s RPC error: %+v", method, resp.Error)
	}
	return resp
}

type rpcTestResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Result json.RawMessage `json:"result"`
}

func textFromA2AResult(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result struct {
		Message *textMessage `json:"message"`
		Task    *struct {
			Status struct {
				Message *textMessage `json:"message"`
			} `json:"status"`
			Artifacts []struct {
				Parts []textPart `json:"parts"`
			} `json:"artifacts"`
			History []textMessage `json:"history"`
		} `json:"task"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Message != nil {
		return textFromTestParts(result.Message.Parts)
	}
	if result.Task != nil {
		if result.Task.Status.Message != nil {
			if text := textFromTestParts(result.Task.Status.Message.Parts); text != "" {
				return text
			}
		}
		for i := len(result.Task.Artifacts) - 1; i >= 0; i-- {
			if text := textFromTestParts(result.Task.Artifacts[i].Parts); text != "" {
				return text
			}
		}
		for i := len(result.Task.History) - 1; i >= 0; i-- {
			if text := textFromTestParts(result.Task.History[i].Parts); text != "" {
				return text
			}
		}
	}
	return ""
}

func taskIDFromA2AResult(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result struct {
		Message *textMessage `json:"message"`
		Task    *struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Message != nil {
		return result.Message.TaskID
	}
	if result.Task != nil {
		return result.Task.ID
	}
	return ""
}

type textMessage struct {
	TaskID string     `json:"taskId"`
	Parts  []textPart `json:"parts"`
}

type textPart struct {
	Text string `json:"text"`
}

func textFromTestParts(parts []textPart) string {
	text := ""
	for _, part := range parts {
		if part.Text == "" {
			continue
		}
		text += part.Text
	}
	return text
}

func newTestA2AServer(t *testing.T) *kronkA2AServer {
	t.Helper()
	a, err := agent.New(agent.Config{
		Name:        "fake_kronk",
		Description: "Fake Kronk agent",
		Run: func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				yield(&session.Event{
					LLMResponse: model.LLMResponse{
						Content: genai.NewContentFromText("fake Kronk reply", genai.RoleModel),
					},
				}, nil)
			}
		},
	})
	if err != nil {
		t.Fatalf("create fake agent: %v", err)
	}
	server, err := newKronkA2AServer(a, "http://127.0.0.1:9797")
	if err != nil {
		t.Fatalf("new A2A server: %v", err)
	}
	return server
}
