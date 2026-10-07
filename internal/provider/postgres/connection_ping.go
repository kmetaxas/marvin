package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// connectionPingTask tests connectivity to the configured PostgreSQL database
// by running a lightweight query and reporting latency and server version.
type connectionPingTask struct {
	provider *Provider
}

func (t *connectionPingTask) Name() string {
	return "postgres.connection.ping"
}

func (t *connectionPingTask) JSONSchema() string {
	return `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Connection Ping Parameters",
  "description": "Test connectivity to a PostgreSQL database and return latency and version information.",
  "properties": {
    "timeout": {
      "type": "string",
      "description": "Query timeout duration (e.g., '5s')."
    }
  }
}`
}

func (t *connectionPingTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	start := time.Now()

	var version string
	row := client.QueryRow(ctx, "SELECT version()")
	if err := row.Scan(&version); err != nil {
		return common.TaskFailure(fmt.Errorf("ping failed: %w", err))
	}

	latency := time.Since(start).Milliseconds()

	return common.SuccessResult(map[string]any{
		"healthy":    true,
		"latency_ms": latency,
		"version":    version,
	}), nil
}

var _ task.Task = (*connectionPingTask)(nil)
