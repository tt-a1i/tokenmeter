package collector

import "time"

// UsageEntry is the unified token-usage record produced by every adapter.
// Mirrors storage.TokenUsageEntry but lives in collector to avoid circular
// imports between adapters and the storage layer.
type UsageEntry struct {
	Source                   string // "amp", "opencode", "claude", etc.
	SessionID                string
	ProjectPath              string // best-effort; empty when adapter cannot extract
	Timestamp                time.Time
	Model                    string
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	CostUSD                  float64
}

// AdapterOpts is the standard input to every Load<Name>Entries function.
type AdapterOpts struct {
	Since    time.Time // zero = no lower bound
	Until    time.Time // zero = no upper bound
	Timezone string    // IANA; empty = system local
	Project  string    // empty = no filter; most adapters ignore this
}
