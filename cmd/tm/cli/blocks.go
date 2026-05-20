package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

// BlocksArgs is the resolved input to RunBlocks.
type BlocksArgs struct {
	Shared        Shared
	SessionLength time.Duration
	Active        bool
	Now           time.Time
}

// BlocksLoader is the storage subset RunBlocks needs. Aliased to
// AggregateLoader so `tm blocks` can honor `--since` / `--until` /
// `--project` (P1 / P2-2 from the v1.0 review). Statusline still goes
// through blocks.Reader via NewActiveBlockAdapter, which doesn't need
// workspace filtering.
type BlocksLoader = AggregateLoader

// RunBlocks lists session blocks, optionally only the active one.
func RunBlocks(ctx context.Context, out io.Writer, args BlocksArgs, loader BlocksLoader) error {
	since, err := parseDateFlag(args.Shared.Since)
	if err != nil {
		return err
	}
	until, err := parseDateFlag(args.Shared.Until)
	if err != nil {
		return err
	}
	entries, err := loader.ListUsageForBlocksFiltered(ctx, since, until, args.Shared.Project)
	if err != nil {
		return err
	}
	all := blocks.Annotate(blocks.Identify(entries, args.SessionLength, args.Now), args.Now)
	if args.Active {
		all = filterActive(all)
	}
	if args.Shared.JSON {
		return writeBlocksJSON(out, all)
	}
	return writeBlocksTable(out, all)
}

func filterActive(in []blocks.SessionBlock) []blocks.SessionBlock {
	var out []blocks.SessionBlock
	for _, b := range in {
		if b.IsActive {
			out = append(out, b)
		}
	}
	return out
}

func writeBlocksTable(w io.Writer, bs []blocks.SessionBlock) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PERIOD\tMODELS\tTOKENS\tCOST\tSTATUS")
	for _, b := range bs {
		status := "closed"
		switch {
		case b.IsActive:
			status = "ACTIVE"
		case b.IsGap:
			status = "gap"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t$%.2f\t%s\n",
			b.StartTime.Format("2006-01-02 15:04"),
			joinModels(b.Models),
			fmtInt(b.Tokens.Total()),
			b.Cost,
			status,
		)
	}
	return tw.Flush()
}

func writeBlocksJSON(w io.Writer, bs []blocks.SessionBlock) error {
	type out struct {
		Start    time.Time `json:"start"`
		End      time.Time `json:"end"`
		IsActive bool      `json:"is_active"`
		IsGap    bool      `json:"is_gap"`
		Tokens   int64     `json:"tokens"`
		Cost     float64   `json:"cost"`
		Models   []string  `json:"models"`
	}
	arr := make([]out, 0, len(bs))
	for _, b := range bs {
		arr = append(arr, out{
			Start: b.StartTime, End: b.EndTime, IsActive: b.IsActive, IsGap: b.IsGap,
			Tokens: b.Tokens.Total(), Cost: b.Cost, Models: b.Models,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(arr)
}

func joinModels(models []string) string {
	if len(models) == 0 {
		return "-"
	}
	s := models[0]
	for _, m := range models[1:] {
		s += ", " + m
	}
	return s
}

func fmtInt(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
