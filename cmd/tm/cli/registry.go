package cli

// AllAdapters is the canonical registry consumed by RunAggregateAllSource.
// Phase B adapter task implementations populate this map as they land:
//
//	AllAdapters["amp"] = collector.LoadAmpEntries
//	AllAdapters["opencode"] = collector.LoadOpencodeEntries
//	...
//
// The empty map is intentional during Phase A — RunAggregateAllSource
// degrades gracefully to SQLite-only output when no adapters are
// registered, which is the v1.0.x behavior users see today.
var AllAdapters = map[string]AdapterLoadFn{}
