package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/apognu/gocal"
)

type Settings struct {
	FeedURL       string
	Username      string
	Password      string
	CalendarName  string
	Timezone      string
	LookaheadDays int
}

type Event struct {
	ID          string `json:"id"`
	UID         string `json:"uid"`
	Title       string `json:"title"`
	Calendar    string `json:"calendar"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"allDay"`
	Location    string `json:"location,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source"`
}

type Client struct {
	HTTPClient *http.Client
}

func (c Client) Fetch(ctx context.Context, settings Settings, now time.Time) ([]Event, error) {
	if strings.TrimSpace(settings.FeedURL) == "" {
		return nil, errors.New("feed URL is required")
	}
	parsed, err := url.Parse(settings.FeedURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("feed URL must be absolute")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, settings.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/calendar, application/calendar+json;q=0.5, */*;q=0.1")
	if settings.Username != "" || settings.Password != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("calendar feed returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return ParseICS(string(body), settings, now)
}

func ParseICS(input string, settings Settings, now time.Time) ([]Event, error) {
	loc := time.UTC
	if settings.Timezone != "" {
		if loaded, err := time.LoadLocation(settings.Timezone); err == nil {
			loc = loaded
		}
	}
	lookahead := settings.LookaheadDays
	if lookahead <= 0 {
		lookahead = 14
	}
	windowStart := now.Add(-24 * time.Hour).In(loc)
	windowEnd := now.AddDate(0, 0, lookahead).In(loc)

	parser := gocal.NewParser(strings.NewReader(input))
	parser.Start = &windowStart
	parser.End = &windowEnd
	parser.AllDayEventsTZ = loc
	parser.Strict.Mode = gocal.StrictModeFailEvent
	parser.Duplicate.Mode = gocal.DuplicateModeKeepLast
	if err := parser.Parse(); err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(parser.Events))
	for _, raw := range parser.Events {
		if raw.Start == nil || raw.End == nil || strings.EqualFold(raw.Status, "CANCELLED") {
			continue
		}
		start := raw.Start.UTC()
		end := raw.End.UTC()
		event := Event{
			UID:         raw.Uid,
			Title:       firstNonEmpty(raw.Summary, "Untitled event"),
			Calendar:    firstNonEmpty(settings.CalendarName, "Calendar"),
			Start:       start.Format(time.RFC3339),
			End:         end.Format(time.RFC3339),
			AllDay:      isAllDay(raw),
			Location:    raw.Location,
			Description: raw.Description,
			Source:      "ics",
		}
		event.ID = eventID(raw.Uid, start)
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start < events[j].Start
	})
	return events, nil
}

func isAllDay(event gocal.Event) bool {
	if strings.EqualFold(event.RawStart.Params["VALUE"], "DATE") {
		return true
	}
	if event.Start == nil || event.End == nil {
		return false
	}
	duration := event.End.Sub(*event.Start)
	return event.Start.Hour() == 0 &&
		event.Start.Minute() == 0 &&
		event.Start.Second() == 0 &&
		duration >= 24*time.Hour &&
		duration%day == 0
}

func eventID(uid string, start time.Time) string {
	if uid == "" {
		uid = "event"
	}
	return uid + ":" + start.UTC().Format("20060102T150405Z")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

const day = 24 * time.Hour
