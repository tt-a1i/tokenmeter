package cli_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

type stubStatuslineLoader struct{ active *blocks.SessionBlock }

func (s stubStatuslineLoader) LoadActive(_ context.Context) (*blocks.SessionBlock, error) {
	return s.active, nil
}

func TestRunStatuslineSmoke(t *testing.T) {
	var out bytes.Buffer
	loader := stubStatuslineLoader{active: nil}
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-sonnet-4-6","session_id":"s","cwd":"/x","transcript_path":""}`),
		&out, loader, "/tmp/no-such.json", time.Now())
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("must produce output")
	}
}
