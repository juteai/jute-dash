package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	WeatherSkillID = "jute.weather.current"
)

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

func NewFromEnv() (*Client, bool, error) {
	url := strings.TrimSpace(os.Getenv("JUTE_MCP_URL"))
	if url == "" {
		return nil, false, nil
	}
	timeout := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("JUTE_MCP_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, true, fmt.Errorf("invalid JUTE_MCP_TIMEOUT")
		}
		timeout = parsed
	}
	client, err := New(Config{
		URL:         url,
		BearerToken: os.Getenv("JUTE_MCP_TOKEN"),
		AgentID:     os.Getenv("JUTE_MCP_AGENT_ID"),
		Timeout:     timeout,
	})
	return client, true, err
}

func (c *Client) CollectJuteContext(ctx context.Context) JuteContext {
	if err := c.Initialize(ctx); err != nil {
		return JuteContext{Unavailable: "MCP initialize failed"}
	}
	dashboardRead := false
	if _, err := c.ReadResourceText(ctx, "jute://dashboard/current"); err == nil {
		dashboardRead = true
	}
	text, err := c.ReadResourceText(ctx, "jute://skills")
	if err != nil {
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
		if skill.SkillID == WeatherSkillID {
			summary.Weather = weatherFromContext(skill.Context)
			if contains(skill.Actions, "refresh") {
				if result, err := c.CallTool(ctx, "jute_skill_invoke_action", map[string]any{
					"skillId":  WeatherSkillID,
					"actionId": "refresh",
				}); err == nil && !result.IsError {
					summary.WeatherRefresh = "completed"
				} else {
					summary.WeatherRefresh = "unavailable"
				}
			}
		}
	}
	if prompt, err := c.GetPrompt(ctx, "jute_home_assistant_guidance", nil); err == nil && len(prompt.Messages) > 0 {
		summary.Prompt = truncate(prompt.Messages[0].Content.Text, 160)
	}
	return summary
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
