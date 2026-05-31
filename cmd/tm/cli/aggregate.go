package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/collector"
	"github.com/tt-a1i/tokenmeter/internal/pricing"
	"github.com/tt-a1i/tokenmeter/internal/projectalias"
	"github.com/tt-a1i/tokenmeter/internal/render"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

type Bucket int

const (
	BucketDaily Bucket = iota
	BucketWeekly
	BucketMonthly
	// BucketSession buckets rows by session_id; primarily used by
	// RunSession but also reachable through the adapter router when an
	// adapter source is invoked with `session` as its sub-command.
	BucketSession
)

type AggregateArgs struct {
	Shared Shared
	Bucket Bucket
	// Platform restricts SQLite-backed source commands (claude/codex).
	// Empty means all local platforms.
	Platform string
}

// AggregateLoader is the read-side interface RunSession / RunDeprecatedAlias
// continue to take. RunAggregate needs more — see AggregateUsageLoader.
type AggregateLoader interface {
	ListUsageForBlocksFiltered(ctx context.Context, since, until time.Time, workspace string) ([]storage.TokenUsageEntry, error)
}

type platformUsageLoader interface {
	ListUsageForBlocksFilteredByPlatform(ctx context.Context, since, until time.Time, workspace, platform string) ([]storage.TokenUsageEntry, error)
}

// AggregateUsageLoader is what RunAggregate actually requires post-push-down.
// *storage.DB satisfies both halves of the interface; the cli layer
// type-asserts at runtime so callers (aliases.go, session.go) that still
// hand in a bare AggregateLoader don't need their signatures rewritten in
// the same commit.
type AggregateUsageLoader interface {
	AggregateLoader
	AggregateUsage(ctx context.Context, f storage.AggregateFilter) ([]storage.AggregateUsageRow, error)
}

// pricingMap is loaded once per process; LoadEmbedded panics on malformed
// snapshot, which the test suite would surface immediately.
var pricingMap = pricing.LoadEmbedded()

func RunAggregate(ctx context.Context, w io.Writer, a AggregateArgs, loader AggregateLoader) error {
	ul, ok := loader.(AggregateUsageLoader)
	if !ok {
		return fmt.Errorf("aggregate loader %T does not implement AggregateUsage; rebuild against storage v1.0.2", loader)
	}

	since, err := parseDateFlag(a.Shared.Since)
	if err != nil {
		return err
	}
	until, err := ParseDateFlagUntil(a.Shared.Until)
	if err != nil {
		return err
	}

	// Note: timezone offset uses the offset at query time, which may
	// mis-bucket historical data that crossed a DST boundary. Known
	// limitation; acceptable for the typical ccusage workflow.
	loc := time.UTC
	if a.Shared.Timezone != "" {
		parsed, err := time.LoadLocation(a.Shared.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", a.Shared.Timezone, err)
		}
		loc = parsed
	}

	mode := pricing.ParseMode(a.Shared.Mode)
	if a.Shared.Instances || a.Shared.ProjectAliases != "" {
		entries, err := listUsageForBlocks(ctx, loader, since, until, a.Shared.Project, a.Platform)
		if err != nil {
			return err
		}
		entries = applyPricingMode(entries, mode)
		aliases, err := projectalias.Load(a.Shared.ProjectAliases)
		if err != nil {
			return err
		}
		rows := aggregateEntriesByProject(entries, a.Bucket, loc, aliases, weekStartFromShared(a.Shared))
		if a.Shared.Order == "desc" {
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Bucket == rows[j].Bucket {
					return rows[i].Project > rows[j].Project
				}
				return rows[i].Bucket > rows[j].Bucket
			})
		}
		return render.New().RenderAggregate(w, bucketKind(a.Bucket), rows, renderOpts(a.Shared, w))
	}
	// When the user asked for ModeAuto without --breakdown, we still need
	// per-model rows from storage so each (bucket, model) row can be
	// inspected for the zero-cost fallback. Without this, a mixed-model
	// bucket whose SUM > 0 (Claude $X + Codex $0 → SUM $X) silently drops
	// the Codex share. We then fold the per-model rows back into one row
	// per bucket so the user-facing view stays unchanged.
	needAutoFallback := mode == pricing.ModeAuto && !a.Shared.Breakdown

	filter := storage.AggregateFilter{
		Since:     since,
		Until:     until,
		Project:   a.Shared.Project,
		Platform:  a.Platform,
		Bucket:    cliBucketToStorage(a.Bucket),
		Breakdown: a.Shared.Breakdown || needAutoFallback,
		Location:  loc,
		WeekStart: weekStartFromShared(a.Shared),
	}
	aggRows, err := ul.AggregateUsage(ctx, filter)
	if err != nil {
		return err
	}

	rows := convertAggregateRows(aggRows, a.Shared.Breakdown, mode, needAutoFallback)

	if a.Shared.Order == "desc" {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Bucket > rows[j].Bucket })
	}

	return render.New().RenderAggregate(w, bucketKind(a.Bucket), rows, renderOpts(a.Shared, w))
}

func aggregateEntriesByProject(entries []storage.TokenUsageEntry, bucket Bucket, loc *time.Location, aliases projectalias.Aliases, weekStart time.Weekday) []render.AggregateRow {
	type key struct {
		bucket  string
		project string
	}
	byKey := map[key]*render.AggregateRow{}
	order := []key{}
	for _, e := range entries {
		project := resolveProjectName(aliases, e.CWD)
		k := key{bucket: entryBucket(e.Timestamp, bucket, loc, weekStart), project: project}
		row, ok := byKey[k]
		if !ok {
			row = &render.AggregateRow{Bucket: k.bucket, Project: project}
			byKey[k] = row
			order = append(order, k)
		}
		row.Models = appendUnique(row.Models, e.Model)
		row.InputTokens += e.InputTokens
		row.OutputTokens += e.OutputTokens
		row.CacheCreateTokens += e.CacheCreationInputTokens
		row.CacheReadTokens += e.CacheReadInputTokens
		row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreateTokens + row.CacheReadTokens
		row.Cost += e.CostUSD
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].bucket == order[j].bucket {
			return order[i].project < order[j].project
		}
		return order[i].bucket < order[j].bucket
	})
	out := make([]render.AggregateRow, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

func resolveProjectName(aliases projectalias.Aliases, cwd string) string {
	if aliases != nil {
		return aliases.Resolve(cwd)
	}
	return projectalias.Aliases{}.Resolve(cwd)
}

func entryBucket(ts time.Time, bucket Bucket, loc *time.Location, weekStart time.Weekday) string {
	if loc == nil {
		loc = time.UTC
	}
	t := ts.In(loc)
	switch bucket {
	case BucketWeekly:
		return weekStartKey(t, weekStart)
	case BucketMonthly:
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

// cliBucketToStorage maps the cli Bucket enum to its storage counterpart.
func cliBucketToStorage(b Bucket) storage.AggregateBucket {
	switch b {
	case BucketWeekly:
		return storage.BucketWeek
	case BucketMonthly:
		return storage.BucketMonth
	case BucketSession:
		return storage.BucketSession
	default:
		return storage.BucketDay
	}
}

// bucketKind maps the cli Bucket enum to the render package's string kind.
func bucketKind(b Bucket) string {
	switch b {
	case BucketWeekly:
		return "weekly"
	case BucketMonthly:
		return "monthly"
	case BucketSession:
		return "session"
	default:
		return "daily"
	}
}

// convertAggregateRows folds storage rows (which carry one row per
// (bucket, model) when Breakdown=true) into render rows (one row per bucket,
// with Breakdown[] populated when requested). Per-row cost honors --mode:
// ModeCalculate always recomputes; ModeAuto recomputes only when the source
// bucket cost is zero (matching v1.0.1 entry-level applyPricingMode semantics
// — Codex zero-cost rows still surface a value); ModeDisplay leaves cost as
// SUM-ed by storage.
//
// foldForAuto handles the ModeAuto-without-breakdown case: storage is asked
// for per-(bucket, model) rows so each row can independently trigger the
// zero-cost fallback, but the output collapses to one render row per bucket
// (no user-visible Breakdown[]) so the default daily view stays identical.
func convertAggregateRows(in []storage.AggregateUsageRow, breakdown bool, mode pricing.Mode, foldForAuto bool) []render.AggregateRow {
	if foldForAuto {
		byBucket := map[string]*render.AggregateRow{}
		order := []string{}
		for _, r := range in {
			row, ok := byBucket[r.Bucket]
			if !ok {
				row = &render.AggregateRow{Bucket: r.Bucket}
				byBucket[r.Bucket] = row
				order = append(order, r.Bucket)
			}
			cost := r.Cost
			if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
				cost = recalculateCost(r)
			}
			row.Models = appendUnique(row.Models, r.Model)
			row.InputTokens += r.InputTokens
			row.OutputTokens += r.OutputTokens
			row.CacheCreateTokens += r.CacheCreateTokens
			row.CacheReadTokens += r.CacheReadTokens
			row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreateTokens + row.CacheReadTokens
			row.Cost += cost
			// Deliberately NOT populating row.Breakdown — user didn't ask
			// for --breakdown; the per-model rows were a means to the end
			// of correct cost.
		}
		out := make([]render.AggregateRow, 0, len(order))
		for _, k := range order {
			out = append(out, *byBucket[k])
		}
		return out
	}
	if !breakdown {
		out := make([]render.AggregateRow, 0, len(in))
		for _, r := range in {
			cost := r.Cost
			if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
				cost = recalculateCost(r)
			}
			out = append(out, render.AggregateRow{
				Bucket:            r.Bucket,
				Models:            r.Models,
				InputTokens:       r.InputTokens,
				OutputTokens:      r.OutputTokens,
				CacheCreateTokens: r.CacheCreateTokens,
				CacheReadTokens:   r.CacheReadTokens,
				TotalTokens:       r.InputTokens + r.OutputTokens + r.CacheCreateTokens + r.CacheReadTokens,
				Cost:              cost,
			})
		}
		return out
	}
	// Breakdown=true: storage emits one row per (bucket, model). Fold by
	// bucket so the render layer sees one AggregateRow per bucket with the
	// per-model rows in Breakdown[].
	byBucket := map[string]*render.AggregateRow{}
	order := []string{}
	for _, r := range in {
		row, ok := byBucket[r.Bucket]
		if !ok {
			row = &render.AggregateRow{Bucket: r.Bucket}
			byBucket[r.Bucket] = row
			order = append(order, r.Bucket)
		}
		cost := r.Cost
		if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
			cost = recalculateCost(r)
		}
		row.Models = appendUnique(row.Models, r.Model)
		row.InputTokens += r.InputTokens
		row.OutputTokens += r.OutputTokens
		row.CacheCreateTokens += r.CacheCreateTokens
		row.CacheReadTokens += r.CacheReadTokens
		row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreateTokens + row.CacheReadTokens
		row.Cost += cost
		row.Breakdown = append(row.Breakdown, render.ModelBreakdown{
			Model:             r.Model,
			InputTokens:       r.InputTokens,
			OutputTokens:      r.OutputTokens,
			CacheCreateTokens: r.CacheCreateTokens,
			CacheReadTokens:   r.CacheReadTokens,
			TotalTokens:       r.InputTokens + r.OutputTokens + r.CacheCreateTokens + r.CacheReadTokens,
			Cost:              cost,
		})
	}
	out := make([]render.AggregateRow, 0, len(order))
	for _, k := range order {
		out = append(out, *byBucket[k])
	}
	return out
}

// appendUnique appends v to slice if not already present. Used to grow the
// per-bucket models list when folding breakdown rows.
func appendUnique(slice []string, v string) []string {
	if v == "" {
		return slice
	}
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}

// recalculateCost re-prices a single AggregateUsageRow from its token totals
// using the bucket's primary model (r.Model when Breakdown=true, else
// r.Models[0]). Mixed-model buckets in non-breakdown mode are approximated by
// the first model; users wanting exact per-entry recompute should pass
// --breakdown.
func recalculateCost(r storage.AggregateUsageRow) float64 {
	model := r.Model
	if model == "" && len(r.Models) > 0 {
		model = r.Models[0]
	}
	if model == "" {
		return r.Cost
	}
	p, ok := pricingMap.Resolve(model)
	if !ok {
		return r.Cost
	}
	return pricing.CalculateCost(p, pricing.Usage{
		Input:       r.InputTokens,
		Output:      r.OutputTokens,
		CacheCreate: r.CacheCreateTokens,
		CacheRead:   r.CacheReadTokens,
	}, speedForModel(model))
}

// parseDateFlag accepts a YYYYMMDD shared-flag value (also tolerating the
// dashed YYYY-MM-DD form, matching ccusage's normalize_date_bound in
// rust/crates/ccusage-cli/src/types.rs:64-66) and returns a UTC time.Time.
// An empty string maps to the zero time, meaning "no bound".
func parseDateFlag(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	normalized := strings.ReplaceAll(s, "-", "")
	t, err := time.Parse("20060102", normalized)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (want YYYYMMDD or YYYY-MM-DD): %w", s, err)
	}
	return t, nil
}

// ParseDateFlagUntil mirrors parseDateFlag but bumps the result forward by
// 24h so a `--until YYYYMMDD` flag becomes an inclusive closed interval to
// the end of that day, matching ccusage's filter behavior. Empty input
// passes through as the zero time (meaning "no bound"). Exported so the
// shared-flag tests can pin the +24h contract directly.
func ParseDateFlagUntil(s string) (time.Time, error) {
	t, err := parseDateFlag(s)
	if err != nil {
		return time.Time{}, err
	}
	if t.IsZero() {
		return t, nil
	}
	return t.Add(24 * time.Hour), nil
}

// RunAggregateAllSource is the default daily/weekly/monthly path when the
// user did NOT pass --no-scan. It pulls token_usage rows from SQLite
// (Claude + Codex; the v1.0 baseline data) plus every registered batch
// adapter, folds them in-memory into storage.AggregateUsageRow shape, and
// hands them to the same convertAggregateRows + render path as RunAggregate
// so the output is visually identical to --no-scan when no adapters are
// installed.
//
// Error policy:
//   - SQLite loader error is fatal (Claude/Codex is the baseline).
//   - Adapter fs.ErrNotExist (user does not have the agent installed) is
//     silently skipped.
//   - Other adapter errors emit a stderr warning and skip that source.
//
// Weekly bucket parity: BucketWeekly uses weekKeySQLiteW (a Go reimpl of
// SQLite strftime('%Y-W%W'), Monday-based with week 00 for days before
// the year's first Monday) so the in-memory path here matches the SQL
// push-down path used by RunAggregate. Both paths now produce identical
// "YYYY-Www" keys; the Phase A.5 ISO 8601-vs-%W asymmetry is resolved.
func RunAggregateAllSource(ctx context.Context, w io.Writer, a AggregateArgs, sqliteLoader AggregateLoader, adapters map[string]AdapterLoadFn) error {
	since, err := parseDateFlag(a.Shared.Since)
	if err != nil {
		return err
	}
	until, err := ParseDateFlagUntil(a.Shared.Until)
	if err != nil {
		return err
	}
	loc := time.UTC
	if a.Shared.Timezone != "" {
		parsed, err := time.LoadLocation(a.Shared.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", a.Shared.Timezone, err)
		}
		loc = parsed
	}
	mode := pricing.ParseMode(a.Shared.Mode)

	sqliteEntries, err := listUsageForBlocks(ctx, sqliteLoader, since, until, a.Shared.Project, a.Platform)
	if err != nil {
		return err
	}
	all := append([]storage.TokenUsageEntry(nil), sqliteEntries...)

	// Stable iteration over adapters so warning output and (in pathological
	// duplicate-key cases) accumulation order are deterministic.
	adapterNames := make([]string, 0, len(adapters))
	for name := range adapters {
		adapterNames = append(adapterNames, name)
	}
	sort.Strings(adapterNames)
	for _, source := range adapterNames {
		fn := adapters[source]
		entries, err := fn(ctx, collector.AdapterOpts{
			Since:    since,
			Until:    until,
			Timezone: a.Shared.Timezone,
			Project:  a.Shared.Project,
		})
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s adapter failed: %v\n", source, err)
			continue
		}
		for _, e := range entries {
			// Defensive post-filter in case the adapter ignored opts.
			// Matches SQL semantics: since is inclusive, until is the
			// +24h exclusive upper bound (ParseDateFlagUntil), so the SQL
			// `u.timestamp <= until` actually includes timestamps equal
			// to until — mirror that with !After().
			if !since.IsZero() && e.Timestamp.Before(since) {
				continue
			}
			if !until.IsZero() && e.Timestamp.After(until) {
				continue
			}
			// Project filter is intentionally NOT applied post-fn: most
			// adapters cannot fill ProjectPath (their log formats do not
			// carry workspace info), so post-filtering would silently
			// drop legitimate rows. --project remains a Claude/Codex
			// filter only.
			all = append(all, storage.TokenUsageEntry{
				SourceID:                 e.SessionID,
				SessionID:                e.SessionID,
				Timestamp:                e.Timestamp,
				Model:                    e.Model,
				InputTokens:              e.InputTokens,
				OutputTokens:             e.OutputTokens,
				CacheCreationInputTokens: e.CacheCreationInputTokens,
				CacheReadInputTokens:     e.CacheReadInputTokens,
				CostUSD:                  e.CostUSD,
			})
		}
	}

	needAutoFallback := mode == pricing.ModeAuto && !a.Shared.Breakdown
	breakdown := a.Shared.Breakdown || needAutoFallback
	aggRows := aggregateEntriesInMemory(all, a.Bucket, loc, breakdown, weekStartFromShared(a.Shared))
	rows := convertAggregateRows(aggRows, a.Shared.Breakdown, mode, needAutoFallback)
	if a.Shared.Order == "desc" {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Bucket > rows[j].Bucket })
	}
	return render.New().RenderAggregate(w, bucketKind(a.Bucket), rows, renderOpts(a.Shared, w))
}

// aggregateEntriesInMemory folds token_usage entries into AggregateUsageRow
// shape so RunAggregateAllSource can reuse convertAggregateRows + the
// boxed renderer. Mirrors storage.AggregateUsage's grouping contract:
// breakdown=true emits one row per (bucket, model); breakdown=false emits
// one row per bucket with Models[] populated.
func aggregateEntriesInMemory(entries []storage.TokenUsageEntry, bucket Bucket, loc *time.Location, breakdown bool, weekStart time.Weekday) []storage.AggregateUsageRow {
	type key struct{ bucket, model string }
	type acc struct {
		models          []string
		in, out, cc, cr int64
		cost            float64
		lastActivity    time.Time
	}
	perKey := map[key]*acc{}
	seen := map[key]bool{}
	var keyOrder []key

	bucketKey := func(t time.Time, sessionID string) string {
		tt := t.In(loc)
		switch bucket {
		case BucketWeekly:
			return weekStartKey(tt, weekStart)
		case BucketMonthly:
			return tt.Format("2006-01")
		case BucketSession:
			return sessionID
		default:
			return tt.Format("2006-01-02")
		}
	}

	for _, e := range entries {
		bk := bucketKey(e.Timestamp, e.SessionID)
		modelKey := ""
		if breakdown {
			modelKey = e.Model
		}
		k := key{bk, modelKey}
		if !seen[k] {
			seen[k] = true
			keyOrder = append(keyOrder, k)
		}
		a := perKey[k]
		if a == nil {
			a = &acc{}
			perKey[k] = a
		}
		if !breakdown {
			a.models = appendUnique(a.models, e.Model)
		}
		a.in += e.InputTokens
		a.out += e.OutputTokens
		a.cc += e.CacheCreationInputTokens
		a.cr += e.CacheReadInputTokens
		a.cost += e.CostUSD
		if e.Timestamp.After(a.lastActivity) {
			a.lastActivity = e.Timestamp
		}
	}

	// Match storage.AggregateUsage's ORDER BY: bucket ASC (then model ASC
	// when breakdown). Stable sort not required — keys are unique by
	// construction.
	sort.Slice(keyOrder, func(i, j int) bool {
		if keyOrder[i].bucket != keyOrder[j].bucket {
			return keyOrder[i].bucket < keyOrder[j].bucket
		}
		return keyOrder[i].model < keyOrder[j].model
	})

	out := make([]storage.AggregateUsageRow, 0, len(keyOrder))
	for _, k := range keyOrder {
		a := perKey[k]
		row := storage.AggregateUsageRow{
			Bucket:            k.bucket,
			InputTokens:       a.in,
			OutputTokens:      a.out,
			CacheCreateTokens: a.cc,
			CacheReadTokens:   a.cr,
			Cost:              a.cost,
		}
		if breakdown {
			row.Model = k.model
		} else {
			sort.Strings(a.models)
			row.Models = a.models
		}
		if bucket == BucketSession {
			row.LastActivity = a.lastActivity
		}
		out = append(out, row)
	}
	return out
}

func weekStartFromShared(s Shared) time.Weekday {
	day, err := parseWeekday(s.StartOfWeek)
	if err != nil {
		return time.Sunday
	}
	return day
}

func weekStartKey(t time.Time, start time.Weekday) string {
	shift := (int(t.Weekday()) - int(start) + 7) % 7
	return t.AddDate(0, 0, -shift).Format("2006-01-02")
}

// applyPricingMode rewrites each entry's CostUSD according to mode. Kept
// here for cli/blocks.go, which still drives entry-level pricing during
// its own push-down migration; once that lands, this helper becomes dead
// and can be deleted.
//
//   - ModeDisplay: leaves CostUSD untouched (trust the source row).
//   - ModeCalculate: recomputes from tokens × pricing for the resolved model.
//   - ModeAuto: when CostUSD > 0 keeps it; when it's zero, falls back to
//     calculate so missing cost columns (e.g. Codex) still surface a value.
func applyPricingMode(entries []storage.TokenUsageEntry, mode pricing.Mode) []storage.TokenUsageEntry {
	if mode == pricing.ModeDisplay {
		return entries
	}
	out := make([]storage.TokenUsageEntry, len(entries))
	for i, e := range entries {
		out[i] = e
		if mode == pricing.ModeAuto && e.CostUSD > 0 {
			continue
		}
		if e.Model == "" {
			continue
		}
		p, ok := pricingMap.Resolve(e.Model)
		if !ok {
			continue
		}
		out[i].CostUSD = pricing.CalculateCost(p, pricing.Usage{
			Input:       e.InputTokens,
			Output:      e.OutputTokens,
			CacheCreate: e.CacheCreationInputTokens,
			CacheRead:   e.CacheReadInputTokens,
		}, speedForModel(e.Model))
	}
	return out
}

func listUsageForBlocks(ctx context.Context, loader AggregateLoader, since, until time.Time, project, platform string) ([]storage.TokenUsageEntry, error) {
	if platform == "" {
		return loader.ListUsageForBlocksFiltered(ctx, since, until, project)
	}
	pl, ok := loader.(platformUsageLoader)
	if !ok {
		return nil, fmt.Errorf("usage loader %T does not support platform filter %q", loader, platform)
	}
	return pl.ListUsageForBlocksFilteredByPlatform(ctx, since, until, project, platform)
}

// weekKeySQLiteW returns a "YYYY-Www" string equivalent to SQLite's
// strftime('%Y-W%W', t). SQLite %W is Monday-based: week 00 contains
// every day from January 1 up to (but not including) the year's first
// Monday; week 01 starts on that Monday and each subsequent week is +7.
// Empirical cross-checked against modernc.org/sqlite for several
// boundary dates — see TestWeekKeySQLiteWMatchesSQLite.
//
// This helper backs RunAggregateAllSource's weekly bucketing so the
// in-memory aggregation produces the same week keys as RunAggregate's
// SQL push-down path. Without it, dates between New Year and the first
// Monday of a year (or between the last Sunday and December 31) would
// fall in different buckets across the two paths.
func weekKeySQLiteW(t time.Time) string {
	year := t.Year()
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, t.Location())
	// Go's Weekday: Sun=0, Mon=1, ..., Sat=6.
	//   jan1 weekday 1 (Mon) -> 0 days until first Monday (jan1 itself)
	//   jan1 weekday 0 (Sun) -> 1 day
	//   jan1 weekday 2 (Tue) -> 6 days
	//   ...                     ((8 - weekday) % 7) covers all cases.
	daysUntilFirstMonday := (8 - int(jan1.Weekday())) % 7
	// YearDay() is 1..366; subtract 1 so jan1 has days=0. Using YearDay
	// instead of t.Sub(jan1) avoids DST/leap-second drift from a float
	// hour division.
	days := t.YearDay() - 1
	var week int
	if days < daysUntilFirstMonday {
		week = 0
	} else {
		week = (days-daysUntilFirstMonday)/7 + 1
	}
	return fmt.Sprintf("%04d-W%02d", year, week)
}
