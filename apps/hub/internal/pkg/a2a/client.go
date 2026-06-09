package a2a

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2acompat/a2av0"
)

var (
	ErrUnsupportedProtocol = errors.New("a2a protocol binding is not implemented")
	ErrAgentRPCFailure     = errors.New("agent returned an a2a json-rpc error")
	ErrAgentTransport      = errors.New("agent transport request failed")
)

type SendMessageRequest struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	Text            string
	BearerToken     string
	ConversationID  string
	TaskID          string
	Extensions      []string
	Metadata        map[string]any
}

type SendMessageResult struct {
	ConversationID string
	TaskID         string
	Status         string
	Text           string
}

type MessageSender interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResult, error)
}

type StreamEvent struct {
	Kind           string
	ConversationID string
	TaskID         string
	Status         string
	Text           string
	Append         bool
	Terminal       bool
}

type StreamHandler func(StreamEvent) error

type StreamingMessageSender interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResult, error)
	StreamMessage(ctx context.Context, req SendMessageRequest, handler StreamHandler) error
}

type TaskHistoryClient interface {
	ListTasks(ctx context.Context, req ListTasksRequest) (ListTasksResult, error)
	GetTask(ctx context.Context, req GetTaskRequest) (TaskRecord, error)
}

type Client interface {
	MessageSender
	StreamingMessageSender
	TaskHistoryClient
}

type JSONRPCClient struct {
	HTTPClient *http.Client
	Logger     *slog.Logger
}

func NewJSONRPCClient() *JSONRPCClient {
	timeout := 10 * time.Minute
	if val := os.Getenv("JUTE_A2A_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			timeout = d
		}
	}
	return &JSONRPCClient{
		HTTPClient: &http.Client{Timeout: timeout},
		Logger:     slog.Default(),
	}
}

func (c *JSONRPCClient) timeout() time.Duration {
	if c.HTTPClient != nil && c.HTTPClient.Timeout > 0 {
		return c.HTTPClient.Timeout
	}
	return 10 * time.Minute
}

type RPCError struct {
	Code int
}

func (e *RPCError) Error() string {
	if e.Code == -32601 {
		return "agent does not support the requested a2a method"
	}
	return ErrAgentRPCFailure.Error()
}

func (c *JSONRPCClient) getClient(
	ctx context.Context,
	endpointURL, protocolBinding, protocolVersion string,
	timeout time.Duration,
) (*a2aclient.Client, error) {
	if protocolBinding != "" && protocolBinding != ProtocolJSONRPC {
		return nil, ErrUnsupportedProtocol
	}
	if strings.TrimSpace(endpointURL) == "" {
		return nil, errors.New("a2a endpoint url is required")
	}

	binding := protocolBinding
	if binding == "" {
		binding = ProtocolJSONRPC
	}
	version := protocolVersion
	if version == "" {
		version = ProtocolVersion10
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	} else {
		cloned := *httpClient
		cloned.Timeout = timeout
		httpClient = &cloned
	}

	iface := &a2a.AgentInterface{
		URL:             endpointURL,
		ProtocolBinding: a2a.TransportProtocol(binding),
		ProtocolVersion: a2a.ProtocolVersion(version),
	}

	opts := []a2aclient.FactoryOption{
		a2aclient.WithJSONRPCTransport(httpClient),
		a2av0.WithJSONRPCTransport(a2av0.JSONRPCTransportConfig{Client: httpClient}),
	}

	client, err := a2aclient.NewFromEndpoints(ctx, []*a2a.AgentInterface{iface}, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create a2a client: %w", err)
	}
	return client, nil
}

func (c *JSONRPCClient) SendMessage(
	ctx context.Context,
	req SendMessageRequest,
) (SendMessageResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return SendMessageResult{}, errors.New("a2a message text is required")
	}

	client, err := c.getClient(
		ctx,
		req.EndpointURL,
		req.ProtocolBinding,
		req.ProtocolVersion,
		c.timeout(),
	)
	if err != nil {
		return SendMessageResult{}, err
	}

	sdkMsg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(req.Text))
	sdkMsg.ContextID = req.ConversationID
	sdkMsg.TaskID = a2a.TaskID(req.TaskID)
	sdkMsg.Metadata = cleanMetadata(req.Metadata)

	sdkReq := &a2a.SendMessageRequest{
		Message: sdkMsg,
		Config: &a2a.SendMessageConfig{
			ReturnImmediately:   false,
			AcceptedOutputModes: []string{"text/plain"},
		},
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
	if len(req.Extensions) > 0 {
		params.Append("A2A-Extensions", strings.Join(req.Extensions, ","))
	}
	ctx = a2aclient.AttachServiceParams(ctx, params)

	res, err := client.SendMessage(ctx, sdkReq)
	if err != nil {
		return SendMessageResult{}, mapError(err)
	}

	switch v := res.(type) {
	case *a2a.Message:
		return resultFromSDKMessage(v), nil
	case *a2a.Task:
		return resultFromSDKTask(v), nil
	default:
		return SendMessageResult{}, fmt.Errorf("unexpected send message result type %T", res)
	}
}

func resultFromSDKMessage(msg *a2a.Message) SendMessageResult {
	text := displayTextFromSDKMessage(msg)
	if text == "" {
		text = "Agent returned an empty message."
	}
	return SendMessageResult{
		ConversationID: fallbackID(msg.ContextID, msg.ID, "local-a2a"),
		TaskID:         string(msg.TaskID),
		Status:         "completed",
		Text:           text,
	}
}

func resultFromSDKTask(t *a2a.Task) SendMessageResult {
	status := normalizeTaskState(string(t.Status.State))
	if status == "" {
		status = "unknown"
	}
	if text := displayTextFromOptionalSDKMessage(t.Status.Message); text != "" {
		return SendMessageResult{
			ConversationID: fallbackID(t.ContextID, string(t.ID), "local-a2a"),
			TaskID:         string(t.ID),
			Status:         status,
			Text:           text,
		}
	}
	for i := len(t.History) - 1; i >= 0; i-- {
		role := strings.TrimSpace(string(t.History[i].Role))
		if role != "agent" && role != "ROLE_AGENT" {
			continue
		}
		if text := displayTextFromSDKMessage(t.History[i]); text != "" {
			return SendMessageResult{
				ConversationID: fallbackID(t.ContextID, string(t.ID), "local-a2a"),
				TaskID:         string(t.ID),
				Status:         status,
				Text:           text,
			}
		}
	}
	for i := len(t.History) - 1; i >= 0; i-- {
		if text := displayTextFromSDKMessage(t.History[i]); text != "" {
			return SendMessageResult{
				ConversationID: fallbackID(t.ContextID, string(t.ID), "local-a2a"),
				TaskID:         string(t.ID),
				Status:         status,
				Text:           text,
			}
		}
	}
	for i := len(t.Artifacts) - 1; i >= 0; i-- {
		if text := displayTextFromSDKParts(t.Artifacts[i].Parts); text != "" {
			return SendMessageResult{
				ConversationID: fallbackID(t.ContextID, string(t.ID), "local-a2a"),
				TaskID:         string(t.ID),
				Status:         status,
				Text:           text,
			}
		}
	}
	return SendMessageResult{
		ConversationID: fallbackID(t.ContextID, string(t.ID), "local-a2a"),
		TaskID:         string(t.ID),
		Status:         status,
		Text:           fmt.Sprintf("Agent task is %s.", status),
	}
}

func fallbackID(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return "local-a2a"
}

func cleanMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	cleaned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		cleaned[key] = value
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var a2aErr *a2a.Error
	if errors.As(err, &a2aErr) {
		if a2aErr.Err != nil && strings.Contains(a2aErr.Err.Error(), "method not found") {
			return &RPCError{Code: -32601}
		}
		return ErrAgentRPCFailure
	}
	return ErrAgentTransport
}
