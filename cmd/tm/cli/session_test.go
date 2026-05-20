package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

func TestRunSessionListMode(t *testing.T) {
	var out bytes.Buffer
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SourceID: "1", SessionID: "abc", Model: "sonnet", InputTokens: 10, OutputTokens: 5, CostUSD: 0.001, Timestamp: time.Now()},
	}}
	if err := cli.RunSession(context.Background(), &out, cli.SessionArgs{Shared: cli.Shared{}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if !strings.Contains(out.String(), "SESSION") {
		t.Fatalf("expected SESSION header in output: %q", out.String())
	}
}
