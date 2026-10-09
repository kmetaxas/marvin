package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maintenanceProgressRows implements pgx.Rows for maintenance progress tests.
type maintenanceProgressRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *maintenanceProgressRows) Close()                                       { r.closed = true }
func (r *maintenanceProgressRows) Err() error                                   { return r.rowErr }
func (r *maintenanceProgressRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *maintenanceProgressRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *maintenanceProgressRows) Values() ([]any, error)                       { return nil, nil }
func (r *maintenanceProgressRows) RawValues() [][]byte                          { return nil }
func (r *maintenanceProgressRows) Conn() *pgx.Conn                              { return nil }
func (r *maintenanceProgressRows) TypeMap() *pgtype.Map                         { return nil }
func (r *maintenanceProgressRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *maintenanceProgressRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	values := r.rows[r.idx-1]
	if len(dest) != len(values) {
		return errors.New("scan destination count mismatch")
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *int32:
			*p = values[i].(int32)
		case *string:
			*p = values[i].(string)
		case **int64:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(int64)
				*p = &v
			}
		case *float64:
			*p = values[i].(float64)
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

type fakeMaintenanceProgressClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *fakeMaintenanceProgressClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}

func TestMaintenanceProgressTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &maintenanceProgressTask{}
	assert.Equal(t, "postgres.maintenance.progress", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestMaintenanceProgressTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &maintenanceProgressTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestMaintenanceProgressTaskEmpty(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
	assert.Equal(t, "no maintenance operations are currently running", data["message"])
	ops := data["operations"].([]MaintenanceProgress)
	assert.Empty(t, ops)
}

func TestMaintenanceProgressTaskVacuum(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == vacuumProgressQuery {
				return &maintenanceProgressRows{rows: [][]any{
					{int32(123), "testdb", int64(456), "VACUUM", "scanning heap", float64(42.5)},
				}}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])

	ops := data["operations"].([]MaintenanceProgress)
	require.Len(t, ops, 1)
	assert.Equal(t, int32(123), ops[0].PID)
	assert.Equal(t, "testdb", ops[0].DatabaseName)
	require.NotNil(t, ops[0].RelID)
	assert.Equal(t, int64(456), *ops[0].RelID)
	assert.Equal(t, "VACUUM", ops[0].Command)
	assert.Equal(t, "scanning heap", ops[0].Phase)
	assert.Equal(t, float64(42.5), ops[0].ProgressPct)
}

func TestMaintenanceProgressTaskAnalyze(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == analyzeProgressQuery {
				return &maintenanceProgressRows{rows: [][]any{
					{int32(124), "testdb", int64(789), "ANALYZE", "acquiring sample rows", float64(10.0)},
				}}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	ops := res.Data.(map[string]any)["operations"].([]MaintenanceProgress)
	require.Len(t, ops, 1)
	assert.Equal(t, "ANALYZE", ops[0].Command)
	assert.Equal(t, "acquiring sample rows", ops[0].Phase)
}

func TestMaintenanceProgressTaskCreateIndex(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == createIndexProgressQuery {
				return &maintenanceProgressRows{rows: [][]any{
					{int32(125), "testdb", int64(1000), "CREATE INDEX", "building index", float64(75.0)},
				}}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	ops := res.Data.(map[string]any)["operations"].([]MaintenanceProgress)
	require.Len(t, ops, 1)
	assert.Equal(t, "CREATE INDEX", ops[0].Command)
	assert.Equal(t, "building index", ops[0].Phase)
	assert.Equal(t, float64(75.0), ops[0].ProgressPct)
}

func TestMaintenanceProgressTaskCluster(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == clusterProgressQuery {
				return &maintenanceProgressRows{rows: [][]any{
					{int32(126), "testdb", int64(2000), "CLUSTER", "seqscanning heap", float64(0.0)},
				}}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	ops := res.Data.(map[string]any)["operations"].([]MaintenanceProgress)
	require.Len(t, ops, 1)
	assert.Equal(t, "CLUSTER", ops[0].Command)
	assert.Equal(t, "seqscanning heap", ops[0].Phase)
}

func TestMaintenanceProgressTaskBasebackup(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == basebackupProgressQuery {
				return &maintenanceProgressRows{rows: [][]any{
					{int32(127), "", nil, "BASEBACKUP", "streaming data", float64(50.0)},
				}}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	ops := res.Data.(map[string]any)["operations"].([]MaintenanceProgress)
	require.Len(t, ops, 1)
	assert.Equal(t, "BASEBACKUP", ops[0].Command)
	assert.Equal(t, "streaming data", ops[0].Phase)
	assert.Nil(t, ops[0].RelID)
}

func TestMaintenanceProgressTaskMultipleViews(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			switch sql {
			case vacuumProgressQuery:
				return &maintenanceProgressRows{rows: [][]any{
					{int32(123), "db1", int64(456), "VACUUM", "scanning heap", float64(42.5)},
				}}, nil
			case analyzeProgressQuery:
				return &maintenanceProgressRows{rows: [][]any{
					{int32(124), "db2", int64(789), "ANALYZE", "acquiring sample rows", float64(10.0)},
				}}, nil
			default:
				return &maintenanceProgressRows{}, nil
			}
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])

	ops := data["operations"].([]MaintenanceProgress)
	require.Len(t, ops, 2)
	assert.Equal(t, "VACUUM", ops[0].Command)
	assert.Equal(t, "ANALYZE", ops[1].Command)
}

func TestMaintenanceProgressTaskMissingView(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == vacuumProgressQuery {
				return nil, errors.New("relation does not exist")
			}
			if sql == analyzeProgressQuery {
				return &maintenanceProgressRows{rows: [][]any{
					{int32(124), "db2", int64(789), "ANALYZE", "acquiring sample rows", float64(10.0)},
				}}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	ops := res.Data.(map[string]any)["operations"].([]MaintenanceProgress)
	require.Len(t, ops, 1)
	assert.Equal(t, "ANALYZE", ops[0].Command)
}

func TestMaintenanceProgressTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
	assert.Equal(t, "no maintenance operations are currently running", data["message"])
}

func TestMaintenanceProgressTaskScanError(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == vacuumProgressQuery {
				return &maintenanceProgressRows{rows: [][]any{{"bad"}}, scanErr: errors.New("scan failed")}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
}

func TestMaintenanceProgressTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &fakeMaintenanceProgressClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql == vacuumProgressQuery {
				return &maintenanceProgressRows{rowErr: errors.New("rows failed")}, nil
			}
			return &maintenanceProgressRows{}, nil
		},
	}
	task := &maintenanceProgressTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
}
