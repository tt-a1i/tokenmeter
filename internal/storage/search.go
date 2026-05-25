package storage

import (
	"strings"
	"time"
)

func (s *DB) SearchAdvanced(q Query, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if !q.hasAdvancedFilters() && len(q.Keywords) > 0 {
		return s.SearchHits(strings.Join(q.Keywords, " "), limit)
	}

	var selects []string
	var args []any
	keywordText := strings.Join(q.Keywords, " ")
	includeFiles := q.Tool == "" && (q.Status == "" || q.Status == "all")

	toolWhere, toolArgs := searchAdvancedWhere("tc.session_id", "s", "tc.start_time", "COALESCE(tc.params_summary, '')", q)
	selects = append(selects, `
		SELECT tc.session_id,
		       COALESCE(s.git_branch, '') AS git_branch,
		       COALESCE(s.cwd, '') AS cwd,
		       s.platform,
		       'tool_param' AS kind,
		       COALESCE(tc.params_summary, '') AS body,
		       tc.start_time AS timestamp
		FROM tool_calls tc
		JOIN sessions s ON tc.session_id = s.session_id
		WHERE `+toolWhere)
	args = append(args, toolArgs...)

	resultWhere, resultArgs := searchAdvancedWhere("tc.session_id", "s", "COALESCE(tc.end_time, tc.start_time)", "COALESCE(tc.result_summary, '')", q)
	selects = append(selects, `
		SELECT tc.session_id,
		       COALESCE(s.git_branch, '') AS git_branch,
		       COALESCE(s.cwd, '') AS cwd,
		       s.platform,
		       'tool_result' AS kind,
		       COALESCE(tc.result_summary, '') AS body,
		       COALESCE(tc.end_time, tc.start_time) AS timestamp
		FROM tool_calls tc
		JOIN sessions s ON tc.session_id = s.session_id
		WHERE `+resultWhere)
	args = append(args, resultArgs...)

	if includeFiles {
		fileWhere, fileArgs := searchAdvancedWhere("fc.session_id", "s", "fc.timestamp", "fc.file_path", q)
		selects = append(selects, `
			SELECT fc.session_id,
			       COALESCE(s.git_branch, '') AS git_branch,
			       COALESCE(s.cwd, '') AS cwd,
			       s.platform,
			       'file' AS kind,
			       fc.file_path AS body,
			       fc.timestamp AS timestamp
			FROM file_changes fc
			JOIN sessions s ON fc.session_id = s.session_id
			WHERE `+fileWhere)
		args = append(args, fileArgs...)
	}

	sql := `
		SELECT session_id, git_branch, cwd, platform, kind, body, timestamp
		FROM (` + strings.Join(selects, " UNION ALL ") + `)
		ORDER BY timestamp DESC
		LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := make([]SearchHit, 0)
	for rows.Next() {
		var hit SearchHit
		var gitBranch, cwd, body, ts string
		if err := rows.Scan(&hit.SessionID, &gitBranch, &cwd, &hit.Platform, &hit.Kind, &body, &ts); err != nil {
			return nil, err
		}
		hit.SessionName = searchSessionName(hit.SessionID, gitBranch, cwd)
		hit.Excerpt = searchExcerpt(body, keywordText)
		hit.Timestamp = parseTime(ts)
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

func searchAdvancedWhere(sessionExpr, sessionAlias, tsExpr, bodyExpr string, q Query) (string, []any) {
	where := []string{"1=1"}
	args := []any{}
	for _, keyword := range q.Keywords {
		where = append(where, bodyExpr+" LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLikePattern(keyword)+"%")
	}
	if q.Tool != "" {
		where = append(where, "tc.tool_name = ?")
		args = append(args, q.Tool)
	}
	switch q.Status {
	case "failed":
		where = append(where, "tc.status = ?")
		args = append(args, "fail")
	case "ok":
		where = append(where, "tc.status = ?")
		args = append(args, "success")
	}
	if q.Session != "" {
		where = append(where, sessionExpr+" LIKE ? ESCAPE '\\'")
		args = append(args, escapeLikePattern(q.Session)+"%")
	}
	if q.Platform != "" && q.Platform != "all" {
		where = append(where, sessionAlias+".platform = ?")
		args = append(args, q.Platform)
	}
	if q.CostMin != nil {
		where = append(where, sessionAlias+".total_cost_usd > ?")
		args = append(args, *q.CostMin)
	}
	if q.CostMax != nil {
		where = append(where, sessionAlias+".total_cost_usd < ?")
		args = append(args, *q.CostMax)
	}
	totalTokens := "(" + sessionAlias + ".total_input_tokens + " + sessionAlias + ".total_output_tokens + " + sessionAlias + ".total_cache_read_tokens + " + sessionAlias + ".total_cache_creation_tokens)"
	if q.TokensMin != nil {
		where = append(where, totalTokens+" > ?")
		args = append(args, *q.TokensMin)
	}
	if q.TokensMax != nil {
		where = append(where, totalTokens+" < ?")
		args = append(args, *q.TokensMax)
	}
	if q.Since != nil {
		where = append(where, tsExpr+" >= ?")
		args = append(args, formatQueryTime(*q.Since))
	}
	if q.Until != nil {
		where = append(where, tsExpr+" < ?")
		args = append(args, formatQueryTime(q.Until.AddDate(0, 0, 1)))
	}
	return strings.Join(where, " AND "), args
}

func searchDateForReport(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
