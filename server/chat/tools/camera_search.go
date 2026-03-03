package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/zapdos-labs/unblink/database"
	"github.com/zapdos-labs/unblink/server/internal/ctxutil"
)

const (
	maxCameraSearchTerms         = 12
	maxCameraSearchKeywordLength = 64
	maxCameraSearchQueryLength   = 256
	defaultCameraSearchLimit     = 20
	maxCameraSearchLimit         = 100
)

// CameraSearchTool is a tool for searching camera events
type CameraSearchTool struct {
	db *database.Client
}

// NewCameraSearchTool creates a new camera search tool
func NewCameraSearchTool(db *database.Client) *CameraSearchTool {
	return &CameraSearchTool{
		db: db,
	}
}

// Id returns the stable tool identifier.
func (t *CameraSearchTool) Id() string {
	return "query_camera_events"
}

// Name returns the human-friendly tool label.
func (t *CameraSearchTool) Name() string {
	return "Search camera events"
}

// Description returns the tool description
func (t *CameraSearchTool) Description() string {
	return "Query accessible camera events with optional text, time range, granularity, service, and limit filters. If query is empty, returns the most recent matching events."
}

// Parameters returns the JSON schema for tool parameters
func (t *CameraSearchTool) Parameters() map[string]any {
	return map[string]any{
		"query": map[string]any{
			"type":        "string",
			"maxLength":   maxCameraSearchQueryLength,
			"description": "Optional text query. Matches event descriptions and detected object labels. Leave empty to list all accessible events matching the other filters.",
		},
		"from_iso": map[string]any{
			"type":        "string",
			"description": "Optional inclusive lower time bound. Prefer RFC3339. YYYY-MM-DD is also accepted and interpreted in server local time.",
		},
		"to_iso": map[string]any{
			"type":        "string",
			"description": "Optional exclusive upper time bound. Prefer RFC3339. YYYY-MM-DD is also accepted and interpreted in server local time.",
		},
		"granularity": map[string]any{
			"type":        "string",
			"enum":        []string{"second", "minute", "hour", "day", "week", "month"},
			"description": "Optional granularity filter for event spans.",
		},
		"service_id": map[string]any{
			"type":        "string",
			"description": "Optional service ID to restrict results to a specific camera/service.",
		},
		"limit": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     maxCameraSearchLimit,
			"description": fmt.Sprintf("Optional max number of events to return. Defaults to %d and is capped at %d.", defaultCameraSearchLimit, maxCameraSearchLimit),
		},
	}
}

// Execute executes the tool with the given arguments
func (t *CameraSearchTool) Execute(ctx context.Context, argumentsJSON string) string {
	var args struct {
		Query       string   `json:"query"`
		Keywords    []string `json:"keywords"`
		FromISO     string   `json:"from_iso"`
		ToISO       string   `json:"to_iso"`
		Granularity string   `json:"granularity"`
		ServiceID   string   `json:"service_id"`
		Limit       int      `json:"limit"`
	}

	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), fmt.Sprintf(`{"error": "invalid arguments: %v"}`, err))
	}

	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), `{"error": "not authenticated"}`)
	}

	searchTerms, err := buildCameraSearchTerms(args.Query, args.Keywords)
	if err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), fmt.Sprintf(`{"error": %q}`, err.Error()))
	}

	fromTime, err := parseCameraEventTime(args.FromISO)
	if err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), fmt.Sprintf(`{"error": %q}`, err.Error()))
	}

	toTime, err := parseCameraEventTime(args.ToISO)
	if err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), fmt.Sprintf(`{"error": %q}`, err.Error()))
	}

	if fromTime != nil && toTime != nil && !fromTime.Before(*toTime) {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), `{"error": "from_iso must be before to_iso"}`)
	}

	granularity, err := normalizeCameraGranularity(args.Granularity)
	if err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), fmt.Sprintf(`{"error": %q}`, err.Error()))
	}

	limit, err := normalizeCameraLimit(args.Limit)
	if err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), fmt.Sprintf(`{"error": %q}`, err.Error()))
	}

	events, err := t.db.QueryCameraEventsForUser(userID, database.CameraEventQuery{
		SearchTerms: searchTerms,
		From:        fromTime,
		To:          toTime,
		Granularity: granularity,
		ServiceID:   strings.TrimSpace(args.ServiceID),
		Limit:       limit,
	})
	if err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), fmt.Sprintf(`{"error": "search failed: %v"}`, err))
	}

	if len(events) == 0 {
		return fmt.Sprintf("tool %s returned: %s", t.Id(), `{"result": "There is no such camera event in the database."}`)
	}

	// Convert events to JSON response
	type eventResult struct {
		ID          string         `json:"id"`
		ServiceName string         `json:"service_name,omitempty"`
		ServiceID   string         `json:"service_id"`
		FromISO     string         `json:"from_iso,omitempty"`
		ToISO       string         `json:"to_iso,omitempty"`
		Granularity string         `json:"granularity,omitempty"`
		Payload     map[string]any `json:"payload"`
		CreatedAt   string         `json:"created_at"`
	}

	serviceNames := make(map[string]string, len(events))
	results := make([]eventResult, len(events))
	for i, e := range events {
		payload := e.Payload.AsMap()
		serviceName := serviceNames[e.ServiceId]
		if serviceName == "" {
			svc, err := t.db.GetService(e.ServiceId)
			if err == nil && svc != nil {
				serviceName = strings.TrimSpace(svc.Name)
				serviceNames[e.ServiceId] = serviceName
			}
		}

		results[i] = eventResult{
			ID:          e.Id,
			ServiceName: serviceName,
			ServiceID:   e.ServiceId,
			FromISO:     stringValueFromMap(payload, "from_iso"),
			ToISO:       stringValueFromMap(payload, "to_iso"),
			Granularity: stringValueFromMap(payload, "granularity"),
			Payload:     payload,
			CreatedAt:   e.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
		}
	}

	responseJSON, _ := json.Marshal(map[string]any{
		"result": fmt.Sprintf("Found %d matching camera event(s)", len(events)),
		"events": results,
		"count":  len(events),
	})

	return fmt.Sprintf("tool %s returned: %s", t.Id(), string(responseJSON))
}

// DisplayMessage returns a human-friendly message describing what the tool is doing
func (t *CameraSearchTool) DisplayMessage(argumentsJSON string) string {
	var args struct {
		Query       string   `json:"query"`
		Keywords    []string `json:"keywords"`
		FromISO     string   `json:"from_iso"`
		ToISO       string   `json:"to_iso"`
		Granularity string   `json:"granularity"`
		ServiceID   string   `json:"service_id"`
	}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return "Querying camera events"
	}

	searchTerms, _ := buildCameraSearchTerms(args.Query, args.Keywords)
	queryText := ""
	if len(searchTerms) > 0 {
		queryText = searchTerms[0]
	}

	fromText := formatCameraEventTimeForDisplay(args.FromISO)
	toText := formatCameraEventTimeForDisplay(args.ToISO)
	granularity, _ := normalizeCameraGranularity(args.Granularity)
	hasServiceFilter := strings.TrimSpace(args.ServiceID) != ""

	base := "Listing camera events"
	if queryText != "" {
		base = fmt.Sprintf("Searching camera events for %q", queryText)
	}

	switch {
	case fromText != "" && toText != "":
		base += fmt.Sprintf(" from %s to %s", fromText, toText)
	case fromText != "":
		base += fmt.Sprintf(" since %s", fromText)
	case toText != "":
		base += fmt.Sprintf(" before %s", toText)
	}

	if granularity != "" {
		base += fmt.Sprintf(" (%s granularity)", granularity)
	}
	if hasServiceFilter {
		base += " for one camera"
	}

	return base
}

func buildCameraSearchTerms(query string, keywords []string) ([]string, error) {
	terms := make([]string, 0, maxCameraSearchTerms)
	seen := make(map[string]struct{}, maxCameraSearchTerms)

	addTerm := func(term string, maxLength int, label string) error {
		term = normalizeCameraSearchText(term)
		if term == "" {
			return nil
		}
		if len(term) > maxLength {
			return fmt.Errorf("%s %q is too long (max %d characters)", label, term, maxLength)
		}
		if _, exists := seen[term]; exists {
			return nil
		}
		if len(terms) >= maxCameraSearchTerms {
			return fmt.Errorf("too many search terms (max %d)", maxCameraSearchTerms)
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		return nil
	}

	query = normalizeCameraSearchText(query)
	if query != "" {
		if err := addTerm(query, maxCameraSearchQueryLength, "query"); err != nil {
			return nil, err
		}
		for _, token := range strings.Fields(query) {
			if len(token) < 2 {
				continue
			}
			if err := addTerm(token, maxCameraSearchKeywordLength, "query term"); err != nil {
				return nil, err
			}
		}
	}

	for _, kw := range keywords {
		if err := addTerm(kw, maxCameraSearchKeywordLength, "keyword"); err != nil {
			return nil, err
		}
	}

	return terms, nil
}

func normalizeCameraSearchText(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}

	var b strings.Builder
	lastWasSpace := true
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastWasSpace = false
			continue
		}
		if !lastWasSpace {
			b.WriteByte(' ')
			lastWasSpace = true
		}
	}

	return strings.TrimSpace(b.String())
}

func parseCameraEventTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parseInLocal := func(layout string) (*time.Time, error) {
		t, err := time.ParseInLocation(layout, raw, time.Local)
		if err != nil {
			return nil, err
		}
		return &t, nil
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, nil
	}
	if t, err := parseInLocal("2006-01-02T15:04:05"); err == nil {
		return t, nil
	}
	if t, err := parseInLocal("2006-01-02T15:04"); err == nil {
		return t, nil
	}
	if t, err := parseInLocal("2006-01-02 15:04:05"); err == nil {
		return t, nil
	}
	if t, err := parseInLocal("2006-01-02"); err == nil {
		return t, nil
	}

	return nil, fmt.Errorf("invalid time %q; use RFC3339 or YYYY-MM-DD", raw)
}

func normalizeCameraGranularity(raw string) (string, error) {
	granularity := strings.ToLower(strings.TrimSpace(raw))
	switch granularity {
	case "":
		return "", nil
	case "second", "minute", "hour", "day", "week", "month":
		return granularity, nil
	default:
		return "", fmt.Errorf("invalid granularity %q", raw)
	}
}

func normalizeCameraLimit(limit int) (int, error) {
	switch {
	case limit == 0:
		return defaultCameraSearchLimit, nil
	case limit < 0:
		return 0, fmt.Errorf("limit must be positive")
	case limit > maxCameraSearchLimit:
		return 0, fmt.Errorf("limit must be at most %d", maxCameraSearchLimit)
	default:
		return limit, nil
	}
}

func stringValueFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}

func formatCameraEventTimeForDisplay(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if t, err := parseCameraEventTime(raw); err == nil && t != nil {
		if len(raw) <= len("2006-01-02") {
			return t.Format("2006-01-02")
		}
		return t.Format("2006-01-02 15:04")
	}

	return raw
}
