package postgres

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// instanceInfoTask returns metadata about the connected PostgreSQL instance:
// server version, network address/port, current database and user, timezone,
// server encoding, and whether the current user is a superuser.
//
// It deliberately does not expose filesystem paths such as data_directory or
// config_file, which would leak host layout information.
type instanceInfoTask struct {
	provider *Provider
	client   PostgresClient
}

func (t *instanceInfoTask) Name() string {
	return "postgres.instance.info"
}

func (t *instanceInfoTask) JSONSchema() string {
	return `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Instance Info Parameters",
  "description": "Return PostgreSQL instance version, settings, and uptime.",
  "properties": {}
}`
}

// instanceInfoQuery collects all instance metadata in a single round trip.
// inet_server_addr()/inet_server_port() return NULL when connected over a Unix
// socket, so they are coalesced to safe defaults. is_superuser is a read-only
// GUC reported as 'on'/'off'.
const instanceInfoQuery = `SELECT
    version(),
    COALESCE(inet_server_addr()::text, ''),
    COALESCE(inet_server_port(), 0),
    current_database(),
    current_user,
    current_setting('TimeZone'),
    pg_encoding_to_char(d.encoding),
    current_setting('is_superuser')
FROM pg_database d
WHERE d.datname = current_database()`

func (t *instanceInfoTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = params

	client := t.client
	if client == nil {
		client = t.provider.CurrentClient()
	}
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	var (
		version        string
		serverAddr     string
		serverPort     int
		databaseName   string
		currentUser    string
		timezone       string
		serverEncoding string
		isSuperuserRaw string
	)

	row := client.QueryRow(ctx, instanceInfoQuery)
	if err := row.Scan(
		&version,
		&serverAddr,
		&serverPort,
		&databaseName,
		&currentUser,
		&timezone,
		&serverEncoding,
		&isSuperuserRaw,
	); err != nil {
		return common.TaskFailure(fmt.Errorf("query instance info: %w", err))
	}

	return common.SuccessResult(map[string]any{
		"version":         version,
		"server_addr":     serverAddr,
		"server_port":     serverPort,
		"database_name":   databaseName,
		"current_user":    currentUser,
		"timezone":        timezone,
		"server_encoding": serverEncoding,
		"is_superuser":    isSuperuserRaw == "on",
	}), nil
}

var _ task.Task = (*instanceInfoTask)(nil)
