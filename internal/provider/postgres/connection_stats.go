package postgres

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*connectionStatsTask)(nil)

type poolStatter interface {
	Stat() PoolStats
}

const connectionStatsQuery = `
SELECT
  count(*) FILTER (WHERE state = 'active')              AS active,
  count(*) FILTER (WHERE state = 'idle')                AS idle,
  count(*) FILTER (WHERE state = 'idle in transaction') AS idle_in_transaction,
  count(*)                                              AS total
FROM pg_stat_activity`

type connectionStatsTask struct {
	provider *Provider
	client   PostgresClient
}

func (t *connectionStatsTask) Name() string { return "postgres.connection.stats" }

func (t *connectionStatsTask) JSONSchema() string {
	return `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Connection Stats Parameters",
  "description": "Return connection pool statistics and active session counts by state.",
  "properties": {}
}`
}

func (t *connectionStatsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = params

	client := t.client
	if client == nil {
		client = t.provider.CurrentClient()
	}
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	var active, idle, idleInTx, total int64
	if err := client.QueryRow(ctx, connectionStatsQuery).Scan(&active, &idle, &idleInTx, &total); err != nil {
		return common.TaskFailure(fmt.Errorf("query pg_stat_activity: %w", err))
	}

	var ps PoolStats
	if statter, ok := client.(poolStatter); ok {
		ps = statter.Stat()
	}

	return common.SuccessResult(map[string]any{
		"total_conns":  ps.TotalConns,
		"idle_conns":   ps.IdleConns,
		"active_conns": ps.AcquiredConns,
		"wait_count":   ps.EmptyAcquireCount,
		"sessions_by_state": map[string]int64{
			"active":              active,
			"idle":                idle,
			"idle_in_transaction": idleInTx,
			"total":               total,
		},
	}), nil
}

func (c *clientImpl) Stat() PoolStats {
	if c == nil || c.pool == nil {
		return PoolStats{}
	}
	s := c.pool.Stat()
	return PoolStats{
		TotalConns:        s.TotalConns(),
		IdleConns:         s.IdleConns(),
		AcquiredConns:     s.AcquiredConns(),
		MaxConns:          s.MaxConns(),
		EmptyAcquireCount: s.EmptyAcquireCount(),
	}
}
