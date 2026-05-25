package storage

import (
	"sort"
	"time"
)

type CostSpike struct {
	Date         string  `json:"date"`
	TodayCost    float64 `json:"today_cost"`
	BaselineCost float64 `json:"baseline_cost"`
	Ratio        float64 `json:"ratio"`
	LookbackDays int     `json:"lookback_days"`
}

type ToolRegression struct {
	Tool                string  `json:"tool"`
	RecentCalls         int64   `json:"recent_calls"`
	RecentFailures      int64   `json:"recent_failures"`
	RecentFailureRate   float64 `json:"recent_failure_rate"`
	BaselineCalls       int64   `json:"baseline_calls"`
	BaselineFailures    int64   `json:"baseline_failures"`
	BaselineFailureRate float64 `json:"baseline_failure_rate"`
	Ratio               float64 `json:"ratio"`
}

func DailyCostSpike(db *DB, today time.Time, lookbackDays int, ratio float64) (*CostSpike, error) {
	if db == nil || lookbackDays <= 0 || ratio <= 0 {
		return nil, nil
	}
	todayStart := dayStartUTC(today)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	todayCost, err := costBetween(db, todayStart, tomorrowStart)
	if err != nil {
		return nil, err
	}

	historyStart := todayStart.AddDate(0, 0, -lookbackDays)
	rows, err := db.db.Query(`
		SELECT strftime('%Y-%m-%d', timestamp) AS day, SUM(cost_usd)
		FROM token_usage
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY day
	`, formatQueryTime(historyStart), formatQueryTime(todayStart))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days int
	var total float64
	for rows.Next() {
		var day string
		var cost float64
		if err := rows.Scan(&day, &cost); err != nil {
			return nil, err
		}
		days++
		total += cost
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if days < lookbackDays || total <= 0 {
		return nil, nil
	}
	baseline := total / float64(days)
	if baseline <= 0 {
		return nil, nil
	}
	actualRatio := todayCost / baseline
	if actualRatio < ratio {
		return nil, nil
	}
	return &CostSpike{
		Date:         todayStart.Format("2006-01-02"),
		TodayCost:    todayCost,
		BaselineCost: baseline,
		Ratio:        actualRatio,
		LookbackDays: lookbackDays,
	}, nil
}

func HourlyToolFailureRegression(db *DB, now time.Time, lookbackDays, minFailures int, ratio float64) ([]ToolRegression, error) {
	if db == nil || lookbackDays <= 0 || minFailures <= 0 || ratio <= 0 {
		return nil, nil
	}
	recentStart := now.UTC().Add(-time.Hour)
	recent, err := db.TopFailingTools(recentStart, now.UTC(), 1000)
	if err != nil {
		return nil, err
	}
	if len(recent) == 0 {
		return nil, nil
	}
	baselineStart := now.UTC().AddDate(0, 0, -lookbackDays)
	baseline, err := db.TopFailingTools(baselineStart, recentStart, 1000)
	if err != nil {
		return nil, err
	}
	baselineByTool := make(map[string]ToolErrorStats, len(baseline))
	for _, stat := range baseline {
		baselineByTool[stat.Tool] = stat
	}

	out := make([]ToolRegression, 0)
	for _, stat := range recent {
		if stat.Failures < int64(minFailures) {
			continue
		}
		base, ok := baselineByTool[stat.Tool]
		if !ok || base.Total == 0 || base.FailureRate <= 0 {
			continue
		}
		actualRatio := stat.FailureRate / base.FailureRate
		if actualRatio < ratio {
			continue
		}
		out = append(out, ToolRegression{
			Tool:                stat.Tool,
			RecentCalls:         stat.Total,
			RecentFailures:      stat.Failures,
			RecentFailureRate:   stat.FailureRate,
			BaselineCalls:       base.Total,
			BaselineFailures:    base.Failures,
			BaselineFailureRate: base.FailureRate,
			Ratio:               actualRatio,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ratio == out[j].Ratio {
			if out[i].RecentFailures == out[j].RecentFailures {
				return out[i].Tool < out[j].Tool
			}
			return out[i].RecentFailures > out[j].RecentFailures
		}
		return out[i].Ratio > out[j].Ratio
	})
	return out, nil
}

func costBetween(db *DB, from, to time.Time) (float64, error) {
	var cost float64
	err := db.db.QueryRow(`
		SELECT COALESCE(SUM(cost_usd), 0)
		FROM token_usage
		WHERE timestamp >= ? AND timestamp < ?
	`, formatQueryTime(from), formatQueryTime(to)).Scan(&cost)
	return cost, err
}
