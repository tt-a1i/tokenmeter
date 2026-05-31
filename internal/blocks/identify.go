package blocks

import (
	"time"

	"github.com/tt-a1i/tokenmeter/internal/storage"
)

// Identify partitions entries into 5h session blocks (or any duration via
// sessionDuration). Entries must already be sorted ascending by timestamp;
// the function does NOT sort. Gap blocks are inserted between any two
// consecutive entries whose timestamps differ by more than sessionDuration.
// The `now` parameter controls active-block detection — pass time.Now() in
// production, an explicit fixed time in tests.
func Identify(entries []storage.TokenUsageEntry, sessionDuration time.Duration, now time.Time) []SessionBlock {
	if len(entries) == 0 {
		return nil
	}
	var (
		out    []SessionBlock
		cur    []storage.TokenUsageEntry
		start  time.Time
		hasCur bool
	)
	flush := func() {
		if !hasCur || len(cur) == 0 {
			return
		}
		out = append(out, buildBlock(start, cur, now, sessionDuration))
		cur = cur[:0]
		hasCur = false
	}
	for _, e := range entries {
		if hasCur {
			lastTS := cur[len(cur)-1].Timestamp
			if e.Timestamp.Sub(start) > sessionDuration || e.Timestamp.Sub(lastTS) > sessionDuration {
				flush()
				if e.Timestamp.Sub(lastTS) > sessionDuration {
					out = append(out, gapBlock(lastTS, e.Timestamp, sessionDuration))
				}
			}
		}
		if !hasCur {
			start = floorToHour(e.Timestamp)
			hasCur = true
		}
		cur = append(cur, e)
	}
	flush()
	return out
}

func floorToHour(ts time.Time) time.Time {
	return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, ts.Location())
}

func buildBlock(start time.Time, entries []storage.TokenUsageEntry, now time.Time, dur time.Duration) SessionBlock {
	end := start.Add(dur)
	first := entries[0].Timestamp
	last := entries[len(entries)-1].Timestamp
	models := uniqueModels(entries)
	tokens := sumTokens(entries)
	cost := sumCost(entries)
	active := now.Before(end) && now.Sub(last) < dur
	return SessionBlock{
		StartTime:  start,
		EndTime:    end,
		FirstEntry: &first,
		ActualEnd:  &last,
		IsActive:   active,
		Tokens:     tokens,
		Cost:       cost,
		Models:     models,
		EntryCount: len(entries),
	}
}

func gapBlock(after, before time.Time, dur time.Duration) SessionBlock {
	return SessionBlock{
		StartTime: after.Add(dur),
		EndTime:   before,
		IsGap:     true,
	}
}

func sumTokens(entries []storage.TokenUsageEntry) TokenCounts {
	var t TokenCounts
	for _, e := range entries {
		t.Input += e.InputTokens
		t.Output += e.OutputTokens
		t.CacheCreate += e.CacheCreationInputTokens
		t.CacheRead += e.CacheReadInputTokens
	}
	return t
}

func sumCost(entries []storage.TokenUsageEntry) float64 {
	var c float64
	for _, e := range entries {
		c += e.CostUSD
	}
	return c
}

func uniqueModels(entries []storage.TokenUsageEntry) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, e := range entries {
		if e.Model == "" {
			continue
		}
		if _, ok := seen[e.Model]; ok {
			continue
		}
		seen[e.Model] = struct{}{}
		out = append(out, e.Model)
	}
	return out
}
