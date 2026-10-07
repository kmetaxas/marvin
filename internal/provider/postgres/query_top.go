package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// queryTopTask returns top queries from pg_stat_statements if available,
// falling back to pg_stat_activity otherwise.
type queryTopTask struct {
	provider *Provider
	client   queryTopClient
}

// queryTopClient is the narrow client surface required by queryTopTask.
type queryTopClient interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (t *queryTopTask) currentClient() queryTopClient {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *queryTopTask) Name() string { return "postgres.query.top" }

func (t *queryTopTask) JSONSchema() string { return queryTopSchema }

func (t *queryTopTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	limit, err := common.OptionalInt(params, "limit", 10)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than 0"))
	}
	if limit > 1000 {
		limit = 1000
	}

	sortBy, err := common.OptionalString(params, "sort_by", "total_time")
	if err != nil {
		return common.TaskFailure(err)
	}
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	// Try pg_stat_statements first.
	queries, err := t.queryPgStatStatements(ctx, client, limit, sortBy)
	if err == nil {
		return common.SuccessResult(map[string]any{
			"source":  "pg_stat_statements",
			"queries": queries,
			"count":   len(queries),
		}), nil
	}

	// Fallback to pg_stat_activity.
	queries, err = t.queryPgStatActivity(ctx, client, limit, sortBy)
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"source":  "pg_stat_activity",
		"queries": queries,
		"count":   len(queries),
	}), nil
}

// pgStatStatementsQuery returns top queries from pg_stat_statements.
// Columns selected match the fields requested in the task spec.
const pgStatStatementsQuery = `SELECT queryid, LEFT(query, 256), calls, total_exec_time, mean_exec_time, rows, shared_blks_hit, shared_blks_read FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT $1`

func (t *queryTopTask) queryPgStatStatements(ctx context.Context, client queryTopClient, limit int, sortBy string) ([]queryTopResult, error) {
	orderCol := pgStatStatementsSortColumn(sortBy)
	sql := fmt.Sprintf("SELECT queryid, LEFT(query, 256), calls, total_exec_time, mean_exec_time, rows, shared_blks_hit, shared_blks_read FROM pg_stat_statements ORDER BY %s DESC LIMIT $1", orderCol)

	rows, err := client.Query(ctx, sql, limit)
	if err != nil {
		return nil, fmt.Errorf("pg_stat_statements query failed: %w", err)
	}
	defer rows.Close()

	return scanQueryTopResults(rows)
}

func pgStatStatementsSortColumn(sortBy string) string {
	switch sortBy {
	case "calls":
		return "calls"
	case "mean_time":
		return "mean_exec_time"
	case "rows":
		return "rows"
	case "total_time":
		return "total_exec_time"
	default:
		return "total_exec_time"
	}
}

// pgStatActivityQuery returns active queries from pg_stat_activity as a fallback.
const pgStatActivityQuery = `SELECT pid, LEFT(query, 256), state, EXTRACT(EPOCH FROM (now() - query_start))::float8 * 1000.0 AS duration_ms, usename, datname, application_name, client_addr::text FROM pg_stat_activity WHERE query_start IS NOT NULL AND query NOT LIKE 'SELECT pid, LEFT(query, 256)%' ORDER BY duration_ms DESC LIMIT $1`

func (t *queryTopTask) queryPgStatActivity(ctx context.Context, client queryTopClient, limit int, sortBy string) ([]queryTopResult, error) {
	// pg_stat_activity fallback does not support sorting by calls/rows/mean_time meaningfully,
	// so we always sort by duration_ms (total_time proxy) descending.
	_ = sortBy

	rows, err := client.Query(ctx, pgStatActivityQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("pg_stat_activity query failed: %w", err)
	}
	defer rows.Close()

	return scanQueryTopActivityResults(rows)
}

// queryTopResult holds data for a single query row.
type queryTopResult struct {
	QueryID        *int64   `json:"queryid,omitempty"`
	Query          string   `json:"query"`
	Calls          *int64   `json:"calls,omitempty"`
	TotalExecTime  *float64 `json:"total_exec_time_ms,omitempty"`
	MeanExecTime   *float64 `json:"mean_exec_time_ms,omitempty"`
	Rows           *int64   `json:"rows,omitempty"`
	SharedBlksHit  *int64   `json:"shared_blks_hit,omitempty"`
	SharedBlksRead *int64   `json:"shared_blks_read,omitempty"`

	// Fallback fields from pg_stat_activity
	PID             *int     `json:"pid,omitempty"`
	State           *string  `json:"state,omitempty"`
	DurationMs      *float64 `json:"duration_ms,omitempty"`
	UserName        *string  `json:"usename,omitempty"`
	DatabaseName    *string  `json:"datname,omitempty"`
	ApplicationName *string  `json:"application_name,omitempty"`
	ClientAddr      *string  `json:"client_addr,omitempty"`
}

func scanQueryTopResults(rows pgx.Rows) ([]queryTopResult, error) {
	results := make([]queryTopResult, 0, 10)
	for rows.Next() {
		var r queryTopResult
		if err := rows.Scan(
			&r.QueryID,
			&r.Query,
			&r.Calls,
			&r.TotalExecTime,
			&r.MeanExecTime,
			&r.Rows,
			&r.SharedBlksHit,
			&r.SharedBlksRead,
		); err != nil {
			return nil, fmt.Errorf("scan pg_stat_statements row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg_stat_statements rows error: %w", err)
	}
	return results, nil
}

func scanQueryTopActivityResults(rows pgx.Rows) ([]queryTopResult, error) {
	results := make([]queryTopResult, 0, 10)
	for rows.Next() {
		var r queryTopResult
		if err := rows.Scan(
			&r.PID,
			&r.Query,
			&r.State,
			&r.DurationMs,
			&r.UserName,
			&r.DatabaseName,
			&r.ApplicationName,
			&r.ClientAddr,
		); err != nil {
			return nil, fmt.Errorf("scan pg_stat_activity row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg_stat_activity rows error: %w", err)
	}
	return results, nil
}

const queryTopSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Top Queries Parameters",
  "description": "Return top queries by execution time or frequency.",
  "properties": {
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 10,
      "description": "Maximum number of queries to return."
    },
    "sort_by": {
      "type": "string",
      "enum": ["total_time", "calls", "mean_time", "rows"],
      "default": "total_time",
      "description": "Sort order for pg_stat_statements results. Fallback pg_stat_activity always sorts by duration."
    }
  }
}`

var _ task.Task = (*queryTopTask)(nil)
