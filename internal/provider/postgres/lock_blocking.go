package postgres

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*lockBlockingTask)(nil)

// lockBlockingQuery returns blocking lock chains by joining pg_locks with
// pg_stat_activity. It identifies sessions that are waiting for a lock and
// the sessions that currently hold the conflicting lock.
const lockBlockingQuery = `SELECT
	blocked_locks.pid AS blocked_pid,
	blocked_activity.usename AS blocked_user,
	blocked_activity.application_name AS blocked_application,
	blocking_locks.pid AS blocking_pid,
	blocking_activity.usename AS blocking_user,
	blocking_activity.application_name AS blocking_application,
	blocked_activity.query AS blocked_statement,
	blocking_activity.query AS blocking_statement,
	blocked_locks.mode AS lock_mode,
	blocked_locks.locktype AS lock_type,
	EXTRACT(EPOCH FROM (now() - blocked_activity.query_start)) * 1000 AS duration_ms
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks
	ON blocking_locks.locktype = blocked_locks.locktype
	AND blocking_locks.relation = blocked_locks.relation
	AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted AND blocking_locks.granted
LIMIT $1`

// BlockingLock represents a single blocking/waiting lock relationship.
type BlockingLock struct {
	BlockedPID          int32   `json:"blocked_pid"`
	BlockedUser         string  `json:"blocked_user"`
	BlockedApplication  string  `json:"blocked_application"`
	BlockingPID         int32   `json:"blocking_pid"`
	BlockingUser        string  `json:"blocking_user"`
	BlockingApplication string  `json:"blocking_application"`
	BlockedStatement    string  `json:"blocked_statement"`
	BlockingStatement   string  `json:"blocking_statement"`
	LockMode            string  `json:"lock_mode"`
	LockType            string  `json:"lock_type"`
	DurationMs          float64 `json:"duration_ms"`
}

type lockBlockingTask struct {
	provider *Provider
	client   PostgresClient
}

func (t *lockBlockingTask) currentClient() PostgresClient {
	if t.client != nil {
		return t.client
	}
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return nil
}

func (t *lockBlockingTask) Name() string { return "postgres.lock.blocking" }

func (t *lockBlockingTask) JSONSchema() string { return lockBlockingSchema }

func (t *lockBlockingTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	rows, err := client.Query(ctx, lockBlockingQuery, limit)
	if err != nil {
		return common.TaskFailure(err)
	}
	defer rows.Close()

	locks := make([]BlockingLock, 0)
	for rows.Next() {
		var l BlockingLock
		if err := rows.Scan(
			&l.BlockedPID,
			&l.BlockedUser,
			&l.BlockedApplication,
			&l.BlockingPID,
			&l.BlockingUser,
			&l.BlockingApplication,
			&l.BlockedStatement,
			&l.BlockingStatement,
			&l.LockMode,
			&l.LockType,
			&l.DurationMs,
		); err != nil {
			return common.TaskFailure(err)
		}
		l.BlockedStatement = truncateQuery(l.BlockedStatement)
		l.BlockingStatement = truncateQuery(l.BlockingStatement)
		locks = append(locks, l)
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"limit": limit,
		"locks": locks,
		"count": len(locks),
	}), nil
}

func truncateQuery(q string) string {
	if len(q) <= 256 {
		return q
	}
	return q[:256]
}

const lockBlockingSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Blocking Locks Parameters",
  "description": "Identify blocking and waiting lock relationships.",
  "properties": {
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 50,
      "description": "Maximum number of blocking lock relationships to return."
    }
  }
}`
