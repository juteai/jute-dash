package widgetskills

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"jute-dash/internal/config"
	"jute-dash/internal/store"
)

const (
	SchemaDashboardContext = "https://jute.dev/mcp/resources/dashboard-context/v1"
	SchemaVisibleWidgets   = "https://jute.dev/mcp/resources/visible-widgets/v1"
	SchemaSkillList        = "https://jute.dev/mcp/resources/widget-skills/v1"
	SchemaSkillContext     = "https://jute.dev/mcp/resources/widget-skill-context/v1"
)

var ErrNotFound = errors.New("widget skill not found")

type ContextFunc func(snapshot Snapshot, instanceID string) map[string]any

var (
	registryMu   sync.RWMutex
	customDefs   = make(map[string]Definition)
	contextFuncs = make(map[string]ContextFunc)
)

func Register(def Definition, contextFn ContextFunc) {
	registryMu.Lock()
	defer registryMu.Unlock()
	customDefs[def.WidgetKind] = def
	if contextFn != nil {
		contextFuncs[def.SkillID] = contextFn
	}
}

type Agent struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	ProtocolBinding string   `json:"protocolBinding"`
	Enabled         bool     `json:"enabled"`
	Capabilities    []string `json:"capabilities,omitempty"`
	AuthConfigured  bool     `json:"authConfigured"`
}

type Snapshot struct {
	Config      config.Config
	Layout      store.WidgetLayout
	Agents      []Agent
	GeneratedAt time.Time
}

type Field struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Unit        string   `json:"unit,omitempty"`
	EnumValues  []string `json:"enumValues,omitempty"`
	Nullable    bool     `json:"nullable,omitempty"`
	Sensitivity string   `json:"sensitivity,omitempty"`
}

type Action struct {
	ID                   string         `json:"id"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	SideEffect           string         `json:"sideEffect"`
	RequiresConfirmation bool           `json:"requiresConfirmation"`
	InputSchema          map[string]any `json:"inputSchema"`
	OutputSchema         map[string]any `json:"outputSchema"`
}

type Prompt struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Purpose string `json:"purpose"`
}

type Definition struct {
	SkillID              string   `json:"skillId"`
	WidgetKind           string   `json:"widgetKind"`
	DisplayName          string   `json:"displayName"`
	Summary              string   `json:"summary"`
	RequiredPermissions  []string `json:"requiredPermissions"`
	VisibilityPolicy     string   `json:"visibilityPolicy"`
	ContextFields        []Field  `json:"contextFields"`
	Actions              []Action `json:"actions"`
	Prompts              []Prompt `json:"prompts"`
	SupportedWidgetSizes []string `json:"supportedWidgetSizes,omitempty"`
}

type Skill struct {
	Definition

	WidgetInstanceID string `json:"widgetInstanceId"`
	WidgetTitle      string `json:"widgetTitle"`
	WidgetSize       string `json:"widgetSize"`
	WidgetVisible    bool   `json:"widgetVisible"`
}

type SkillSummary struct {
	SkillID          string   `json:"skillId"`
	WidgetInstanceID string   `json:"widgetInstanceId"`
	WidgetKind       string   `json:"widgetKind"`
	DisplayName      string   `json:"displayName"`
	Summary          string   `json:"summary"`
	Actions          []string `json:"actions"`
	Prompts          []string `json:"prompts"`
}

type DashboardContext struct {
	Schema      string        `json:"schema"`
	GeneratedAt string        `json:"generatedAt"`
	Display     Display       `json:"display"`
	Dashboard   Dashboard     `json:"dashboard"`
	Skills      []SkillResult `json:"skills"`
}

type Display struct {
	DeviceID        string `json:"deviceId"`
	Profile         string `json:"profile"`
	Locale          string `json:"locale"`
	Timezone        string `json:"timezone"`
	InteractionMode string `json:"interactionMode"`
}

type Dashboard struct {
	VisibleWidgetIDs []string `json:"visibleWidgetIds"`
	FocusedWidgetID  string   `json:"focusedWidgetId"`
	Stale            bool     `json:"stale"`
}

type SkillResult struct {
	SkillSummary

	Context map[string]any `json:"context"`
}

type SkillList struct {
	Schema      string        `json:"schema"`
	GeneratedAt string        `json:"generatedAt"`
	Skills      []SkillResult `json:"skills"`
}

type VisibleWidgets struct {
	Schema      string          `json:"schema"`
	GeneratedAt string          `json:"generatedAt"`
	Widgets     []WidgetSummary `json:"widgets"`
}

type WidgetSummary struct {
	ID          string        `json:"id"`
	Kind        string        `json:"kind"`
	Title       string        `json:"title"`
	Size        string        `json:"size"`
	X           int           `json:"x"`
	Y           int           `json:"y"`
	W           int           `json:"w"`
	H           int           `json:"h"`
	Visible     bool          `json:"visible"`
	Skill       *SkillSummary `json:"skill,omitempty"`
	SkillURI    string        `json:"skillUri,omitempty"`
	ContextURI  string        `json:"contextUri,omitempty"`
	Permissions []string      `json:"permissions,omitempty"`
}

type SkillDefinitionResource struct {
	Schema      string   `json:"schema"`
	GeneratedAt string   `json:"generatedAt"`
	Skill       Skill    `json:"skill"`
	ContextURI  string   `json:"contextUri"`
	Actions     []Action `json:"actions"`
	Prompts     []Prompt `json:"prompts"`
}

type SkillContextResource struct {
	Schema      string         `json:"schema"`
	GeneratedAt string         `json:"generatedAt"`
	Skill       SkillSummary   `json:"skill"`
	Context     map[string]any `json:"context"`
}

func Available(snapshot Snapshot) []Skill {
	defs := definitionsByKind()
	skills := []Skill{}
	for _, widget := range snapshot.Layout.Widgets {
		if !widget.Visible {
			continue
		}
		def, ok := defs[widget.Kind]
		if !ok {
			continue
		}
		skills = append(skills, Skill{
			Definition:       def,
			WidgetInstanceID: widget.ID,
			WidgetTitle:      firstNonEmpty(widget.Title, def.DisplayName),
			WidgetSize:       widget.Size,
			WidgetVisible:    widget.Visible,
		})
	}
	return skills
}

func DashboardSnapshot(snapshot Snapshot) DashboardContext {
	skills := Available(snapshot)
	results := make([]SkillResult, 0, len(skills))
	visibleWidgetIDs := make([]string, 0, len(snapshot.Layout.Widgets))
	for _, widget := range snapshot.Layout.Widgets {
		if widget.Visible {
			visibleWidgetIDs = append(visibleWidgetIDs, widget.ID)
		}
	}
	for _, skill := range skills {
		results = append(results, SkillResult{
			SkillSummary: summarize(skill),
			Context:      contextForSkill(snapshot, skill),
		})
	}
	return DashboardContext{
		Schema:      SchemaDashboardContext,
		GeneratedAt: generatedAt(snapshot),
		Display: Display{
			DeviceID:        "default-display",
			Profile:         firstNonEmpty(snapshot.Layout.ProfileID, "default-dashboard"),
			Locale:          snapshot.Config.Home.Locale,
			Timezone:        snapshot.Config.Home.Timezone,
			InteractionMode: "touch",
		},
		Dashboard: Dashboard{
			VisibleWidgetIDs: visibleWidgetIDs,
			FocusedWidgetID:  "",
			Stale:            false,
		},
		Skills: results,
	}
}

func SkillListSnapshot(snapshot Snapshot) SkillList {
	skills := Available(snapshot)
	results := make([]SkillResult, 0, len(skills))
	for _, skill := range skills {
		results = append(results, SkillResult{
			SkillSummary: summarize(skill),
			Context:      contextForSkill(snapshot, skill),
		})
	}
	return SkillList{
		Schema:      SchemaSkillList,
		GeneratedAt: generatedAt(snapshot),
		Skills:      results,
	}
}

func VisibleWidgetsSnapshot(snapshot Snapshot) VisibleWidgets {
	skillsByWidget := map[string]Skill{}
	for _, skill := range Available(snapshot) {
		skillsByWidget[skill.WidgetInstanceID] = skill
	}
	widgets := make([]WidgetSummary, 0, len(snapshot.Layout.Widgets))
	for _, widget := range snapshot.Layout.Widgets {
		if !widget.Visible {
			continue
		}
		summary := WidgetSummary{
			ID:      widget.ID,
			Kind:    widget.Kind,
			Title:   widget.Title,
			Size:    widget.Size,
			X:       widget.X,
			Y:       widget.Y,
			W:       widget.W,
			H:       widget.H,
			Visible: widget.Visible,
		}
		if skill, ok := skillsByWidget[widget.ID]; ok {
			skillSummary := summarize(skill)
			summary.Skill = &skillSummary
			summary.SkillURI = "jute://widgets/" + widget.ID + "/skill"
			summary.ContextURI = "jute://widgets/" + widget.ID + "/context"
			summary.Permissions = append([]string(nil), skill.RequiredPermissions...)
		}
		widgets = append(widgets, summary)
	}
	return VisibleWidgets{
		Schema:      SchemaVisibleWidgets,
		GeneratedAt: generatedAt(snapshot),
		Widgets:     widgets,
	}
}

func SkillDefinition(snapshot Snapshot, skillID string) (SkillDefinitionResource, error) {
	skill, err := findSkill(snapshot, skillID, "")
	if err != nil {
		return SkillDefinitionResource{}, err
	}
	return SkillDefinitionResource{
		Schema:      SchemaSkillList,
		GeneratedAt: generatedAt(snapshot),
		Skill:       skill,
		ContextURI:  "jute://skills/" + skill.SkillID + "/context",
		Actions:     append([]Action(nil), skill.Actions...),
		Prompts:     append([]Prompt(nil), skill.Prompts...),
	}, nil
}

func SkillContext(snapshot Snapshot, skillID string, widgetID string) (SkillContextResource, error) {
	skill, err := findSkill(snapshot, skillID, widgetID)
	if err != nil {
		return SkillContextResource{}, err
	}
	return SkillContextResource{
		Schema:      SchemaSkillContext,
		GeneratedAt: generatedAt(snapshot),
		Skill:       summarize(skill),
		Context:     contextForSkill(snapshot, skill),
	}, nil
}

func WidgetSkill(snapshot Snapshot, widgetID string) (SkillDefinitionResource, error) {
	for _, skill := range Available(snapshot) {
		if skill.WidgetInstanceID == widgetID {
			return SkillDefinition(snapshot, skill.SkillID)
		}
	}
	return SkillDefinitionResource{}, ErrNotFound
}

func WidgetContext(snapshot Snapshot, widgetID string) (SkillContextResource, error) {
	skill, err := findSkill(snapshot, "", widgetID)
	if err != nil {
		return SkillContextResource{}, err
	}
	return SkillContext(snapshot, skill.SkillID, widgetID)
}

func InvokeAction(
	snapshot Snapshot,
	skillID string,
	widgetID string,
	actionID string,
	arguments map[string]any,
) (map[string]any, error) {
	skill, err := findSkill(snapshot, skillID, widgetID)
	if err != nil {
		return nil, err
	}
	for _, action := range skill.Actions {
		if action.ID == actionID {
			context := contextForSkill(snapshot, skill)
			return map[string]any{
				"status":           "completed",
				"skillId":          skill.SkillID,
				"widgetInstanceId": skill.WidgetInstanceID,
				"actionId":         action.ID,
				"sideEffect":       action.SideEffect,
				"arguments":        cleanArguments(arguments),
				"context":          context,
				"updatedAt":        generatedAt(snapshot),
			}, nil
		}
	}
	return nil, fmt.Errorf("%w: action not found", ErrNotFound)
}

func PromptText(snapshot Snapshot, skillID string, promptID string) (string, error) {
	if skillID == "" {
		return homeAssistantGuidance(), nil
	}
	skill, err := findSkill(snapshot, skillID, "")
	if err != nil {
		return "", err
	}
	for _, prompt := range skill.Prompts {
		if prompt.ID == promptID {
			return fmt.Sprintf(
				"Use the %s widget skill (%s) only through its public context and declared actions. Purpose: %s. Do not infer hidden widget state, credentials, private household data, camera frames, microphone audio, or browser storage.",
				skill.DisplayName,
				skill.SkillID,
				prompt.Purpose,
			), nil
		}
	}
	return "", fmt.Errorf("%w: prompt not found", ErrNotFound)
}

func HomeAssistantGuidance() string {
	return homeAssistantGuidance()
}

func summarize(skill Skill) SkillSummary {
	actionIDs := make([]string, 0, len(skill.Actions))
	for _, action := range skill.Actions {
		actionIDs = append(actionIDs, action.ID)
	}
	promptIDs := make([]string, 0, len(skill.Prompts))
	for _, prompt := range skill.Prompts {
		promptIDs = append(promptIDs, prompt.ID)
	}
	return SkillSummary{
		SkillID:          skill.SkillID,
		WidgetInstanceID: skill.WidgetInstanceID,
		WidgetKind:       skill.WidgetKind,
		DisplayName:      skill.DisplayName,
		Summary:          skill.Summary,
		Actions:          actionIDs,
		Prompts:          promptIDs,
	}
}

func findSkill(snapshot Snapshot, skillID string, widgetID string) (Skill, error) {
	for _, skill := range Available(snapshot) {
		if skillID != "" && skill.SkillID != skillID {
			continue
		}
		if widgetID != "" && skill.WidgetInstanceID != widgetID {
			continue
		}
		return skill, nil
	}
	return Skill{}, ErrNotFound
}

func contextForSkill(snapshot Snapshot, skill Skill) map[string]any {
	registryMu.RLock()
	fn, exists := contextFuncs[skill.SkillID]
	registryMu.RUnlock()

	if exists && fn != nil {
		return fn(snapshot, skill.WidgetInstanceID)
	}

	return defaultContextExtractor(snapshot, skill)
}

func defaultContextExtractor(snapshot Snapshot, skill Skill) map[string]any {
	ctxMap := make(map[string]any)

	// Find the widget instance in the layout
	var targetWidget *store.WidgetInstance
	for i := range snapshot.Layout.Widgets {
		if snapshot.Layout.Widgets[i].ID == skill.WidgetInstanceID {
			targetWidget = &snapshot.Layout.Widgets[i]
			break
		}
	}

	if targetWidget == nil {
		return ctxMap
	}

	if targetWidget.Data != nil {
		ctxMap["data"] = targetWidget.Data
	}

	// Copy declared ContextFields from the instance's Settings
	for _, field := range skill.ContextFields {
		if val, exists := targetWidget.Settings[field.Name]; exists {
			ctxMap[field.Name] = val
		} else if field.Nullable {
			ctxMap[field.Name] = nil
		}
	}

	return ctxMap
}

func definitionsByKind() map[string]Definition {
	registryMu.RLock()
	defer registryMu.RUnlock()
	byKind := make(map[string]Definition, len(customDefs))
	maps.Copy(byKind, customDefs)
	return byKind
}

func ReadAction(id, title, description string) Action {
	return Action{
		ID:                   id,
		Title:                title,
		Description:          description,
		SideEffect:           "read",
		RequiresConfirmation: false,
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{"type": "string"},
			},
			"required": []string{"status"},
		},
	}
}

func homeAssistantGuidance() string {
	return strings.Join([]string{
		"You are a Jute Dash home assistant agent.",
		"Return only the final user-facing answer in A2A messages. Never include private reasoning, scratchpad text, analysis, tool-selection notes, function-call plans, or statements like \"I should\" or \"no need to call tools\" in assistant output.",
		"Use Jute MCP resources and Widget Skills to understand only the visible dashboard context that the hub exposes.",
		"Prefer skill context over guesses. Invoke only declared actions, and only when the user's request requires that action or context.",
		"For simple greetings or ordinary chat, reply naturally without calling tools.",
		"Before using a tool, choose the narrowest relevant Jute resource or Widget Skill action. Do not invent tools or capabilities that are not listed.",
		"Do not infer hidden widget state, secrets, private household data, camera frames, microphone audio, browser storage, or raw adapter payloads.",
		"If context is missing or unauthorized, say so briefly and continue the A2A conversation without it.",
	}, " ")
}

func cleanArguments(arguments map[string]any) map[string]any {
	if arguments == nil {
		return map[string]any{}
	}
	cleaned := make(map[string]any, len(arguments))
	maps.Copy(cleaned, arguments)
	return cleaned
}

func generatedAt(snapshot Snapshot) string {
	if snapshot.GeneratedAt.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return snapshot.GeneratedAt.UTC().Format(time.RFC3339Nano)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
