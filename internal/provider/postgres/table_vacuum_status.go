package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*tableVacuumStatusTask)(nil)

// tableVacuumStatusQuery returns per-table vacuum health from
// pg_stat_user_tables, ordered by dead-tuple ratio descending so the tables
// most in need of vacuuming appear first.
const tableVacuumStatusQuery = `SELECT schemaname, relname, n_live_tup, n_dead_tup, CASE WHEN n_live_tup > 0 THEN (n_dead_tup::float / n_live_tup * 100) ELSE 0 END as dead_tup_ratio_pct, last_vacuum, last_autovacuum, vacuum_count, autovacuum_count, analyze_count, autoanalyze_count FROM pg_stat_user_tables WHERE schemaname = $1 ORDER BY dead_tup_ratio_pct DESC LIMIT $2`

// TableVacuumStatus holds vacuum health metrics for a single user table.
type TableVacuumStatus struct {
	SchemaName       string     `json:"schema_name"`
	RelName          string     `json:"rel_name"`
	LiveTuples       int64      `json:"live_tuples"`
	DeadTuples       int64      `json:"dead_tuples"`
	DeadTupRatioPct  float64    `json:"dead_tup_ratio_pct"`
	LastVacuum       *time.Time `json:"last_vacuum,omitempty"`
	LastAutovacuum   *time.Time `json:"last_autovacuum,omitempty"`
	VacuumCount      int64      `json:"vacuum_count"`
	AutovacuumCount  int64      `json:"autovacuum_count"`
	AnalyzeCount     int64      `json:"analyze_count"`
	AutoanalyzeCount int64      `json:"autoanalyze_count"`
	NeedsVacuum      bool       `json:"needs_vacuum"`
}

type tableVacuumStatusTask struct {
	provider *Provider
	client   PostgresClient
}

func (t *tableVacuumStatusTask) currentClient() PostgresClient {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *tableVacuumStatusTask) Name() string { return "postgres.table.vacuum_status" }

func (t *tableVacuumStatusTask) JSONSchema() string { return tableVacuumStatusSchema }

func (t *tableVacuumStatusTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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
	threshold, err := optionalFloat(params, "dead_tup_ratio_threshold", 10)
	if err != nil {
		return common.TaskFailure(err)
	}
	if threshold < 0 {
		return common.TaskFailure(fmt.Errorf("parameter dead_tup_ratio_threshold must not be negative"))
	}

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	rows, err := client.Query(ctx, tableVacuumStatusQuery, schema, limit)
	if err != nil {
		return common.TaskFailure(err)
	}
	defer rows.Close()

	tables := make([]TableVacuumStatus, 0)
	for rows.Next() {
		var s TableVacuumStatus
		if err := rows.Scan(
			&s.SchemaName,
			&s.RelName,
			&s.LiveTuples,
			&s.DeadTuples,
			&s.DeadTupRatioPct,
			&s.LastVacuum,
			&s.LastAutovacuum,
			&s.VacuumCount,
			&s.AutovacuumCount,
			&s.AnalyzeCount,
			&s.AutoanalyzeCount,
		); err != nil {
			return common.TaskFailure(err)
		}
		s.NeedsVacuum = s.DeadTupRatioPct >= threshold
		tables = append(tables, s)
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"schema":                   schema,
		"tables":                   tables,
		"count":                    len(tables),
		"dead_tup_ratio_threshold": threshold,
	}), nil
}

// optionalFloat extracts an optional numeric parameter, accepting int, int32,
// int64, and float64 values. It returns defaultValue when the key is absent.
func optionalFloat(params map[string]any, key string, defaultValue float64) (float64, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	switch value := v.(type) {
	case int:
		return float64(value), nil
	case int32:
		return float64(value), nil
	case int64:
		return float64(value), nil
	case float64:
		return value, nil
	default:
		return 0, fmt.Errorf("parameter %s must be a number", key)
	}
}

const tableVacuumStatusSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Table Vacuum Status Parameters",
  "description": "Return vacuum and analyze status for tables.",
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
      "description": "Maximum number of tables to return, ordered by dead-tuple ratio descending."
    },
    "dead_tup_ratio_threshold": {
      "type": "number",
      "minimum": 0,
      "default": 10,
      "description": "Dead-tuple ratio percentage at or above which needs_vacuum is true."
    }
  }
}`
