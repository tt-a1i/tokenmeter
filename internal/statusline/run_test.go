package statusline_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/statusline"
)

type stubReader struct{ active *blocks.SessionBlock }

func (s stubReader) LoadActive(_ context.Context) (*blocks.SessionBlock, error) {
	return s.active, nil
}

func TestRunNoActiveBlockStillProducesOutput(t *testing.T) {
	in := bytes.NewBufferString(`{"model_id":"claude-sonnet-4-6","session_id":"s","cwd":"/x","transcript_path":""}`)
	var out bytes.Buffer
	err := statusline.Run(context.Background(), in, &out, stubReader{active: nil}, statusline.Config{}, time.Now())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "no active block") {
		t.Fatalf("expected fallback message, got %q", out.String())
	}
}
