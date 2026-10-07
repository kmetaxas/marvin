package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*maintenanceProgressTask)(nil)

// maintenanceProgressClient is the narrow client surface required by the
// maintenance progress task. It is satisfied by PostgresClient and can be
// faked in tests without a live database.
type maintenanceProgressClient interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// MaintenanceProgress represents a single ongoing maintenance operation.
type MaintenanceProgress struct {
	PID          int32   `json:"pid"`
	DatabaseName string  `json:"datname"`
	RelID        *int64  `json:"relid,omitempty"`
	Command      string  `json:"command"`
	Phase        string  `json:"phase"`
	ProgressPct  float64 `json:"progress_pct"`
	Detail       string  `json:"detail,omitempty"`
}

type maintenanceProgressTask struct {
	provider *Provider
	client   maintenanceProgressClient
}

func (t *maintenanceProgressTask) currentClient() maintenanceProgressClient {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *maintenanceProgressTask) Name() string { return "postgres.maintenance.progress" }

func (t *maintenanceProgressTask) JSONSchema() string { return maintenanceProgressSchema }

func (t *maintenanceProgressTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = params

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	var results []MaintenanceProgress

	// Query each pg_stat_progress_* view independently. If a view does not
	// exist (older PostgreSQL) or returns an error, we skip it and continue
	// with the others so partial information is still useful.
	results = append(results, t.queryVacuum(ctx, client)...)
	results = append(results, t.queryAnalyze(ctx, client)...)
	results = append(results, t.queryCreateIndex(ctx, client)...)
	results = append(results, t.queryCluster(ctx, client)...)
	results = append(results, t.queryBasebackup(ctx, client)...)

	if len(results) == 0 {
		return common.SuccessResult(map[string]any{
			"operations": results,
			"count":      0,
			"message":    "no maintenance operations are currently running",
		}), nil
	}

	return common.SuccessResult(map[string]any{
		"operations": results,
		"count":      len(results),
	}), nil
}

// vacuumProgressQuery reports VACUUM progress.
const vacuumProgressQuery = `SELECT
	pid,
	COALESCE(datname, '') AS datname,
	relid::bigint AS relid,
	'VACUUM' AS command,
	phase,
	CASE WHEN heap_blks_total > 0 THEN (heap_blks_scanned::float8 / heap_blks_total::float8) * 100.0 ELSE 0 END AS progress_pct
FROM pg_stat_progress_vacuum
WHERE datname IS NOT NULL`

func (t *maintenanceProgressTask) queryVacuum(ctx context.Context, client maintenanceProgressClient) []MaintenanceProgress {
	return t.queryProgressView(ctx, client, vacuumProgressQuery, "VACUUM")
}

// analyzeProgressQuery reports ANALYZE progress.
const analyzeProgressQuery = `SELECT
	pid,
	COALESCE(datname, '') AS datname,
	relid::bigint AS relid,
	'ANALYZE' AS command,
	phase,
	CASE WHEN sample_blks_total > 0 THEN (sample_blks_scanned::float8 / sample_blks_total::float8) * 100.0 ELSE 0 END AS progress_pct
FROM pg_stat_progress_analyze
WHERE datname IS NOT NULL`

func (t *maintenanceProgressTask) queryAnalyze(ctx context.Context, client maintenanceProgressClient) []MaintenanceProgress {
	return t.queryProgressView(ctx, client, analyzeProgressQuery, "ANALYZE")
}

// createIndexProgressQuery reports CREATE INDEX / REINDEX progress.
const createIndexProgressQuery = `SELECT
	pid,
	COALESCE(datname, '') AS datname,
	relid::bigint AS relid,
	command,
	phase,
	CASE WHEN blocks_total > 0 THEN (blocks_done::float8 / blocks_total::float8) * 100.0 ELSE 0 END AS progress_pct
FROM pg_stat_progress_create_index
WHERE datname IS NOT NULL`

func (t *maintenanceProgressTask) queryCreateIndex(ctx context.Context, client maintenanceProgressClient) []MaintenanceProgress {
	return t.queryProgressView(ctx, client, createIndexProgressQuery, "CREATE INDEX")
}

// clusterProgressQuery reports CLUSTER progress.
const clusterProgressQuery = `SELECT
	pid,
	COALESCE(datname, '') AS datname,
	relid::bigint AS relid,
	command,
	phase,
	0 AS progress_pct
FROM pg_stat_progress_cluster
WHERE datname IS NOT NULL`

func (t *maintenanceProgressTask) queryCluster(ctx context.Context, client maintenanceProgressClient) []MaintenanceProgress {
	return t.queryProgressView(ctx, client, clusterProgressQuery, "CLUSTER")
}

// basebackupProgressQuery reports BASEBACKUP progress.
const basebackupProgressQuery = `SELECT
	pid,
	''::name AS datname,
	NULL::bigint AS relid,
	'BASEBACKUP' AS command,
	phase,
	CASE WHEN tablespaces_total > 0 THEN (tablespaces_streamed::float8 / tablespaces_total::float8) * 100.0 ELSE 0 END AS progress_pct
FROM pg_stat_progress_basebackup`

func (t *maintenanceProgressTask) queryBasebackup(ctx context.Context, client maintenanceProgressClient) []MaintenanceProgress {
	return t.queryProgressView(ctx, client, basebackupProgressQuery, "BASEBACKUP")
}

// queryProgressView executes a single progress-view query and scans the rows
// into MaintenanceProgress values.  If the query fails (e.g. the view does
// not exist) an empty slice is returned and the error is silently swallowed
// so that other views can still contribute results.
func (t *maintenanceProgressTask) queryProgressView(ctx context.Context, client maintenanceProgressClient, sql, defaultCommand string) []MaintenanceProgress {
	rows, err := client.Query(ctx, sql)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []MaintenanceProgress
	for rows.Next() {
		var p MaintenanceProgress
		var relid *int64
		if err := rows.Scan(&p.PID, &p.DatabaseName, &relid, &p.Command, &p.Phase, &p.ProgressPct); err != nil {
			return nil
		}
		p.RelID = relid
		if p.Command == "" {
			p.Command = defaultCommand
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return out
}

const maintenanceProgressSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Maintenance Progress Parameters",
  "description": "Show progress of running VACUUM, CREATE INDEX, or other maintenance operations.",
  "properties": {}
}`
