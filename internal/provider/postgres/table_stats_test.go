package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRows implements pgx.Rows for table stats tests. It yields a fixed set of
// rows, each represented as a slice of scan destinations.
type fakeRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *fakeRows) Close()                                       { r.closed = true }
func (r *fakeRows) Err() error                                   { return r.rowErr }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) TypeMap() *pgtype.Map                         { return nil }

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
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

type fakeTableStatsClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *fakeTableStatsClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}

func TestTableStatsTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &tableStatsTask{}
	assert.Equal(t, "postgres.table.stats", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTableStatsTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &tableStatsTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestTableStatsTaskSuccess(t *testing.T) {
	t.Parallel()

	vacuum := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	client := &fakeTableStatsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			require.Equal(t, "public", args[0])
			require.Equal(t, 50, args[1])
			return &fakeRows{rows: [][]any{
				{
					"public", "orders", int64(1000), int64(5),
					int64(2000), int64(100), int64(10),
					int64(3), int64(500),
					vacuum, nil, vacuum, nil,
					int64(1048576),
				},
			}}, nil
		},
	}
	task := &tableStatsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "public", data["schema"])
	assert.Equal(t, 1, data["count"])

	tables := data["tables"].([]TableStat)
	require.Len(t, tables, 1)
	assert.Equal(t, "orders", tables[0].RelName)
	assert.Equal(t, int64(1000), tables[0].LiveTuples)
	assert.Equal(t, int64(5), tables[0].DeadTuples)
	assert.Equal(t, int64(1048576), tables[0].TotalSizeBytes)
	require.NotNil(t, tables[0].LastVacuum)
	assert.Equal(t, vacuum, *tables[0].LastVacuum)
	assert.Nil(t, tables[0].LastAutovacuum)
}

func TestTableStatsTaskCustomSchemaAndLimit(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeTableStatsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &fakeRows{}, nil
		},
	}
	task := &tableStatsTask{client: client}

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

func TestTableStatsTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &tableStatsTask{client: &fakeTableStatsClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestTableStatsTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeTableStatsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &fakeRows{}, nil
		},
	}
	task := &tableStatsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 1000, capturedArgs[1])
}

func TestTableStatsTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakeTableStatsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &tableStatsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestTableStatsTaskScanError(t *testing.T) {
	t.Parallel()

	client := &fakeTableStatsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{rows: [][]any{{"public"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &tableStatsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestTableStatsTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &fakeTableStatsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &tableStatsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}
