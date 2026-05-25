package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tt-a1i/tokenmeter/internal/storage"
)

func runSearch() error {
	if maybePrintCmdHelp("search", os.Args[2:]) {
		return nil
	}
	args, err := parseSearchArgs(os.Args[2:])
	if err != nil {
		return err
	}
	query, err := storage.ParseQuery(args.Query)
	if err != nil {
		return err
	}

	db := mustOpenDB()
	defer db.Close()

	hits, err := db.SearchAdvanced(query, args.Limit)
	if err != nil {
		return err
	}
	if args.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"query":   query,
			"results": hits,
		})
	}

	fmt.Printf("Found %d matches:\n\n", len(hits))
	for _, hit := range hits {
		fmt.Printf("[%s] %s · %s\n", hit.Kind, hit.SessionName, hit.Timestamp.Format("2006-01-02 15:04"))
		detail := plainSearchExcerpt(hit.Excerpt)
		if hit.Kind == "tool_param" {
			if toolName := searchHitToolName(db, hit, strings.Join(query.Keywords, " "), true); toolName != "" {
				detail = toolName + " " + detail
			}
		} else if hit.Kind == "tool_result" && !strings.HasPrefix(detail, "output:") {
			detail = "output: " + detail
		}
		fmt.Printf("  %s\n\n", detail)
	}
	return nil
}

func plainSearchExcerpt(s string) string {
	s = strings.ReplaceAll(s, "<mark>", "")
	return strings.ReplaceAll(s, "</mark>", "")
}

type searchArgs struct {
	Query string
	Limit int
	JSON  bool
}

func parseSearchArgs(args []string) (searchArgs, error) {
	out := searchArgs{Limit: 20}
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 >= len(args) {
				return out, fmt.Errorf("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				return out, fmt.Errorf("invalid limit: %s", args[i+1])
			}
			out.Limit = n
			i++
		case "--json":
			out.JSON = true
		default:
			if strings.HasPrefix(args[i], "--") {
				return out, fmt.Errorf("unknown search argument: %s", args[i])
			}
			queryParts = append(queryParts, args[i])
		}
	}
	out.Query = strings.TrimSpace(strings.Join(queryParts, " "))
	if out.Query == "" {
		return out, fmt.Errorf("usage: tm search <query> [--limit N]")
	}
	return out, nil
}

func searchHitToolName(db *storage.DB, hit storage.SearchHit, query string, params bool) string {
	calls, err := db.ListToolCalls(hit.SessionID, 200)
	if err != nil {
		return ""
	}
	query = strings.ToLower(query)
	for _, call := range calls {
		if params && strings.Contains(strings.ToLower(call.ParamsSummary), query) {
			return call.ToolName
		}
		if !params && strings.Contains(strings.ToLower(call.ResultSummary), query) {
			return call.ToolName
		}
	}
	return ""
}
