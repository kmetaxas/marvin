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

// replicationSlotsRows implements pgx.Rows for replication slot tests.
type replicationSlotsRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *replicationSlotsRows) Close()                                       { r.closed = true }
func (r *replicationSlotsRows) Err() error                                   { return r.rowErr }
func (r *replicationSlotsRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *replicationSlotsRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *replicationSlotsRows) Values() ([]any, error)                       { return nil, nil }
func (r *replicationSlotsRows) RawValues() [][]byte                          { return nil }
func (r *replicationSlotsRows) Conn() *pgx.Conn                              { return nil }
func (r *replicationSlotsRows) TypeMap() *pgtype.Map                         { return nil }

func (r *replicationSlotsRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *replicationSlotsRows) Scan(dest ...any) error {
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
		case *bool:
			*p = values[i].(bool)
		case *int32:
			*p = values[i].(int32)
		case *int64:
			*p = values[i].(int64)
		case *uint32:
			*p = values[i].(uint32)
		case **string:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(string)
				*p = &v
			}
		case **int32:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(int32)
				*p = &v
			}
		case **int64:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(int64)
				*p = &v
			}
		case **uint32:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(uint32)
				*p = &v
			}
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

// fakeReplicationSlotsClient is a PostgresClient whose Query returns canned rows.
type fakeReplicationSlotsClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *fakeReplicationSlotsClient) Ping(ctx context.Context) error { return nil }
func (c *fakeReplicationSlotsClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}
func (c *fakeReplicationSlotsClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &errorRow{err: errors.New("not used")}
}
func (c *fakeReplicationSlotsClient) Close() error { return nil }
func (c *fakeReplicationSlotsClient) Guardrails() config.PostgresGuardrails {
	return config.PostgresGuardrails{}
}
func (c *fakeReplicationSlotsClient) Stat() PoolStats { return PoolStats{} }

func TestReplicationSlotsTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &replicationSlotsTask{}
	assert.Equal(t, "postgres.replication.slots", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestReplicationSlotsTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &replicationSlotsTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestReplicationSlotsTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakeReplicationSlotsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			require.Equal(t, 2, len(args))
			require.Equal(t, 50, args[0])
			require.Equal(t, "", args[1])
			return &replicationSlotsRows{rows: [][]any{
				{
					"slot_a", "pgoutput", "logical",
					uint32(16384), "mydb", true, int32(1234),
					uint32(5678), "0/16B3748", "0/16B3748",
					"reserved", int64(1048576),
				},
				{
					"slot_b", nil, "physical",
					nil, nil, false, nil,
					nil, nil, nil,
					nil, nil,
				},
			}}, nil
		},
	}
	task := &replicationSlotsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 50, data["limit"])
	assert.Equal(t, "", data["slot_type"])
	assert.Equal(t, 2, data["count"])

	slots := data["slots"].([]ReplicationSlot)
	require.Len(t, slots, 2)

	assert.Equal(t, "slot_a", slots[0].SlotName)
	require.NotNil(t, slots[0].Plugin)
	assert.Equal(t, "pgoutput", *slots[0].Plugin)
	assert.Equal(t, "logical", slots[0].SlotType)
	require.NotNil(t, slots[0].DatabaseOID)
	assert.Equal(t, uint32(16384), *slots[0].DatabaseOID)
	require.NotNil(t, slots[0].Database)
	assert.Equal(t, "mydb", *slots[0].Database)
	assert.True(t, slots[0].Active)
	require.NotNil(t, slots[0].ActivePID)
	assert.Equal(t, int32(1234), *slots[0].ActivePID)
	require.NotNil(t, slots[0].Xmin)
	assert.Equal(t, uint32(5678), *slots[0].Xmin)
	require.NotNil(t, slots[0].RestartLSN)
	assert.Equal(t, "0/16B3748", *slots[0].RestartLSN)
	require.NotNil(t, slots[0].ConfirmedFlushLSN)
	assert.Equal(t, "0/16B3748", *slots[0].ConfirmedFlushLSN)
	require.NotNil(t, slots[0].WalStatus)
	assert.Equal(t, "reserved", *slots[0].WalStatus)
	require.NotNil(t, slots[0].SafeWalSize)
	assert.Equal(t, int64(1048576), *slots[0].SafeWalSize)

	assert.Equal(t, "slot_b", slots[1].SlotName)
	assert.Nil(t, slots[1].Plugin)
	assert.Equal(t, "physical", slots[1].SlotType)
	assert.False(t, slots[1].Active)
	assert.Nil(t, slots[1].ActivePID)
	assert.Nil(t, slots[1].RestartLSN)
}

func TestReplicationSlotsTaskEmpty(t *testing.T) {
	t.Parallel()

	client := &fakeReplicationSlotsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationSlotsRows{}, nil
		},
	}
	task := &replicationSlotsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
	slots := data["slots"].([]ReplicationSlot)
	assert.Empty(t, slots)
}

func TestReplicationSlotsTaskSlotTypeFilter(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeReplicationSlotsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &replicationSlotsRows{}, nil
		},
	}
	task := &replicationSlotsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{
		"limit":     10,
		"slot_type": "logical",
	})
	require.NoError(t, err)
	require.True(t, res.Success)

	assert.Equal(t, 10, capturedArgs[0])
	assert.Equal(t, "logical", capturedArgs[1])

	data := res.Data.(map[string]any)
	assert.Equal(t, "logical", data["slot_type"])
}

func TestReplicationSlotsTaskInvalidSlotType(t *testing.T) {
	t.Parallel()

	task := &replicationSlotsTask{client: &fakeReplicationSlotsClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"slot_type": "bogus"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "slot_type must be one of")
}

func TestReplicationSlotsTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &replicationSlotsTask{client: &fakeReplicationSlotsClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestReplicationSlotsTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeReplicationSlotsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &replicationSlotsRows{}, nil
		},
	}
	task := &replicationSlotsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 1000, capturedArgs[0])
}

func TestReplicationSlotsTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakeReplicationSlotsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &replicationSlotsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestReplicationSlotsTaskScanError(t *testing.T) {
	t.Parallel()

	client := &fakeReplicationSlotsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationSlotsRows{rows: [][]any{{"slot_a"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &replicationSlotsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestReplicationSlotsTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &fakeReplicationSlotsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &replicationSlotsRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &replicationSlotsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}
