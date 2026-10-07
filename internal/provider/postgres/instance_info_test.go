package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceInfoTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &instanceInfoTask{}
	assert.Equal(t, "postgres.instance.info", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestInstanceInfoTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &instanceInfoTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestInstanceInfoTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePostgresClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{
				"PostgreSQL 16.2 on x86_64-pc-linux-gnu",
				"10.0.0.5",
				5432,
				"appdb",
				"appuser",
				"UTC",
				"UTF8",
				"off",
			}}
		},
	}
	task := &instanceInfoTask{provider: fakePostgresProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "PostgreSQL 16.2 on x86_64-pc-linux-gnu", data["version"])
	assert.Equal(t, "10.0.0.5", data["server_addr"])
	assert.Equal(t, 5432, data["server_port"])
	assert.Equal(t, "appdb", data["database_name"])
	assert.Equal(t, "appuser", data["current_user"])
	assert.Equal(t, "UTC", data["timezone"])
	assert.Equal(t, "UTF8", data["server_encoding"])
	assert.Equal(t, false, data["is_superuser"])
}

func TestInstanceInfoTaskSuperuserTrue(t *testing.T) {
	t.Parallel()

	client := &fakePostgresClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{
				"PostgreSQL 16.2",
				"",
				0,
				"postgres",
				"postgres",
				"Etc/UTC",
				"UTF8",
				"on",
			}}
		},
	}
	task := &instanceInfoTask{provider: fakePostgresProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "", data["server_addr"])
	assert.Equal(t, 0, data["server_port"])
	assert.Equal(t, true, data["is_superuser"])
}

func TestInstanceInfoTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakePostgresClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: errors.New("connection refused")}
		},
	}
	task := &instanceInfoTask{provider: fakePostgresProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "query instance info")
	assert.Contains(t, res.Error, "connection refused")
}

func TestInstanceInfoQueryExcludesSensitivePaths(t *testing.T) {
	t.Parallel()

	assert.NotContains(t, instanceInfoQuery, "data_directory")
	assert.NotContains(t, instanceInfoQuery, "config_file")
}
