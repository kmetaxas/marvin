package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*transactionLongRunningTask)(nil)

// transactionLongRunningQuery returns transactions that have been open longer
// than the supplied interval. $1 is the row limit, $2 is the minimum duration
// (encoded as a PostgreSQL interval). Idle sessions are excluded because they
// hold no active transaction work.
const transactionLongRunningQuery = `SELECT pid, usename, application_name, state, xact_start, query_start, EXTRACT(EPOCH FROM (now() - xact_start)) as duration_seconds, LEFT(query, 256) as query FROM pg_stat_activity WHERE xact_start IS NOT NULL AND state <> 'idle' AND (now() - xact_start) > $2 ORDER BY xact_start LIMIT $1`

// LongRunningTransaction describes a single transaction that has exceeded the
// configured duration threshold.
type LongRunningTransaction struct {
	PID             int32      `json:"pid"`
	UserName        string     `json:"usename"`
	ApplicationName string     `json:"application_name"`
	State           string     `json:"state"`
	XactStart       *time.Time `json:"xact_start,omitempty"`
	QueryStart      *time.Time `json:"query_start,omitempty"`
	DurationSeconds float64    `json:"duration_seconds"`
	Query           string     `json:"query"`
}

// transactionLongRunningClient is the narrow client surface required by
// transactionLongRunningTask.
type transactionLongRunningClient interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type transactionLongRunningTask struct {
	provider *Provider
	client   transactionLongRunningClient
}

func (t *transactionLongRunningTask) currentClient() transactionLongRunningClient {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *transactionLongRunningTask) Name() string { return "postgres.transaction.long_running" }

func (t *transactionLongRunningTask) JSONSchema() string { return transactionLongRunningSchema }

func (t *transactionLongRunningTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	minDurationStr, err := common.OptionalString(params, "min_duration", "5m")
	if err != nil {
		return common.TaskFailure(err)
	}
	minDuration, err := time.ParseDuration(minDurationStr)
	if err != nil {
		return common.TaskFailure(fmt.Errorf("parameter min_duration must be a valid duration: %w", err))
	}
	if minDuration <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter min_duration must be greater than 0"))
	}

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	rows, err := client.Query(ctx, transactionLongRunningQuery, limit, minDuration)
	if err != nil {
		return common.TaskFailure(err)
	}
	defer rows.Close()

	transactions := make([]LongRunningTransaction, 0)
	for rows.Next() {
		var tx LongRunningTransaction
		if err := rows.Scan(
			&tx.PID,
			&tx.UserName,
			&tx.ApplicationName,
			&tx.State,
			&tx.XactStart,
			&tx.QueryStart,
			&tx.DurationSeconds,
			&tx.Query,
		); err != nil {
			return common.TaskFailure(err)
		}
		tx.Query = truncateQuery(tx.Query)
		transactions = append(transactions, tx)
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"min_duration": minDurationStr,
		"limit":        limit,
		"transactions": transactions,
		"count":        len(transactions),
	}), nil
}

const transactionLongRunningSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Long Running Transactions Parameters",
  "description": "Find transactions that have been running longer than a threshold.",
  "properties": {
    "min_duration": {
      "type": "string",
      "default": "5m",
      "description": "Minimum transaction age as a Go duration string (e.g. 30s, 5m, 1h)."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 50,
      "description": "Maximum number of long-running transactions to return."
    }
  }
}`
