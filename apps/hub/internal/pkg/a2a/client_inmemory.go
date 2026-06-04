package a2a

import (
	"context"
	"sync"
)

// InMemoryClient is a test adapter implementing the Client interface.
type InMemoryClient struct {
	mu sync.RWMutex

	// Configurable stubs
	SendMessageFunc   func(ctx context.Context, req SendMessageRequest) (SendMessageResult, error)
	StreamMessageFunc func(ctx context.Context, req SendMessageRequest, handler StreamHandler) error
	ListTasksFunc     func(ctx context.Context, req ListTasksRequest) (ListTasksResult, error)
	GetTaskFunc       func(ctx context.Context, req GetTaskRequest) (TaskRecord, error)

	// Spy records of calls made to this client
	SentMessages      []SendMessageRequest
	StreamRequests    []SendMessageRequest
	ListTasksRequests []ListTasksRequest
	GetTaskRequests   []GetTaskRequest

	// Simplified stub values
	sendMessageResult SendMessageResult
	sendMessageErr    error
	streamEvents      []StreamEvent
	streamErr         error
	listTasksResult   ListTasksResult
	listTasksErr      error
	getTaskRecord     TaskRecord
	getTaskErr        error
}

// NewInMemoryClient returns a configured in-memory client.
func NewInMemoryClient() *InMemoryClient {
	return &InMemoryClient{
		sendMessageResult: SendMessageResult{
			ConversationID: "ctx-inmemory",
			Status:         "completed",
			Text:           "Default in-memory message reply",
		},
		listTasksResult: ListTasksResult{
			Tasks: []TaskRecord{},
		},
	}
}

// StubSendMessage sets a simple static result for SendMessage.
func (c *InMemoryClient) StubSendMessage(res SendMessageResult, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sendMessageResult = res
	c.sendMessageErr = err
}

// StubStreamMessage sets stream events and error for StreamMessage.
func (c *InMemoryClient) StubStreamMessage(events []StreamEvent, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streamEvents = events
	c.streamErr = err
}

// StubListTasks sets tasks result and error for ListTasks.
func (c *InMemoryClient) StubListTasks(res ListTasksResult, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listTasksResult = res
	c.listTasksErr = err
}

// StubGetTask sets task record and error for GetTask.
func (c *InMemoryClient) StubGetTask(record TaskRecord, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getTaskRecord = record
	c.getTaskErr = err
}

// SendMessage implements Client.
func (c *InMemoryClient) SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResult, error) {
	c.mu.Lock()
	c.SentMessages = append(c.SentMessages, req)
	fn := c.SendMessageFunc
	res := c.sendMessageResult
	err := c.sendMessageErr
	c.mu.Unlock()

	if fn != nil {
		return fn(ctx, req)
	}
	return res, err
}

// StreamMessage implements Client.
func (c *InMemoryClient) StreamMessage(ctx context.Context, req SendMessageRequest, handler StreamHandler) error {
	c.mu.Lock()
	c.StreamRequests = append(c.StreamRequests, req)
	fn := c.StreamMessageFunc
	events := append([]StreamEvent(nil), c.streamEvents...)
	err := c.streamErr
	c.mu.Unlock()

	if fn != nil {
		return fn(ctx, req, handler)
	}

	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := handler(event); err != nil {
				return err
			}
		}
	}
	return err
}

// ListTasks implements Client.
func (c *InMemoryClient) ListTasks(ctx context.Context, req ListTasksRequest) (ListTasksResult, error) {
	c.mu.Lock()
	c.ListTasksRequests = append(c.ListTasksRequests, req)
	fn := c.ListTasksFunc
	res := c.listTasksResult
	err := c.listTasksErr
	c.mu.Unlock()

	if fn != nil {
		return fn(ctx, req)
	}
	return res, err
}

// GetTask implements Client.
func (c *InMemoryClient) GetTask(ctx context.Context, req GetTaskRequest) (TaskRecord, error) {
	c.mu.Lock()
	c.GetTaskRequests = append(c.GetTaskRequests, req)
	fn := c.GetTaskFunc
	res := c.getTaskRecord
	err := c.getTaskErr
	c.mu.Unlock()

	if fn != nil {
		return fn(ctx, req)
	}
	return res, err
}

// Compile-time interface check.
var _ Client = (*InMemoryClient)(nil)
