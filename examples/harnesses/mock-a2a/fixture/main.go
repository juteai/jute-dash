package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const dashboardContextExtensionURI = "https://jute.dev/a2a/extensions/dashboard-context/v1"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type sendParams struct {
	Message message `json:"message"`
}

type message struct {
	MessageID string         `json:"messageId,omitempty"`
	ContextID string         `json:"contextId,omitempty"`
	TaskID    string         `json:"taskId,omitempty"`
	Role      string         `json:"role,omitempty"`
	Parts     []part         `json:"parts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type task struct {
	ID        string     `json:"id"`
	ContextID string     `json:"contextId"`
	Status    taskStatus `json:"status"`
	History   []message  `json:"history,omitempty"`
	Artifacts []artifact `json:"artifacts,omitempty"`
}

type taskStatus struct {
	State   string   `json:"state"`
	Message *message `json:"message,omitempty"`
}

type artifact struct {
	Parts []part `json:"parts"`
}

type taskStore struct {
	mu    sync.Mutex
	tasks []task
	byID  map[string]task
}

var history = taskStore{byID: map[string]task{}}

type part struct {
	Text string `json:"text,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func main() {
	listen := strings.TrimSpace(os.Getenv("MOCK_A2A_LISTEN"))
	if listen == "" {
		listen = "127.0.0.1:9797"
	}
	baseURL := "http://" + listen

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name":        "Jute Mock A2A Agent",
			"description": "A deterministic local A2A 1.0 fixture for testing Jute Dash chat and dashboard context.",
			"version":     "1.0.0",
			"supportedInterfaces": []map[string]string{
				{"url": baseURL + "/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"},
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
				{"id": "chat", "name": "Local chat", "description": "Replies to short dashboard test prompts.", "tags": []string{"chat", "dev"}},
				{"id": "dashboard-context", "name": "Dashboard context", "description": "Reports whether Jute dashboard context was supplied.", "tags": []string{"jute", "context"}},
			},
		})
	})
	mux.HandleFunc("/invoke", handleInvoke)

	server := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Mock A2A agent card: %s/.well-known/agent-card.json", baseURL)
	log.Printf("Mock A2A JSON-RPC endpoint: %s/invoke", baseURL)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
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
		handleSend(w, r, req)
	case "ListTasks":
		handleListTasks(w, req)
	case "GetTask":
		handleGetTask(w, req)
	default:
		writeRPCError(w, req.ID, -32601, "method not found")
	}
}

func handleSend(w http.ResponseWriter, r *http.Request, req rpcRequest) {
	var params sendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid send params")
		return
	}
	text := textFromParts(params.Message.Parts)
	if text == "" {
		text = "empty prompt"
	}
	contextID := strings.TrimSpace(params.Message.ContextID)
	if contextID == "" {
		contextID = "ctx-" + newID()
	}
	hasExtensionHeader := strings.Contains(r.Header.Get("A2A-Extensions"), dashboardContextExtensionURI)
	_, hasMetadata := params.Message.Metadata[dashboardContextExtensionURI]
	contextStatus := "no dashboard context received"
	if hasExtensionHeader && hasMetadata {
		contextStatus = "dashboard context received"
	}
	mcpStatus := mcpContextForTurn(r.Context()).Sentence()
	answer := fmt.Sprintf("Mock A2A reply: %s. I saw %s. MCP: %s.", text, contextStatus, mcpStatus)
	taskID := "task-" + newID()
	record := task{
		ID:        taskID,
		ContextID: contextID,
		Status: taskStatus{
			State: "completed",
		},
		History: []message{
			{MessageID: firstNonEmpty(params.Message.MessageID, "msg-"+newID()), ContextID: contextID, Role: "ROLE_USER", Parts: []part{{Text: text}}},
			{MessageID: "msg-" + newID(), ContextID: contextID, TaskID: taskID, Role: "ROLE_AGENT", Parts: []part{{Text: answer}}},
		},
		Artifacts: []artifact{{Parts: []part{{Text: answer}}}},
	}
	history.save(record)
	if req.Method == "SendStreamingMessage" {
		writeStream(w, req.ID, record)
		return
	}
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"message": message{
				MessageID: "msg-" + newID(),
				ContextID: contextID,
				TaskID:    taskID,
				Role:      "ROLE_AGENT",
				Parts:     []part{{Text: answer}},
			},
		},
	})
}

type JuteContext struct {
	Available      bool
	Unavailable    string
	DashboardRead  bool
	SkillCount     int
	SkillIDs       []string
	Weather        WeatherContext
	WeatherRefresh string
	Prompt         string
}

type WeatherContext struct {
	LocationName string
	Condition    string
	Temperature  string
	Status       string
}

type SkillList struct {
	Skills []SkillResult `json:"skills"`
}

type SkillResult struct {
	SkillID     string         `json:"skillId"`
	DisplayName string         `json:"displayName"`
	Actions     []string       `json:"actions"`
	Context     map[string]any `json:"context"`
}

func mcpContextForTurn(ctx context.Context) JuteContext {
	mcpURL := strings.TrimSpace(os.Getenv("JUTE_MCP_URL"))
	if mcpURL == "" {
		return JuteContext{Unavailable: "MCP not configured"}
	}
	token := strings.TrimSpace(os.Getenv("JUTE_MCP_TOKEN"))
	agentID := strings.TrimSpace(os.Getenv("JUTE_MCP_AGENT_ID"))

	var initResult struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	err := mcpCall(ctx, mcpURL, token, agentID, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"clientInfo": map[string]any{
			"name":    "jute-dev-agent",
			"version": "dev",
		},
	}, &initResult)
	if err != nil {
		return JuteContext{Unavailable: "MCP initialize failed"}
	}

	var dashboardResult struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	dashboardRead := false
	if err := mcpCall(ctx, mcpURL, token, agentID, "resources/read", map[string]any{"uri": "jute://dashboard/current"}, &dashboardResult); err == nil {
		dashboardRead = true
	}

	var skillsResult struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := mcpCall(ctx, mcpURL, token, agentID, "resources/read", map[string]any{"uri": "jute://skills"}, &skillsResult); err != nil {
		return JuteContext{Unavailable: "MCP skills unavailable"}
	}
	var text string
	for _, content := range skillsResult.Contents {
		if strings.TrimSpace(content.Text) != "" {
			text = content.Text
			break
		}
	}
	if text == "" {
		return JuteContext{Unavailable: "MCP skills unavailable"}
	}
	var skills SkillList
	if err := json.Unmarshal([]byte(text), &skills); err != nil {
		return JuteContext{Unavailable: "MCP skills could not be decoded"}
	}

	summary := JuteContext{
		Available:     true,
		DashboardRead: dashboardRead,
		SkillCount:    len(skills.Skills),
	}
	for _, skill := range skills.Skills {
		summary.SkillIDs = append(summary.SkillIDs, skill.SkillID)
		if skill.SkillID == "jute.weather.current" {
			summary.Weather = weatherFromContext(skill.Context)
			if contains(skill.Actions, "refresh") {
				var toolResult struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
					IsError bool `json:"isError"`
				}
				err := mcpCall(ctx, mcpURL, token, agentID, "tools/call", map[string]any{
					"name": "jute_skill_invoke_action",
					"arguments": map[string]any{
						"skillId":  "jute.weather.current",
						"actionId": "refresh",
					},
				}, &toolResult)
				if err == nil && !toolResult.IsError {
					summary.WeatherRefresh = "completed"
				} else {
					summary.WeatherRefresh = "unavailable"
				}
			}
		}
	}

	var promptResult struct {
		Description string `json:"description"`
		Messages    []struct {
			Role    string `json:"role"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := mcpCall(ctx, mcpURL, token, agentID, "prompts/get", map[string]any{
		"name":      "jute_home_assistant_guidance",
		"arguments": map[string]any{},
	}, &promptResult); err == nil && len(promptResult.Messages) > 0 {
		summary.Prompt = truncate(promptResult.Messages[0].Content.Text, 160)
	}

	return summary
}

func mcpCall(ctx context.Context, mcpURL, token, agentID, method string, params any, result any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("%d", time.Now().UnixNano()),
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if agentID != "" {
		req.Header.Set("X-Jute-Agent-ID", agentID)
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error code %d", rpcResp.Error.Code)
	}
	if result != nil && len(rpcResp.Result) > 0 {
		return json.Unmarshal(rpcResp.Result, result)
	}
	return nil
}

func (s JuteContext) Sentence() string {
	if !s.Available {
		if s.Unavailable == "" {
			return "MCP not configured"
		}
		return s.Unavailable
	}
	parts := []string{fmt.Sprintf("MCP saw %d widget skills", s.SkillCount)}
	if s.DashboardRead {
		parts = append(parts, "dashboard context read")
	}
	if len(s.SkillIDs) > 0 {
		parts = append(parts, "skills: "+strings.Join(s.SkillIDs, ", "))
	}
	if s.Weather.LocationName != "" || s.Weather.Condition != "" {
		weather := strings.TrimSpace(strings.Join(nonEmpty([]string{s.Weather.LocationName, s.Weather.Condition, s.Weather.Temperature, s.Weather.Status}), " "))
		if weather != "" {
			parts = append(parts, "weather: "+weather)
		}
	}
	if s.WeatherRefresh != "" {
		parts = append(parts, "weather refresh: "+s.WeatherRefresh)
	}
	return strings.Join(parts, "; ")
}

func weatherFromContext(context map[string]any) WeatherContext {
	return WeatherContext{
		LocationName: stringValue(context["locationName"]),
		Condition:    stringValue(context["condition"]),
		Temperature:  temperatureValue(context["temperature"], stringValue(context["temperatureUnit"])),
		Status:       stringValue(context["status"]),
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func temperatureValue(value any, unit string) string {
	if value == nil {
		return ""
	}
	var number string
	switch typed := value.(type) {
	case float64:
		number = strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		number = strconv.Itoa(typed)
	default:
		number = fmt.Sprint(typed)
	}
	return strings.TrimSpace(number + " " + unit)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func nonEmpty(values []string) []string {
	out := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit]) + "..."
}


func writeStream(w http.ResponseWriter, id json.RawMessage, record task) {
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

func handleListTasks(w http.ResponseWriter, req rpcRequest) {
	var params struct {
		ContextID string `json:"contextId"`
		PageSize  int    `json:"pageSize"`
	}
	_ = json.Unmarshal(req.Params, &params)
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tasks": history.list(params.ContextID, params.PageSize),
		},
	})
}

func handleGetTask(w http.ResponseWriter, req rpcRequest) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.ID) == "" {
		writeRPCError(w, req.ID, -32602, "task id is required")
		return
	}
	record, ok := history.get(params.ID)
	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"task": record}})
}

func writeSSE(w http.ResponseWriter, id json.RawMessage, result any) {
	bytes, err := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
}

func textFromParts(parts []part) string {
	chunks := []string{}
	for _, item := range parts {
		if text := strings.TrimSpace(item.Text); text != "" {
			chunks = append(chunks, text)
		}
	}
	return strings.Join(chunks, "\n\n")
}

func (s *taskStore) save(record task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[record.ID] = record
	s.tasks = append([]task{record}, s.tasks...)
}

func (s *taskStore) list(contextID string, pageSize int) []task {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 50
	}
	tasks := make([]task, 0, pageSize)
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

func (s *taskStore) get(id string) (task, bool) {
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
