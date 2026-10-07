package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vacuumFakeRows implements pgx.Rows for vacuum status tests.
type vacuumFakeRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
}

func (r *vacuumFakeRows) Close()                                       {}
func (r *vacuumFakeRows) Err() error                                   { return r.rowErr }
func (r *vacuumFakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *vacuumFakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *vacuumFakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *vacuumFakeRows) RawValues() [][]byte                          { return nil }
func (r *vacuumFakeRows) Conn() *pgx.Conn                              { return nil }
func (r *vacuumFakeRows) TypeMap() *pgtype.Map                         { return nil }

func (r *vacuumFakeRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *vacuumFakeRows) Scan(dest ...any) error {
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
		case *int64:
			*p = values[i].(int64)
		case *float64:
			*p = values[i].(float64)
		case **time.Time:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(time.Time)
				*p = &v
			}
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

// vacuumStatusClient is a PostgresClient whose Query returns canned rows.
type vacuumStatusClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *vacuumStatusClient) Ping(ctx context.Context) error { return nil }
func (c *vacuumStatusClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}
func (c *vacuumStatusClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &errorRow{err: errors.New("not used")}
}
func (c *vacuumStatusClient) Close() error { return nil }
func (c *vacuumStatusClient) Guardrails() config.PostgresGuardrails {
	return config.PostgresGuardrails{}
}
func (c *vacuumStatusClient) Stat() PoolStats { return PoolStats{} }

func TestTableVacuumStatusTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &tableVacuumStatusTask{}
	assert.Equal(t, "postgres.table.vacuum_status", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTableVacuumStatusTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &tableVacuumStatusTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestTableVacuumStatusTaskSuccess(t *testing.T) {
	t.Parallel()

	vacuum := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	client := &vacuumStatusClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			require.Equal(t, "public", args[0])
			require.Equal(t, 50, args[1])
			return &vacuumFakeRows{rows: [][]any{
				{
					"public", "orders", int64(1000), int64(200), float64(20),
					vacuum, nil, int64(3), int64(1), int64(2), int64(4),
				},
				{
					"public", "users", int64(1000), int64(5), float64(0.5),
					nil, nil, int64(0), int64(0), int64(0), int64(0),
				},
			}}, nil
		},
	}
	task := &tableVacuumStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "public", data["schema"])
	assert.Equal(t, 2, data["count"])
	assert.Equal(t, float64(10), data["dead_tup_ratio_threshold"])

	tables := data["tables"].([]TableVacuumStatus)
	require.Len(t, tables, 2)

	assert.Equal(t, "orders", tables[0].RelName)
	assert.Equal(t, int64(1000), tables[0].LiveTuples)
	assert.Equal(t, int64(200), tables[0].DeadTuples)
	assert.Equal(t, float64(20), tables[0].DeadTupRatioPct)
	assert.True(t, tables[0].NeedsVacuum)
	assert.Equal(t, int64(3), tables[0].VacuumCount)
	assert.Equal(t, int64(1), tables[0].AutovacuumCount)
	assert.Equal(t, int64(2), tables[0].AnalyzeCount)
	assert.Equal(t, int64(4), tables[0].AutoanalyzeCount)
	require.NotNil(t, tables[0].LastVacuum)
	assert.Equal(t, vacuum, *tables[0].LastVacuum)

	assert.Equal(t, "users", tables[1].RelName)
	assert.False(t, tables[1].NeedsVacuum)
}

func TestTableVacuumStatusTaskCustomThreshold(t *testing.T) {
	t.Parallel()

	client := &vacuumStatusClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &vacuumFakeRows{rows: [][]any{
				{"public", "t", int64(100), int64(5), float64(5), nil, nil, int64(0), int64(0), int64(0), int64(0)},
			}}, nil
		},
	}
	task := &tableVacuumStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"dead_tup_ratio_threshold": 5})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, float64(5), data["dead_tup_ratio_threshold"])
	tables := data["tables"].([]TableVacuumStatus)
	require.Len(t, tables, 1)
	assert.True(t, tables[0].NeedsVacuum)
}

func TestTableVacuumStatusTaskCustomSchemaAndLimit(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &vacuumStatusClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &vacuumFakeRows{}, nil
		},
	}
	task := &tableVacuumStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{
		"schema": "analytics",
		"limit":  10,
	})
	require.NoError(t, err)
	require.True(t, res.Success)

	assert.Equal(t, "analytics", capturedArgs[0])
	assert.Equal(t, 10, capturedArgs[1])

	data := res.Data.(map[string]any)
	assert.Equal(t, "analytics", data["schema"])
	assert.Equal(t, 0, data["count"])
}

func TestTableVacuumStatusTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &tableVacuumStatusTask{client: &vacuumStatusClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestTableVacuumStatusTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &vacuumStatusClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &vacuumFakeRows{}, nil
		},
	}
	task := &tableVacuumStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 1000, capturedArgs[1])
}

func TestTableVacuumStatusTaskThresholdValidation(t *testing.T) {
	t.Parallel()

	task := &tableVacuumStatusTask{client: &vacuumStatusClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"dead_tup_ratio_threshold": -1})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "dead_tup_ratio_threshold must not be negative")
}

func TestTableVacuumStatusTaskThresholdWrongType(t *testing.T) {
	t.Parallel()

	task := &tableVacuumStatusTask{client: &vacuumStatusClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"dead_tup_ratio_threshold": "high"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "dead_tup_ratio_threshold must be a number")
}

func TestTableVacuumStatusTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &vacuumStatusClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &tableVacuumStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestTableVacuumStatusTaskScanError(t *testing.T) {
	t.Parallel()

	client := &vacuumStatusClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &vacuumFakeRows{rows: [][]any{{"public"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &tableVacuumStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestTableVacuumStatusTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &vacuumStatusClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &vacuumFakeRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &tableVacuumStatusTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}

func TestOptionalFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  map[string]any
		want    float64
		wantErr bool
	}{
		{"absent", map[string]any{}, 10, false},
		{"nil", map[string]any{"x": nil}, 10, false},
		{"int", map[string]any{"x": 5}, 5, false},
		{"int32", map[string]any{"x": int32(5)}, 5, false},
		{"int64", map[string]any{"x": int64(5)}, 5, false},
		{"float64", map[string]any{"x": 2.5}, 2.5, false},
		{"wrong type", map[string]any{"x": "nope"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := optionalFloat(tt.params, "x", 10)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
