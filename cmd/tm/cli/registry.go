package cli

import (
	"github.com/tt-a1i/tokenmeter/internal/collector"
)

// AllAdapters is the canonical registry consumed by RunAggregateAllSource.
// Phase B adapter task implementations populate this map as they land:
//
//	AllAdapters["amp"] = collector.LoadAmpEntries
//	AllAdapters["opencode"] = collector.LoadOpencodeEntries
//	...
//
// An empty map degrades RunAggregateAllSource gracefully to SQLite-only
// output, matching the v1.0.x behavior users see today.
var AllAdapters = map[string]AdapterLoadFn{
	"openclaw": collector.LoadOpenClawEntries,
}
