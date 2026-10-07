package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionPingTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &connectionPingTask{}
	assert.Equal(t, "postgres.connection.ping", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConnectionPingTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &connectionPingTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestConnectionPingTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePostgresClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{"PostgreSQL 16.2 on x86_64-pc-linux-gnu"}}
		},
	}
	task := &connectionPingTask{provider: fakePostgresProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["healthy"])
	assert.Equal(t, "PostgreSQL 16.2 on x86_64-pc-linux-gnu", data["version"])
	assert.Contains(t, data, "latency_ms")
}

func TestConnectionPingTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakePostgresClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: errors.New("connection refused")}
		},
	}
	task := &connectionPingTask{provider: fakePostgresProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "ping failed")
	assert.Contains(t, res.Error, "connection refused")
}
