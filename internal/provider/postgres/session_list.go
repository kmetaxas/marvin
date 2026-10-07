package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*sessionListTask)(nil)

// sessionListClient is the narrow client surface required by sessionListTask.
// It is satisfied by PostgresClient and can be faked in tests without a live
// database.
type sessionListClient interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Guardrails() config.PostgresGuardrails
}

// sessionListBaseQuery selects the session columns returned by the task.
// client_addr is cast to text so it can be scanned into a string without
// depending on the inet codec.
const sessionListBaseQuery = `SELECT pid, usename, application_name, client_addr::text, state, query_start, xact_start, wait_event_type, wait_event, LEFT(query, 256) as query FROM pg_stat_activity`

// sessionListActiveFilter excludes idle sessions and orders by most recent
// query start. $1 is the row limit.
const sessionListActiveFilter = ` WHERE state IS NOT NULL AND state <> 'idle' ORDER BY query_start DESC LIMIT $1`

// sessionListStateFilter restricts results to a single session state.
// $1 is the row limit and $2 is the requested state.
const sessionListStateFilter = ` WHERE state = $2 ORDER BY query_start DESC LIMIT $1`

// Session describes a single row from pg_stat_activity.
type Session struct {
	PID             int32      `json:"pid"`
	UserName        *string    `json:"usename,omitempty"`
	ApplicationName *string    `json:"application_name,omitempty"`
	ClientAddr      *string    `json:"client_addr,omitempty"`
	State           *string    `json:"state,omitempty"`
	QueryStart      *time.Time `json:"query_start,omitempty"`
	XactStart       *time.Time `json:"xact_start,omitempty"`
	WaitEventType   *string    `json:"wait_event_type,omitempty"`
	WaitEvent       *string    `json:"wait_event,omitempty"`
	Query           string     `json:"query"`
}

type sessionListTask struct {
	provider *Provider
	client   sessionListClient
}

func (t *sessionListTask) currentClient() sessionListClient {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *sessionListTask) Name() string { return "postgres.session.list" }

func (t *sessionListTask) JSONSchema() string { return sessionListSchema }

func (t *sessionListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	defaultLimit := 50
	maxLimit := 1000
	if g := client.Guardrails(); g.MaxRows > 0 {
		defaultLimit = g.MaxRows
		maxLimit = g.MaxRows
	}

	limit, err := common.OptionalInt(params, "limit", defaultLimit)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than 0"))
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	state, err := common.OptionalString(params, "state", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	query := sessionListBaseQuery
	args := []any{limit}
	if state != "" {
		query += sessionListStateFilter
		args = append(args, state)
	} else {
		query += sessionListActiveFilter
	}

	rows, err := client.Query(ctx, query, args...)
	if err != nil {
		return common.TaskFailure(err)
	}
	defer rows.Close()

	sessions := make([]Session, 0)
	for rows.Next() {
		var s Session
		if err := rows.Scan(
			&s.PID,
			&s.UserName,
			&s.ApplicationName,
			&s.ClientAddr,
			&s.State,
			&s.QueryStart,
			&s.XactStart,
			&s.WaitEventType,
			&s.WaitEvent,
			&s.Query,
		); err != nil {
			return common.TaskFailure(err)
		}
		s.Query = truncateQuery(s.Query)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"limit":    limit,
		"state":    state,
		"sessions": sessions,
		"count":    len(sessions),
	}), nil
}

const sessionListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Session List Parameters",
  "description": "List active sessions with query, state, and duration.",
  "properties": {
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 50,
      "description": "Maximum number of sessions to return."
    },
    "state": {
      "type": "string",
      "enum": [
        "active",
        "idle",
        "idle in transaction",
        "idle in transaction (aborted)",
        "fastpath function call",
        "disabled"
      ],
      "description": "Filter sessions by state. When omitted, idle sessions are excluded."
    }
  }
}`
