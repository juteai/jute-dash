package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
	"jute-dash/apps/hub/pkg/widgetskills"
)

// ==========================================
// 1. Home State Service (from internal/home)
// ==========================================

type State struct {
	GeneratedAt time.Time     `json:"generatedAt"`
	Home        HomeConfig    `json:"home"`
	Rooms       []RoomSummary `json:"rooms"`
	Tiles       []TileState   `json:"tiles"`
}

type RoomSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type TileState struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

func HomeStateFromConfig(cfg Config, now time.Time) State {
	rooms := make([]RoomSummary, 0, len(cfg.Rooms))
	for _, room := range cfg.Rooms {
		rooms = append(rooms, RoomSummary{
			ID:      room.ID,
			Name:    room.Name,
			Summary: room.Summary,
			Status:  room.Status,
		})
	}

	tiles := make([]TileState, 0, len(cfg.Tiles))
	for _, tile := range cfg.Tiles {
		tiles = append(tiles, TileState{
			ID:     tile.ID,
			Kind:   tile.Kind,
			Label:  tile.Label,
			Value:  tile.Value,
			Detail: tile.Detail,
		})
	}

	return State{
		GeneratedAt: now.UTC(),
		Home:        cfg.Home,
		Rooms:       rooms,
		Tiles:       tiles,
	}
}

// ======================================================
// 2. Dashboard Context Projector (from dashboardcontext)
// ======================================================

type DashboardSnapshot struct {
	GeneratedAt time.Time
	Display     DisplayInfo
	Dashboard   DashboardInfo
	Widgets     []WidgetInfo
}

type DisplayInfo struct {
	DeviceID        string
	Profile         string
	Locale          string
	Timezone        string
	InteractionMode string
}

type DashboardInfo struct {
	VisibleWidgetIDs []string
	FocusedWidgetID  string
	Stale            bool
}

type WidgetInfo struct {
	ID            string
	Kind          string
	Title         string
	Size          string
	X, Y, W, H    int
	Permissions   []string
	PublicContext map[string]any
}

func Project(ctx context.Context, layout WidgetLayout, cfg Config) DashboardSnapshot {
	// Map app configuration to widgetskills types to prevent circular dependencies
	wsCfg := widgetskills.Config{}
	wsCfg.Home.Locale = cfg.Home.Locale
	wsCfg.Home.Timezone = cfg.Home.Timezone

	wsWidgets := make([]widgetskills.WidgetInstance, len(layout.Widgets))
	for i, w := range layout.Widgets {
		wsWidgets[i] = widgetskills.WidgetInstance{
			ID:       w.ID,
			Kind:     w.Kind,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			W:        w.W,
			H:        w.H,
			Visible:  w.Visible,
			Size:     w.Size,
			Settings: w.Settings,
			Data:     w.Data,
		}
	}
	wsLayout := widgetskills.WidgetLayout{
		ProfileID: layout.ProfileID,
		Widgets:   wsWidgets,
	}

	snap := widgetskills.Snapshot{
		Config:      wsCfg,
		Layout:      wsLayout,
		GeneratedAt: time.Now().UTC(),
	}

	skills := widgetskills.Available(snap)
	skillsByWidget := make(map[string]widgetskills.Skill, len(skills))
	for _, skill := range skills {
		skillsByWidget[skill.WidgetInstanceID] = skill
	}

	visibleWidgetIDs := make([]string, 0, len(layout.Widgets))
	widgets := make([]WidgetInfo, 0, len(layout.Widgets))

	for _, widget := range layout.Widgets {
		if !widget.Visible {
			continue
		}
		visibleWidgetIDs = append(visibleWidgetIDs, widget.ID)

		info := WidgetInfo{
			ID:    widget.ID,
			Kind:  widget.Kind,
			Title: widget.Title,
			Size:  widget.Size,
			X:     widget.X,
			Y:     widget.Y,
			W:     widget.W,
			H:     widget.H,
		}

		if skill, ok := skillsByWidget[widget.ID]; ok {
			info.Permissions = append([]string(nil), skill.RequiredPermissions...)
			info.PublicContext = widgetskills.ContextForSkill(snap, skill)
		} else {
			info.PublicContext = map[string]any{}
			if widget.Data != nil {
				info.PublicContext["data"] = widget.Data
			}
		}

		widgets = append(widgets, info)
	}

	return DashboardSnapshot{
		GeneratedAt: snap.GeneratedAt,
		Display: DisplayInfo{
			DeviceID:        "default-display",
			Profile:         firstNonEmpty(layout.ProfileID, "default-dashboard"),
			Locale:          cfg.Home.Locale,
			Timezone:        cfg.Home.Timezone,
			InteractionMode: "touch",
		},
		Dashboard: DashboardInfo{
			VisibleWidgetIDs: visibleWidgetIDs,
			FocusedWidgetID:  "",
			Stale:            false,
		},
		Widgets: widgets,
	}
}

// ==========================================
// 3. Turn Runner Service (from turnrunner)
// ==========================================

type ConversationCreateRequest struct {
	AgentID     string `json:"agentId"`
	Title       string `json:"title,omitempty"`
	InitialText string `json:"initialText,omitempty"`
}

type ConversationTurnRequest struct {
	AgentID string `json:"agentId"`
	Text    string `json:"text"`
}

type Conversation struct {
	ID                 string `json:"id"`
	AgentID            string `json:"agentId"`
	Title              string `json:"title"`
	Status             string `json:"status"`
	A2AContextID       string `json:"a2aContextId"`
	LatestTaskID       string `json:"latestTaskId"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	HistoryUnsupported bool   `json:"historyUnsupported,omitempty"`
}

type ConversationMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	AgentID        string `json:"agentId"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Status         string `json:"status"`
	A2AMessageID   string `json:"a2aMessageId"`
	A2ATaskID      string `json:"a2aTaskId"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ConversationDetail struct {
	Conversation Conversation          `json:"conversation"`
	Messages     []ConversationMessage `json:"messages"`
}

type AgentCardCache struct {
	SelectedEndpointURL       string
	SelectedProtocolBinding   string
	SelectedProtocolVersion   string
	Streaming                 bool
	DashboardContextSupported bool
}

type RunnerOptions struct {
	Registry            registry.Registry
	GetAgentConfig      func(string) (AgentConfig, bool)
	GetAgentCardCache   func(ctx context.Context, agent registry.Agent) (AgentCardCache, bool)
	GetDashboardContext func(ctx context.Context) map[string]any
	Messages            a2aclient.MessageSender
}

type Runner struct {
	opts RunnerOptions
}

func NewRunner(opts RunnerOptions) *Runner {
	return &Runner{opts: opts}
}

type EventKind string

const (
	EventTurnStarted    EventKind = "turn_started"
	EventAssistantDelta EventKind = "assistant_delta"
	EventStatusChanged  EventKind = "status_changed"
	EventTurnCompleted  EventKind = "turn_completed"
	EventTurnFailed     EventKind = "turn_failed"
)

type Event struct {
	Kind           EventKind           `json:"kind"`
	ConversationID string              `json:"conversationId"`
	AgentID        string              `json:"agentId"`
	TaskID         string              `json:"taskId,omitempty"`
	Status         string              `json:"status,omitempty"`
	Text           string              `json:"text,omitempty"`
	Append         bool                `json:"append,omitempty"`
	Terminal       bool                `json:"terminal,omitempty"`
	Detail         *ConversationDetail `json:"detail,omitempty"`
	Message        string              `json:"message,omitempty"`
}

type turnContext struct {
	agent       registry.Agent
	bearerToken string
	sendRequest a2aclient.SendMessageRequest
	streaming   bool
}

func (runner *Runner) prepare(
	ctx context.Context,
	conversationID string,
	req ConversationTurnRequest,
) (turnContext, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return turnContext{}, errors.New("text is required")
	}
	agent, ok := runner.opts.Registry.Find(strings.TrimSpace(req.AgentID))
	if !ok {
		return turnContext{}, errors.New("agent not found")
	}
	if !agent.Enabled {
		return turnContext{}, errors.New("agent is disabled")
	}
	configured, ok := runner.opts.GetAgentConfig(agent.ID)
	if !ok {
		return turnContext{}, errors.New("agent not found")
	}

	selected := selectedAgentInterface{
		EndpointURL:     agent.EndpointURL,
		ProtocolBinding: agent.ProtocolBinding,
		ProtocolVersion: a2aclient.ProtocolVersion10,
	}

	cache, ok := runner.opts.GetAgentCardCache(ctx, agent)
	if ok && cache.SelectedEndpointURL != "" {
		selected.EndpointURL = cache.SelectedEndpointURL
		selected.ProtocolBinding = cache.SelectedProtocolBinding
		selected.ProtocolVersion = cache.SelectedProtocolVersion
		selected.Streaming = cache.Streaming
		if cache.DashboardContextSupported {
			selected.Extensions = []string{a2aclient.DashboardContextExtensionURI}
			selected.Metadata = map[string]any{
				a2aclient.DashboardContextExtensionURI: runner.opts.GetDashboardContext(ctx),
			}
		}
	}

	if selected.ProtocolBinding != a2aclient.ProtocolJSONRPC {
		return turnContext{}, a2aclient.ErrUnsupportedProtocol
	}

	bearerToken, ok := agentBearerToken(configured)
	if !ok {
		return turnContext{}, errors.New("agent credentials are not available")
	}

	sendReq := a2aclient.SendMessageRequest{
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		ProtocolVersion: selected.ProtocolVersion,
		Text:            text,
		BearerToken:     bearerToken,
		ConversationID:  conversationID,
		Extensions:      selected.Extensions,
		Metadata:        selected.Metadata,
	}

	return turnContext{
		agent:       agent,
		bearerToken: bearerToken,
		sendRequest: sendReq,
		streaming:   selected.Streaming,
	}, nil
}

type selectedAgentInterface struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	Streaming       bool
	Extensions      []string
	Metadata        map[string]any
}

func agentBearerToken(agent AgentConfig) (string, bool) {
	if agent.Auth == nil {
		return "", true
	}
	if !strings.EqualFold(strings.TrimSpace(agent.Auth.Type), "bearer") {
		return "", false
	}
	token := strings.TrimSpace(osGetenv(agent.Auth.EnvToken))
	if token == "" {
		return "", false
	}
	return token, true
}

// Injected [os.Getenv] fallback
//
//nolint:gochecknoglobals // mock fallback function for testing
var osGetenv = func(name string) string {
	return os.Getenv(name)
}

func SetEnvReader(fn func(string) string) {
	osGetenv = fn
}

func (runner *Runner) Run(
	ctx context.Context,
	conversationID string,
	req ConversationTurnRequest,
	callback func(Event) error,
) (ConversationDetail, error) {
	turn, err := runner.prepare(ctx, conversationID, req)
	if err != nil {
		return ConversationDetail{}, err
	}

	if callback != nil {
		_ = callback(Event{
			Kind:           EventTurnStarted,
			ConversationID: conversationID,
			AgentID:        turn.agent.ID,
			Status:         "working",
		})
	}

	var streamText strings.Builder
	var taskID string
	status := "working"
	activeConversationID := conversationID

	if turn.streaming {
		if streamer, ok := runner.opts.Messages.(a2aclient.StreamingMessageSender); ok {
			streamedDelta := false
			err := streamer.StreamMessage(ctx, turn.sendRequest, func(event a2aclient.StreamEvent) error {
				if event.ConversationID != "" {
					activeConversationID = event.ConversationID
				}
				if event.TaskID != "" {
					taskID = event.TaskID
				}
				if event.Status != "" {
					status = event.Status
				}
				switch event.Kind {
				case "artifact", "message":
					if event.Text != "" {
						streamedDelta = true
						if event.Append {
							streamText.WriteString(event.Text)
						} else {
							streamText.Reset()
							streamText.WriteString(event.Text)
						}
						if callback != nil {
							_ = callback(Event{
								Kind:           EventAssistantDelta,
								ConversationID: activeConversationID,
								AgentID:        turn.agent.ID,
								TaskID:         taskID,
								Text:           event.Text,
								Append:         event.Append,
							})
						}
					}
				case "task", "status":
					if callback != nil {
						_ = callback(Event{
							Kind:           EventStatusChanged,
							ConversationID: activeConversationID,
							AgentID:        turn.agent.ID,
							TaskID:         taskID,
							Status:         status,
							Terminal:       event.Terminal,
						})
					}
				}
				return nil
			})

			if err == nil {
				detail := runner.detailAfterTurn(
					ctx,
					turn.agent,
					turn.sendRequest,
					turn.bearerToken,
					activeConversationID,
					taskID,
					turn.sendRequest.Text,
					streamText.String(),
					status,
				)
				if callback != nil {
					_ = callback(Event{
						Kind:           EventTurnCompleted,
						ConversationID: activeConversationID,
						AgentID:        turn.agent.ID,
						Detail:         &detail,
					})
				}
				return detail, nil
			}

			if streamedDelta {
				return ConversationDetail{}, err
			}
		}
	}

	result, err := runner.opts.Messages.SendMessage(ctx, turn.sendRequest)
	if err != nil {
		return ConversationDetail{}, err
	}

	detail := runner.detailAfterTurn(
		ctx,
		turn.agent,
		turn.sendRequest,
		turn.bearerToken,
		result.ConversationID,
		result.TaskID,
		turn.sendRequest.Text,
		result.Text,
		result.Status,
	)
	if callback != nil {
		_ = callback(Event{
			Kind:           EventTurnCompleted,
			ConversationID: result.ConversationID,
			AgentID:        turn.agent.ID,
			Detail:         &detail,
		})
	}
	return detail, nil
}

func (runner *Runner) detailAfterTurn(
	ctx context.Context,
	agent registry.Agent,
	sendReq a2aclient.SendMessageRequest,
	bearerToken, conversationID, taskID, userText, assistantText, status string,
) ConversationDetail {
	if taskID != "" {
		if history, ok := runner.opts.Messages.(a2aclient.TaskHistoryClient); ok {
			if task, err := history.GetTask(ctx, a2aclient.GetTaskRequest{
				EndpointURL:     sendReq.EndpointURL,
				ProtocolBinding: sendReq.ProtocolBinding,
				ProtocolVersion: sendReq.ProtocolVersion,
				BearerToken:     bearerToken,
				TaskID:          taskID,
				HistoryLength:   50,
			}); err == nil {
				return ConversationDetail{
					Conversation: ConversationFromTask(agent, task),
					Messages:     ConversationMessages(agent.ID, FirstNonEmpty(task.ContextID, conversationID), task),
				}
			}
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	conversation := Conversation{
		ID:           conversationID,
		AgentID:      agent.ID,
		Title:        userText,
		Status:       FirstNonEmpty(status, "completed"),
		A2AContextID: conversationID,
		LatestTaskID: taskID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return ConversationDetail{
		Conversation: conversation,
		Messages: []ConversationMessage{
			{
				ID:             "local-user-" + NewLocalID(),
				ConversationID: conversationID,
				AgentID:        agent.ID,
				Role:           "user",
				Content:        userText,
				Status:         "sent",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "local-agent-" + NewLocalID(),
				ConversationID: conversationID,
				AgentID:        agent.ID,
				Role:           "assistant",
				Content:        assistantText,
				Status:         "sent",
				A2ATaskID:      taskID,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
}

func ConversationFromTask(agent registry.Agent, task a2aclient.TaskRecord) Conversation {
	return Conversation{
		ID:           FirstNonEmpty(task.ContextID, task.ID),
		AgentID:      agent.ID,
		Title:        FirstNonEmpty(FirstUserText(task.Messages), task.Text, agent.Name),
		Status:       task.Status,
		A2AContextID: FirstNonEmpty(task.ContextID, task.ID),
		LatestTaskID: task.ID,
		CreatedAt:    task.UpdatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func ConversationMessages(agentID, conversationID string, task a2aclient.TaskRecord) []ConversationMessage {
	messages := make([]ConversationMessage, 0, len(task.Messages))
	for i, message := range task.Messages {
		messages = append(messages, ConversationMessage{
			ID:             FirstNonEmpty(message.ID, fmt.Sprintf("%s-%d", task.ID, i)),
			ConversationID: conversationID,
			AgentID:        agentID,
			Role:           message.Role,
			Content:        message.Text,
			Status:         "sent",
			A2ATaskID:      task.ID,
			CreatedAt:      FirstNonEmpty(message.CreatedAt, task.UpdatedAt),
			UpdatedAt:      task.UpdatedAt,
		})
	}
	return messages
}

func UnsupportedConversation(agent registry.Agent) Conversation {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return Conversation{
		ID:                 "history-unsupported-" + agent.ID,
		AgentID:            agent.ID,
		Title:              "History unavailable for this agent",
		Status:             "unsupported",
		CreatedAt:          now,
		UpdatedAt:          now,
		HistoryUnsupported: true,
	}
}

func FirstUserText(messages []a2aclient.TaskMessage) string {
	for _, message := range messages {
		if message.Role == "user" && strings.TrimSpace(message.Text) != "" {
			return strings.TrimSpace(message.Text)
		}
	}
	return ""
}

func FirstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func NewLocalID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func firstNonEmpty(candidates ...string) string {
	for _, val := range candidates {
		if val != "" {
			return val
		}
	}
	return ""
}
