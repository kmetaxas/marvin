package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*tableStatsTask)(nil)

// tableStatsClient is the narrow client surface required by the table stats
// task. It is satisfied by PostgresClient and can be faked in tests without a
// live database.
type tableStatsClient interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// tableStatsQuery returns per-table statistics from pg_stat_user_tables,
// ordered by total on-disk size (table + indexes + toast) descending.
const tableStatsQuery = `SELECT schemaname, relname, n_live_tup, n_dead_tup, n_tup_ins, n_tup_upd, n_tup_del, seq_scan, idx_scan, last_vacuum, last_autovacuum, last_analyze, last_autoanalyze, pg_total_relation_size(relid) as total_size_bytes FROM pg_stat_user_tables WHERE schemaname = $1 ORDER BY pg_total_relation_size(relid) DESC LIMIT $2`

// TableStat holds statistics for a single user table.
type TableStat struct {
	SchemaName      string     `json:"schema_name"`
	RelName         string     `json:"rel_name"`
	LiveTuples      int64      `json:"live_tuples"`
	DeadTuples      int64      `json:"dead_tuples"`
	InsertedTuples  int64      `json:"inserted_tuples"`
	UpdatedTuples   int64      `json:"updated_tuples"`
	DeletedTuples   int64      `json:"deleted_tuples"`
	SeqScans        int64      `json:"seq_scans"`
	IndexScans      int64      `json:"index_scans"`
	LastVacuum      *time.Time `json:"last_vacuum,omitempty"`
	LastAutovacuum  *time.Time `json:"last_autovacuum,omitempty"`
	LastAnalyze     *time.Time `json:"last_analyze,omitempty"`
	LastAutoanalyze *time.Time `json:"last_autoanalyze,omitempty"`
	TotalSizeBytes  int64      `json:"total_size_bytes"`
}

type tableStatsTask struct {
	provider *Provider
	client   tableStatsClient
}

func (t *tableStatsTask) currentClient() tableStatsClient {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *tableStatsTask) Name() string { return "postgres.table.stats" }

func (t *tableStatsTask) JSONSchema() string { return tableStatsSchema }

func (t *tableStatsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	schema, err := common.OptionalString(params, "schema", "public")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := common.OptionalInt(params, "limit", 50)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than 0"))
	}
	if limit > 1000 {
		limit = 1000
	}

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	rows, err := client.Query(ctx, tableStatsQuery, schema, limit)
	if err != nil {
		return common.TaskFailure(err)
	}
	defer rows.Close()

	stats := make([]TableStat, 0)
	for rows.Next() {
		var s TableStat
		if err := rows.Scan(
			&s.SchemaName,
			&s.RelName,
			&s.LiveTuples,
			&s.DeadTuples,
			&s.InsertedTuples,
			&s.UpdatedTuples,
			&s.DeletedTuples,
			&s.SeqScans,
			&s.IndexScans,
			&s.LastVacuum,
			&s.LastAutovacuum,
			&s.LastAnalyze,
			&s.LastAutoanalyze,
			&s.TotalSizeBytes,
		); err != nil {
			return common.TaskFailure(err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"schema": schema,
		"tables": stats,
		"count":  len(stats),
	}), nil
}

const tableStatsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Table Stats Parameters",
  "description": "Return table row counts, sizes, and index usage.",
  "properties": {
    "schema": {
      "type": "string",
      "default": "public",
      "description": "Schema name to inspect."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 50,
      "description": "Maximum number of tables to return, ordered by total size descending."
    }
  }
}`
