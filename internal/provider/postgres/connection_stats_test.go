package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionStatsTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &connectionStatsTask{}
	assert.Equal(t, "postgres.connection.stats", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConnectionStatsTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &connectionStatsTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestConnectionStatsTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePostgresClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{int64(3), int64(5), int64(1), int64(9)}}
		},
		poolStats: PoolStats{
			TotalConns:        9,
			IdleConns:         5,
			AcquiredConns:     3,
			MaxConns:          10,
			EmptyAcquireCount: 42,
		},
	}
	task := &connectionStatsTask{provider: fakePostgresProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, int32(9), data["total_conns"])
	assert.Equal(t, int32(5), data["idle_conns"])
	assert.Equal(t, int32(3), data["active_conns"])
	assert.Equal(t, int64(42), data["wait_count"])

	states := data["sessions_by_state"].(map[string]int64)
	assert.Equal(t, int64(3), states["active"])
	assert.Equal(t, int64(5), states["idle"])
	assert.Equal(t, int64(1), states["idle_in_transaction"])
	assert.Equal(t, int64(9), states["total"])
}

func TestConnectionStatsTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakePostgresClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: errors.New("connection refused")}
		},
	}
	task := &connectionStatsTask{provider: fakePostgresProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "query pg_stat_activity")
	assert.Contains(t, res.Error, "connection refused")
}

func TestConnectionStatsQueryExcludesBackendPIDs(t *testing.T) {
	t.Parallel()

	assert.NotContains(t, connectionStatsQuery, "pid")
	assert.NotContains(t, connectionStatsQuery, "backend")
}
