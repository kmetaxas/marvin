package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replicationStatusRows implements pgx.Rows for replication status tests.
type replicationStatusRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *replicationStatusRows) Close()                                       { r.closed = true }
func (r *replicationStatusRows) Err() error                                   { return r.rowErr }
func (r *replicationStatusRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *replicationStatusRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *replicationStatusRows) Values() ([]any, error)                       { return nil, nil }
func (r *replicationStatusRows) RawValues() [][]byte                          { return nil }
func (r *replicationStatusRows) Conn() *pgx.Conn                              { return nil }
func (r *replicationStatusRows) TypeMap() *pgtype.Map                         { return nil }
func (r *replicationStatusRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *replicationStatusRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	values := r.rows[r.idx-1]
	if len(dest) != len(values) {
		return errors.New("scan destination count mismatch")
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			*p = values[i].(string)
		case *int32:
			*p = values[i].(int32)
		case *bool:
			*p = values[i].(bool)
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

// replicationStatusClient is a PostgresClient with configurable QueryRow and Query.
type replicationStatusClient struct {
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *replicationStatusClient) Ping(ctx context.Context) error { return nil }
func (c *replicationStatusClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if c.queryFunc != nil {
		return c.queryFunc(ctx, sql, args...)
	}
	return nil, nil
}
func (c *replicationStatusClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if c.queryRowFunc != nil {
		return c.queryRowFunc(ctx, sql, args...)
	}
	return &errorRow{err: errors.New("no queryRowFunc configured")}
}
func (c *replicationStatusClient) Close() error { return nil }
func (c *replicationStatusClient) Guardrails() config.PostgresGuardrails {
	return config.PostgresGuardrails{}
}
func (c *replicationStatusClient) Stat() PoolStats { return PoolStats{} }

func TestReplicationStatusTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &replicationStatusTask{}
	assert.Equal(t, "postgres.replication.status", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestReplicationStatusTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &replicationStatusTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestReplicationStatusTaskPrimaryNoReplicas(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{false}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rows: [][]any{}}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["replication_enabled"])
	assert.Equal(t, "Replication not configured", data["message"])
}

func TestReplicationStatusTaskPrimaryWithReplicas(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{false}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rows: [][]any{
				{
					"app1", "10.0.0.1", "streaming",
					"0/1000000", "0/1000000", "0/1000000", "0/1000000",
					"00:00:00.001", "00:00:00.002", "00:00:00.003",
				},
				{
					"app2", "10.0.0.2", "catchup",
					"0/2000000", "0/2000000", "0/2000000", "0/2000000",
					"", "", "",
				},
			}}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["replication_enabled"])
	assert.Equal(t, "primary", data["role"])
	assert.Equal(t, 2, data["replica_count"])

	replicas := data["replicas"].([]ReplicationConnection)
	require.Len(t, replicas, 2)

	assert.Equal(t, "app1", replicas[0].ApplicationName)
	assert.Equal(t, "10.0.0.1", replicas[0].ClientAddr)
	assert.Equal(t, "streaming", replicas[0].State)
	assert.Equal(t, "0/1000000", replicas[0].SentLsn)
	assert.Equal(t, "0/1000000", replicas[0].WriteLsn)
	assert.Equal(t, "0/1000000", replicas[0].FlushLsn)
	assert.Equal(t, "0/1000000", replicas[0].ReplayLsn)
	assert.Equal(t, "00:00:00.001", replicas[0].WriteLag)
	assert.Equal(t, "00:00:00.002", replicas[0].FlushLag)
	assert.Equal(t, "00:00:00.003", replicas[0].ReplayLag)

	assert.Equal(t, "app2", replicas[1].ApplicationName)
	assert.Equal(t, "catchup", replicas[1].State)
}

func TestReplicationStatusTaskReplicaWithReceiver(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{true}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rows: [][]any{
				{
					"host=primary.example.com port=5432 user=replicator",
					"0/3000000", "0/3000001",
					"2024-01-01 12:00:00+00", "primary.example.com", int32(5432),
				},
			}}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["replication_enabled"])
	assert.Equal(t, "replica", data["role"])

	walReceiver := data["wal_receiver"].(WALReceiverInfo)
	assert.Equal(t, "host=primary.example.com port=5432 user=replicator", walReceiver.ConnInfo)
	assert.Equal(t, "0/3000000", walReceiver.ReceivedLsn)
	assert.Equal(t, "0/3000001", walReceiver.LatestEndLsn)
	assert.Equal(t, "2024-01-01 12:00:00+00", walReceiver.LatestEndTime)
	assert.Equal(t, "primary.example.com", walReceiver.SenderHost)
	assert.Equal(t, int32(5432), walReceiver.SenderPort)
}

func TestReplicationStatusTaskReplicaNoReceiver(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{true}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rows: [][]any{}}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["replication_enabled"])
	assert.Equal(t, "Replication not configured", data["message"])
}

func TestReplicationStatusTaskRecoveryQueryError(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{err: errors.New("connection refused")}
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "query pg_is_in_recovery")
	assert.Contains(t, res.Error, "connection refused")
}

func TestReplicationStatusTaskPrimaryQueryError(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{false}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "query pg_stat_replication")
	assert.Contains(t, res.Error, "connection refused")
}

func TestReplicationStatusTaskPrimaryScanError(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{false}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rows: [][]any{{"not-enough"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan pg_stat_replication")
	assert.Contains(t, res.Error, "scan failed")
}

func TestReplicationStatusTaskReplicaQueryError(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{true}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "query pg_stat_wal_receiver")
	assert.Contains(t, res.Error, "connection refused")
}

func TestReplicationStatusTaskReplicaScanError(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{true}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rows: [][]any{{"not-enough"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan pg_stat_wal_receiver")
	assert.Contains(t, res.Error, "scan failed")
}

func TestReplicationStatusTaskPrimaryRowsError(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{false}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "iterate pg_stat_replication")
	assert.Contains(t, res.Error, "rows failed")
}

func TestReplicationStatusTaskReplicaRowsError(t *testing.T) {
	t.Parallel()

	client := &replicationStatusClient{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{values: []any{true}}
		},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationStatusRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &replicationStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "iterate pg_stat_wal_receiver")
	assert.Contains(t, res.Error, "rows failed")
}
