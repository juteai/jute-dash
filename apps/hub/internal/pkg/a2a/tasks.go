package a2a

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
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

func (c *JSONRPCClient) ListTasks(ctx context.Context, req ListTasksRequest) (ListTasksResult, error) {
	if req.ProtocolBinding != "" && req.ProtocolBinding != ProtocolJSONRPC {
		return ListTasksResult{}, ErrUnsupportedProtocol
	}
	if strings.TrimSpace(req.EndpointURL) == "" {
		return ListTasksResult{}, errors.New("a2a endpoint url is required")
	}

	client, err := c.getClient(ctx, req.EndpointURL, req.ProtocolBinding, req.ProtocolVersion, c.timeout())
	if err != nil {
		return ListTasksResult{}, err
	}

	sdkReq := &a2a.ListTasksRequest{
		ContextID: req.ContextID,
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
	}

	params := make(a2aclient.ServiceParams)
	if req.BearerToken != "" {
		params.Append("Authorization", "Bearer "+req.BearerToken)
	}
	if req.ProtocolVersion != "" {
		params.Append("A2A-Version", req.ProtocolVersion)
	} else {
		params.Append("A2A-Version", ProtocolVersion10)
	}
	ctx = a2aclient.AttachServiceParams(ctx, params)

	res, err := client.ListTasks(ctx, sdkReq)
	if err != nil {
		return ListTasksResult{}, mapError(err)
	}

	return ListTasksResult{
		Tasks:         taskRecords(res.Tasks),
		NextPageToken: res.NextPageToken,
	}, nil
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

	client, err := c.getClient(ctx, req.EndpointURL, req.ProtocolBinding, req.ProtocolVersion, c.timeout())
	if err != nil {
		return TaskRecord{}, err
	}

	sdkReq := &a2a.GetTaskRequest{
		ID: a2a.TaskID(req.TaskID),
	}
	if req.HistoryLength > 0 {
		sdkReq.HistoryLength = &req.HistoryLength
	}

	params := make(a2aclient.ServiceParams)
	if req.BearerToken != "" {
		params.Append("Authorization", "Bearer "+req.BearerToken)
	}
	if req.ProtocolVersion != "" {
		params.Append("A2A-Version", req.ProtocolVersion)
	} else {
		params.Append("A2A-Version", ProtocolVersion10)
	}
	ctx = a2aclient.AttachServiceParams(ctx, params)

	res, err := client.GetTask(ctx, sdkReq)
	if err != nil {
		return TaskRecord{}, mapError(err)
	}
	return taskRecord(res), nil
}

func taskRecords(tasks []*a2a.Task) []TaskRecord {
	records := make([]TaskRecord, 0, len(tasks))
	for _, item := range tasks {
		if item != nil {
			records = append(records, taskRecord(item))
		}
	}
	return records
}

func taskRecord(t *a2a.Task) TaskRecord {
	result := resultFromSDKTask(t)
	messages := taskMessages(t)
	return TaskRecord{
		ID:        string(t.ID),
		ContextID: fallbackID(t.ContextID, string(t.ID), "local-a2a"),
		Status:    result.Status,
		Text:      result.Text,
		Messages:  messages,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func taskMessages(t *a2a.Task) []TaskMessage {
	messages := make([]TaskMessage, 0, len(t.History)+1)
	hasAgentMessage := false
	for _, item := range t.History {
		if item == nil {
			continue
		}
		text := displayTextFromSDKMessage(item)
		if text == "" {
			continue
		}
		role := normalizeRole(string(item.Role))
		if role == "assistant" {
			hasAgentMessage = true
		}
		messages = append(messages, TaskMessage{
			ID:   item.ID,
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

func agentReplyText(t *a2a.Task) string {
	if text := displayTextFromOptionalSDKMessage(t.Status.Message); text != "" {
		return text
	}
	for _, v := range slices.Backward(t.Artifacts) {
		if v == nil {
			continue
		}
		if text := displayTextFromSDKParts(v.Parts); text != "" {
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
