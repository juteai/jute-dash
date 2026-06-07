package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"
)

type TaskMessage struct {
	ID        string `json:"id,omitempty"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type TaskRecord struct {
	ID        string        `json:"id"`
	ContextID string        `json:"contextId"`
	Status    string        `json:"status"`
	Text      string        `json:"text,omitempty"`
	Messages  []TaskMessage `json:"messages,omitempty"`
	UpdatedAt string        `json:"updatedAt"`
}

type ListTasksRequest struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	BearerToken     string
	ContextID       string
	PageSize        int
	PageToken       string
}

type ListTasksResult struct {
	Tasks         []TaskRecord `json:"tasks"`
	NextPageToken string       `json:"nextPageToken,omitempty"`
}

type GetTaskRequest struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	BearerToken     string
	TaskID          string
	HistoryLength   int
}

type listTasksParams struct {
	ContextID string `json:"contextId,omitempty"`
	PageSize  int    `json:"pageSize,omitempty"`
	PageToken string `json:"pageToken,omitempty"`
}

type getTaskParams struct {
	ID            string `json:"id"`
	HistoryLength int    `json:"historyLength,omitempty"`
}

type listTasksResponse struct {
	Tasks         []task `json:"tasks,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

func (c *JSONRPCClient) ListTasks(ctx context.Context, req ListTasksRequest) (ListTasksResult, error) {
	if req.ProtocolBinding != "" && req.ProtocolBinding != ProtocolJSONRPC {
		return ListTasksResult{}, ErrUnsupportedProtocol
	}
	if strings.TrimSpace(req.EndpointURL) == "" {
		return ListTasksResult{}, errors.New("a2a endpoint url is required")
	}
	params := listTasksParams{
		ContextID: strings.TrimSpace(req.ContextID),
		PageSize:  req.PageSize,
		PageToken: strings.TrimSpace(req.PageToken),
	}
	raw, err := c.call(ctx, req.EndpointURL, req.ProtocolVersion, req.BearerToken, methodListTasks, params)
	if err != nil {
		return ListTasksResult{}, err
	}
	var wrapped listTasksResponse
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Tasks != nil {
		return ListTasksResult{Tasks: taskRecords(wrapped.Tasks), NextPageToken: wrapped.NextPageToken}, nil
	}
	var tasks []task
	if err := json.Unmarshal(raw, &tasks); err == nil {
		return ListTasksResult{Tasks: taskRecords(tasks)}, nil
	}
	return ListTasksResult{}, errors.New("decode a2a list tasks result: unsupported result shape")
}

func (c *JSONRPCClient) GetTask(ctx context.Context, req GetTaskRequest) (TaskRecord, error) {
	if req.ProtocolBinding != "" && req.ProtocolBinding != ProtocolJSONRPC {
		return TaskRecord{}, ErrUnsupportedProtocol
	}
	if strings.TrimSpace(req.EndpointURL) == "" {
		return TaskRecord{}, errors.New("a2a endpoint url is required")
	}
	if strings.TrimSpace(req.TaskID) == "" {
		return TaskRecord{}, errors.New("a2a task id is required")
	}
	raw, err := c.call(ctx, req.EndpointURL, req.ProtocolVersion, req.BearerToken, methodGetTask, getTaskParams{
		ID:            strings.TrimSpace(req.TaskID),
		HistoryLength: req.HistoryLength,
	})
	if err != nil {
		return TaskRecord{}, err
	}
	var wrapped struct {
		Task *task `json:"task,omitempty"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Task != nil {
		return taskRecord(*wrapped.Task), nil
	}
	var t task
	if err := json.Unmarshal(raw, &t); err != nil {
		return TaskRecord{}, fmt.Errorf("decode a2a task result: %w", err)
	}
	return taskRecord(t), nil
}

func (c *JSONRPCClient) call(
	ctx context.Context,
	endpointURL, protocolVersion, bearerToken, method string,
	params any,
) (json.RawMessage, error) {
	payload := jsonRPCRequest{
		JSONRPC: jsonRPCVersion,
		ID:      newID(),
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode a2a request: %w", err)
	}
	httpReq, err := newHTTPRequest(ctx, SendMessageRequest{
		EndpointURL:     endpointURL,
		ProtocolVersion: protocolVersion,
		BearerToken:     bearerToken,
	}, body)
	if err != nil {
		return nil, err
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = NewJSONRPCClient().HTTPClient
	}

	logger := c.Logger
	if logger == nil {
		logger = slog.Default() // fallback default logger
	}

	start := time.Now()
	resp, err := httpClient.Do(httpReq)
	duration := time.Since(start)

	attrs := []any{
		slog.String("url", endpointURL),
		slog.String("method", method),
		slog.Float64("duration_ms", float64(duration.Microseconds())/1000.0),
	}

	switch method {
	case methodListTasks:
		if p, ok := params.(listTasksParams); ok && p.ContextID != "" {
			attrs = append(attrs, slog.String("conversation_id", p.ContextID))
		}
	case methodGetTask:
		if p, ok := params.(getTaskParams); ok && p.ID != "" {
			attrs = append(attrs, slog.String("task_id", p.ID))
		}
	}

	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
		logger.ErrorContext(ctx, "a2a client request failed", attrs...)
		return nil, ErrAgentTransport
	}
	defer resp.Body.Close()

	attrs = append(attrs, slog.Int("status", resp.StatusCode))

	responseBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
		logger.ErrorContext(ctx, "a2a client read response failed", attrs...)
		return nil, fmt.Errorf("read a2a response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logger.ErrorContext(ctx, "a2a client returned non-2xx status", attrs...)
		return nil, fmt.Errorf("%w: status %d", ErrAgentTransport, resp.StatusCode)
	}
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(responseBytes, &rpcResp); err != nil {
		attrs = append(attrs, slog.Any("error", err))
		logger.ErrorContext(ctx, "a2a client decode JSON-RPC failed", attrs...)
		return nil, fmt.Errorf("decode a2a response: %w", err)
	}
	if rpcResp.Error != nil {
		logger.ErrorContext(
			ctx,
			"a2a client returned JSON-RPC error",
			append(attrs, slog.Int("code", rpcResp.Error.Code))...,
		)
		return nil, &RPCError{Code: rpcResp.Error.Code}
	}
	if len(rpcResp.Result) == 0 {
		logger.ErrorContext(ctx, "a2a client returned empty result", attrs...)
		return nil, errors.New("a2a response did not include a result")
	}

	logger.InfoContext(ctx, "a2a client request succeeded", attrs...)
	return rpcResp.Result, nil
}

func taskRecords(tasks []task) []TaskRecord {
	records := make([]TaskRecord, 0, len(tasks))
	for _, item := range tasks {
		records = append(records, taskRecord(item))
	}
	return records
}

func taskRecord(t task) TaskRecord {
	result := resultFromTask(t)
	messages := taskMessages(t)
	return TaskRecord{
		ID:        t.ID,
		ContextID: fallbackID(t.ContextID, t.ID, "local-a2a"),
		Status:    result.Status,
		Text:      result.Text,
		Messages:  messages,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

// taskMessages collects the user-visible message thread for a task. Many
// agents (notably ADK-based ones using OutputArtifactPerRun) put the user
// message in t.History but emit the agent's reply as an artifact, so we
// also synthesise an assistant message from t.Status.Message or
// t.Artifacts whenever history doesn't already contain one.
func taskMessages(t task) []TaskMessage {
	messages := make([]TaskMessage, 0, len(t.History)+1)
	hasAgentMessage := false
	for _, item := range t.History {
		text := displayTextFromMessage(item)
		if text == "" {
			continue
		}
		role := normalizeRole(item.Role)
		if role == "assistant" {
			hasAgentMessage = true
		}
		messages = append(messages, TaskMessage{
			ID:   item.MessageID,
			Role: role,
			Text: text,
		})
	}
	if !hasAgentMessage {
		if text := agentReplyText(t); text != "" {
			messages = append(messages, TaskMessage{Role: "assistant", Text: text})
		}
	}
	return messages
}

// agentReplyText returns the latest agent-authored text from the task's
// status message or artifacts, used as a fallback when the task history
// doesn't contain an explicit assistant turn.
func agentReplyText(t task) string {
	if text := displayTextFromOptionalMessage(t.Status.Message); text != "" {
		return text
	}
	for _, v := range slices.Backward(t.Artifacts) {
		if text := displayTextFromParts(v.Parts); text != "" {
			return text
		}
	}
	return ""
}

func normalizeRole(role string) string {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "ROLE_USER", "USER":
		return "user"
	case "ROLE_AGENT", "AGENT", "ASSISTANT":
		return "assistant"
	default:
		return "assistant"
	}
}

// normalizeTaskState collapses the verbose A2A 1.0 task state names
// (TASK_STATE_COMPLETED, TASK_STATE_WORKING, ...) into the short form
// the hub and display use throughout the rest of the codebase. Short
// values from older agents pass through unchanged.
func normalizeTaskState(state string) string {
	trimmed := strings.TrimSpace(state)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "TASK_STATE_UNSPECIFIED":
		return ""
	case "TASK_STATE_SUBMITTED":
		return "submitted"
	case "TASK_STATE_WORKING":
		return "working"
	case "TASK_STATE_INPUT_REQUIRED":
		return "input-required"
	case "TASK_STATE_COMPLETED":
		return "completed"
	case "TASK_STATE_CANCELED", "TASK_STATE_CANCELLED":
		return "canceled"
	case "TASK_STATE_FAILED":
		return "failed"
	case "TASK_STATE_REJECTED":
		return "rejected"
	case "TASK_STATE_AUTH_REQUIRED":
		return "auth-required"
	}
	if after, ok := strings.CutPrefix(upper, "TASK_STATE_"); ok {
		return strings.ToLower(after)
	}
	return strings.ToLower(trimmed)
}
