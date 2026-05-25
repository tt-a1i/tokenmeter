package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Query struct {
	Keywords  []string   `json:"keywords,omitempty"`
	Tool      string     `json:"tool,omitempty"`
	Session   string     `json:"session,omitempty"`
	Status    string     `json:"status,omitempty"`
	Platform  string     `json:"platform,omitempty"`
	CostMin   *float64   `json:"cost_min,omitempty"`
	CostMax   *float64   `json:"cost_max,omitempty"`
	TokensMin *int64     `json:"tokens_min,omitempty"`
	TokensMax *int64     `json:"tokens_max,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
}

func ParseQuery(input string) (Query, error) {
	var q Query
	for _, field := range strings.Fields(input) {
		key, value, ok := strings.Cut(field, ":")
		if !ok {
			q.Keywords = append(q.Keywords, field)
			continue
		}
		if value == "" {
			return q, fmt.Errorf("empty value for filter %q", key)
		}
		switch key {
		case "tool":
			q.Tool = value
		case "session":
			q.Session = value
		case "status":
			switch value {
			case "ok", "failed", "all":
				q.Status = value
			default:
				return q, fmt.Errorf("invalid status %q (use ok, failed, all)", value)
			}
		case "platform":
			switch value {
			case "claude", "codex", "all":
				q.Platform = value
			default:
				return q, fmt.Errorf("invalid platform %q (use claude, codex, all)", value)
			}
		case "cost":
			if err := parseFloatRange(value, &q.CostMin, &q.CostMax); err != nil {
				return q, fmt.Errorf("invalid cost filter %q: %w", value, err)
			}
		case "tokens":
			if err := parseIntRange(value, &q.TokensMin, &q.TokensMax); err != nil {
				return q, fmt.Errorf("invalid tokens filter %q: %w", value, err)
			}
		case "since":
			t, err := parseSearchDate(value)
			if err != nil {
				return q, fmt.Errorf("invalid since date %q (want YYYY-MM-DD): %w", value, err)
			}
			q.Since = &t
		case "until":
			t, err := parseSearchDate(value)
			if err != nil {
				return q, fmt.Errorf("invalid until date %q (want YYYY-MM-DD): %w", value, err)
			}
			q.Until = &t
		default:
			return q, fmt.Errorf("unknown filter %q — supported: tool, session, status, platform, cost, tokens, since, until", key+":")
		}
	}
	return q, nil
}

func (q Query) String() string {
	parts := append([]string(nil), q.Keywords...)
	if q.Tool != "" {
		parts = append(parts, "tool:"+q.Tool)
	}
	if q.Session != "" {
		parts = append(parts, "session:"+q.Session)
	}
	if q.Status != "" {
		parts = append(parts, "status:"+q.Status)
	}
	if q.Platform != "" {
		parts = append(parts, "platform:"+q.Platform)
	}
	if q.CostMin != nil {
		parts = append(parts, "cost:>"+formatSearchFloat(*q.CostMin))
	}
	if q.CostMax != nil {
		parts = append(parts, "cost:<"+formatSearchFloat(*q.CostMax))
	}
	if q.TokensMin != nil {
		parts = append(parts, fmt.Sprintf("tokens:>%d", *q.TokensMin))
	}
	if q.TokensMax != nil {
		parts = append(parts, fmt.Sprintf("tokens:<%d", *q.TokensMax))
	}
	if q.Since != nil {
		parts = append(parts, "since:"+q.Since.Format("2006-01-02"))
	}
	if q.Until != nil {
		parts = append(parts, "until:"+q.Until.Format("2006-01-02"))
	}
	return strings.Join(parts, " ")
}

func (q Query) hasAdvancedFilters() bool {
	return q.Tool != "" || q.Session != "" || q.Status != "" || q.Platform != "" ||
		q.CostMin != nil || q.CostMax != nil || q.TokensMin != nil || q.TokensMax != nil ||
		q.Since != nil || q.Until != nil
}

func parseFloatRange(raw string, min, max **float64) error {
	op, value := splitRangeFilter(raw)
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	switch op {
	case "<":
		*max = &n
	default:
		*min = &n
	}
	return nil
}

func parseIntRange(raw string, min, max **int64) error {
	op, value := splitRangeFilter(raw)
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	switch op {
	case "<":
		*max = &n
	default:
		*min = &n
	}
	return nil
}

func splitRangeFilter(raw string) (string, string) {
	if strings.HasPrefix(raw, ">") || strings.HasPrefix(raw, "<") {
		return raw[:1], raw[1:]
	}
	return ">", raw
}

func parseSearchDate(raw string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", raw, time.Local)
}

func formatSearchFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
