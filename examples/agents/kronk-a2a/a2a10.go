package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

const dashboardContextExtensionURI = "https://jute.dev/a2a/extensions/dashboard-context/v1"

type kronkA2AServer struct {
	agent   agent.Agent
	runner  *runner.Runner
	baseURL string
	history taskStore
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type sendParams struct {
	Message a2aMessage `json:"message"`
}

type a2aMessage struct {
	MessageID string         `json:"messageId,omitempty"`
	ContextID string         `json:"contextId,omitempty"`
	TaskID    string         `json:"taskId,omitempty"`
	Role      string         `json:"role,omitempty"`
	Parts     []a2aPart      `json:"parts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type a2aPart struct {
	Text string `json:"text,omitempty"`
}

type a2aTask struct {
	ID        string        `json:"id"`
	ContextID string        `json:"contextId"`
	Status    a2aTaskStatus `json:"status"`
	History   []a2aMessage  `json:"history,omitempty"`
	Artifacts []a2aArtifact `json:"artifacts,omitempty"`
}

type a2aTaskStatus struct {
	State   string      `json:"state"`
	Message *a2aMessage `json:"message,omitempty"`
}

type a2aArtifact struct {
	Parts []a2aPart `json:"parts"`
}

type taskStore struct {
	mu    sync.Mutex
	tasks []a2aTask
	byID  map[string]a2aTask
}

func newKronkA2AServer(a agent.Agent, baseURL string) (*kronkA2AServer, error) {
	r, err := runner.New(runner.Config{
		AppName:           a.Name(),
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create ADK runner: %w", err)
	}
	return &kronkA2AServer{
		agent:   a,
		runner:  r,
		baseURL: strings.TrimRight(baseURL, "/"),
		history: taskStore{byID: map[string]a2aTask{}},
	}, nil
}

func (s *kronkA2AServer) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        s.agent.Name(),
		"description": s.agent.Description(),
		"version":     "1.0.0",
		"supportedInterfaces": []map[string]string{
			{"url": s.baseURL + "/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"},
		},
		"capabilities": map[string]any{
			"streaming": true,
			"extensions": []map[string]any{
				{"uri": dashboardContextExtensionURI, "description": "Receives redacted Jute dashboard context in message metadata."},
			},
		},
		"defaultInputModes":  []string{"text/plain"},
		"defaultOutputModes": []string{"text/plain"},
		"skills": []map[string]any{
			{
				"id":          s.agent.Name(),
				"name":        "Local Kronk chat",
				"description": "Replies with a local Kronk model.",
				"tags":        []string{"chat", "local", "kronk"},
				"inputModes":  []string{"text/plain"},
				"outputModes": []string{"text/plain"},
			},
		},
	})
}

func (s *kronkA2AServer) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if version := r.Header.Get("A2A-Version"); version != "1.0" {
		writeRPCError(w, nil, -32001, "A2A 1.0 is required")
		return
	}
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "invalid JSON")
		return
	}
	switch req.Method {
	case "SendMessage", "SendStreamingMessage":
		s.handleSend(r.Context(), w, req)
	case "ListTasks":
		s.handleListTasks(w, req)
	case "GetTask":
		s.handleGetTask(w, req)
	default:
		writeRPCError(w, req.ID, -32601, "method not found")
	}
}

func (s *kronkA2AServer) handleSend(ctx context.Context, w http.ResponseWriter, req rpcRequest) {
	var params sendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid send params")
		return
	}
	text := textFromParts(params.Message.Parts)
	if strings.TrimSpace(text) == "" {
		writeRPCError(w, req.ID, -32602, "message text is required")
		return
	}
	contextID := strings.TrimSpace(params.Message.ContextID)
	if contextID == "" {
		contextID = "ctx-" + newID()
	}

	answer, err := s.generateAnswer(ctx, contextID, text)
	if err != nil {
		writeRPCError(w, req.ID, -32000, "agent response failed")
		return
	}
	taskID := "task-" + newID()
	record := a2aTask{
		ID:        taskID,
		ContextID: contextID,
		Status:    a2aTaskStatus{State: "completed"},
		History: []a2aMessage{
			{MessageID: firstNonEmpty(params.Message.MessageID, "msg-"+newID()), ContextID: contextID, Role: "ROLE_USER", Parts: []a2aPart{{Text: text}}},
			{MessageID: "msg-" + newID(), ContextID: contextID, TaskID: taskID, Role: "ROLE_AGENT", Parts: []a2aPart{{Text: answer}}},
		},
		Artifacts: []a2aArtifact{{Parts: []a2aPart{{Text: answer}}}},
	}
	s.history.save(record)

	if req.Method == "SendStreamingMessage" {
		writeStream(w, req.ID, record)
		return
	}
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"message": a2aMessage{
				MessageID: "msg-" + newID(),
				ContextID: contextID,
				TaskID:    taskID,
				Role:      "ROLE_AGENT",
				Parts:     []a2aPart{{Text: answer}},
			},
		},
	})
}

func (s *kronkA2AServer) generateAnswer(ctx context.Context, contextID, text string) (string, error) {
	userMessage := genai.NewContentFromText(text, genai.RoleUser)
	finalText := ""
	allText := []string{}
	for event, err := range s.runner.Run(ctx, "jute-user", contextID, userMessage, agent.RunConfig{StreamingMode: agent.StreamingModeNone}) {
		if err != nil {
			return "", err
		}
		if event == nil || event.LLMResponse.Content == nil {
			continue
		}
		eventText := textFromGenAIParts(event.LLMResponse.Content.Parts)
		if strings.TrimSpace(eventText) == "" {
			continue
		}
		allText = append(allText, eventText)
		if event.IsFinalResponse() {
			finalText = eventText
		}
	}
	if strings.TrimSpace(finalText) != "" {
		return finalText, nil
	}
	if len(allText) > 0 {
		return allText[len(allText)-1], nil
	}
	return "Kronk returned an empty response.", nil
}

func (s *kronkA2AServer) handleListTasks(w http.ResponseWriter, req rpcRequest) {
	var params struct {
		ContextID string `json:"contextId"`
		PageSize  int    `json:"pageSize"`
	}
	_ = json.Unmarshal(req.Params, &params)
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tasks": s.history.list(params.ContextID, params.PageSize),
		},
	})
}

func (s *kronkA2AServer) handleGetTask(w http.ResponseWriter, req rpcRequest) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.ID) == "" {
		writeRPCError(w, req.ID, -32602, "task id is required")
		return
	}
	record, ok := s.history.get(params.ID)
	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"task": record}})
}

func writeStream(w http.ResponseWriter, id json.RawMessage, record a2aTask) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeRPCError(w, id, -32000, "streaming unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	writeSSE(w, id, map[string]any{
		"task": map[string]any{
			"id":        record.ID,
			"contextId": record.ContextID,
			"status":    map[string]any{"state": "working"},
		},
	})
	flusher.Flush()
	answer := textFromParts(record.Artifacts[0].Parts)
	for _, chunk := range streamChunks(answer, 8) {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		time.Sleep(90 * time.Millisecond)
		writeSSE(w, id, map[string]any{
			"artifactUpdate": map[string]any{
				"taskId":    record.ID,
				"contextId": record.ContextID,
				"append":    true,
				"artifact": map[string]any{
					"parts": []map[string]string{{"text": chunk}},
				},
			},
		})
		flusher.Flush()
	}
	time.Sleep(150 * time.Millisecond)
	writeSSE(w, id, map[string]any{
		"statusUpdate": map[string]any{
			"taskId":    record.ID,
			"contextId": record.ContextID,
			"final":     true,
			"status":    map[string]any{"state": "completed"},
		},
	})
	flusher.Flush()
}

func streamChunks(value string, maxWords int) []string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return nil
	}
	if maxWords <= 0 {
		maxWords = 8
	}
	chunks := []string{}
	for i := 0; i < len(words); i += maxWords {
		end := i + maxWords
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		if end < len(words) {
			chunk += " "
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}

func writeSSE(w http.ResponseWriter, id json.RawMessage, result any) {
	bytes, err := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
}

func textFromParts(parts []a2aPart) string {
	chunks := []string{}
	for _, item := range parts {
		if text := strings.TrimSpace(item.Text); text != "" {
			chunks = append(chunks, text)
		}
	}
	return strings.Join(chunks, "\n\n")
}

func textFromGenAIParts(parts []*genai.Part) string {
	chunks := []string{}
	for _, item := range parts {
		if item == nil {
			continue
		}
		if text := strings.TrimSpace(item.Text); text != "" {
			chunks = append(chunks, text)
		}
	}
	return strings.Join(chunks, "")
}

func (s *taskStore) save(record a2aTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[record.ID] = record
	s.tasks = append([]a2aTask{record}, s.tasks...)
}

func (s *taskStore) list(contextID string, pageSize int) []a2aTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 50
	}
	tasks := make([]a2aTask, 0, pageSize)
	for _, record := range s.tasks {
		if contextID != "" && record.ContextID != contextID {
			continue
		}
		tasks = append(tasks, record)
		if len(tasks) >= pageSize {
			break
		}
	}
	return tasks
}

func (s *taskStore) get(id string) (a2aTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.byID[id]
	return record, ok
}

func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage(`null`)
	}
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
