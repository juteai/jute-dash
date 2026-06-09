package a2a

import (
	"context"
	"errors"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

func (c *JSONRPCClient) StreamMessage(
	ctx context.Context,
	req SendMessageRequest,
	handler StreamHandler,
) error {
	if req.ProtocolBinding != "" && req.ProtocolBinding != ProtocolJSONRPC {
		return ErrUnsupportedProtocol
	}
	if strings.TrimSpace(req.EndpointURL) == "" {
		return errors.New("a2a endpoint url is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("a2a message text is required")
	}
	if handler == nil {
		return errors.New("a2a stream handler is required")
	}

	client, err := c.getClient(ctx, req.EndpointURL, req.ProtocolBinding, req.ProtocolVersion, 0)
	if err != nil {
		return err
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

	eventsSeq := client.SendStreamingMessage(ctx, sdkReq)

	var streamErr error
	for event, err := range eventsSeq {
		if err != nil {
			streamErr = mapError(err)
			break
		}

		localEvent, ok := mapSDKEventToLocal(event)
		if !ok {
			continue
		}

		if err := handler(localEvent); err != nil {
			streamErr = err
			break
		}

		if localEvent.Terminal {
			break
		}
	}

	return streamErr
}

func mapSDKEventToLocal(event a2a.Event) (StreamEvent, bool) {
	switch v := event.(type) {
	case *a2a.Message:
		return StreamEvent{
			Kind:           "message",
			ConversationID: v.ContextID,
			TaskID:         string(v.TaskID),
			Status:         "completed",
			Text:           displayTextFromSDKMessage(v),
			Terminal:       true,
		}, true
	case *a2a.Task:
		return StreamEvent{
			Kind:           "task",
			ConversationID: v.ContextID,
			TaskID:         string(v.ID),
			Status:         fallbackID(normalizeTaskState(string(v.Status.State)), "working"),
			Text:           displayTextFromOptionalSDKMessage(v.Status.Message),
			Terminal:       isTerminalTaskState(string(v.Status.State)),
		}, true
	case *a2a.TaskStatusUpdateEvent:
		return StreamEvent{
			Kind:           "status",
			ConversationID: v.ContextID,
			TaskID:         string(v.TaskID),
			Status:         fallbackID(normalizeTaskState(string(v.Status.State)), "working"),
			Text:           displayTextFromOptionalSDKMessage(v.Status.Message),
			Terminal:       isTerminalTaskState(string(v.Status.State)),
		}, true
	case *a2a.TaskArtifactUpdateEvent:
		if v.Artifact == nil {
			return StreamEvent{}, false
		}
		return StreamEvent{
			Kind:           "artifact",
			ConversationID: v.ContextID,
			TaskID:         string(v.TaskID),
			Status:         "working",
			Text:           displayTextFromSDKParts(v.Artifact.Parts),
			Append:         v.Append,
			Terminal:       false,
		}, true
	default:
		return StreamEvent{}, false
	}
}

func isTerminalTaskState(state string) bool {
	switch normalizeTaskState(state) {
	case "completed", "failed", "canceled", "rejected":
		return true
	default:
		return false
	}
}
