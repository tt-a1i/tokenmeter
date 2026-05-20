package blocks_test

import (
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

func TestSessionBlockTokenSum(t *testing.T) {
	b := blocks.SessionBlock{
		Tokens: blocks.TokenCounts{
			Input:       100,
			Output:      50,
			CacheCreate: 10,
			CacheRead:   5,
		},
	}
	if got, want := b.Tokens.Total(), int64(165); got != want {
		t.Fatalf("Total() = %d, want %d", got, want)
	}
}
