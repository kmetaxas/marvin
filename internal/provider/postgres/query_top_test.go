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

// fakeQueryTopRows implements pgx.Rows for query top tests.
type fakeQueryTopRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *fakeQueryTopRows) Close()                                       { r.closed = true }
func (r *fakeQueryTopRows) Err() error                                   { return r.rowErr }
func (r *fakeQueryTopRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeQueryTopRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeQueryTopRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeQueryTopRows) RawValues() [][]byte                          { return nil }
func (r *fakeQueryTopRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeQueryTopRows) TypeMap() *pgtype.Map                         { return nil }

func (r *fakeQueryTopRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeQueryTopRows) Scan(dest ...any) error {
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
		case *int:
			*p = values[i].(int)
		case *int64:
			*p = values[i].(int64)
		case *float64:
			*p = values[i].(float64)
		case **string:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(string)
				*p = &v
			}
		case **int:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(int)
				*p = &v
			}
		case **int64:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(int64)
				*p = &v
			}
		case **float64:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(float64)
				*p = &v
			}
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

type fakeQueryTopClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *fakeQueryTopClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}

func TestQueryTopTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &queryTopTask{}
	assert.Equal(t, "postgres.query.top", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestQueryTopTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &queryTopTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestQueryTopTaskPgStatStatementsSuccess(t *testing.T) {
	t.Parallel()

	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			// First call: pg_stat_statements query
			require.Len(t, args, 1)
			require.Equal(t, 10, args[0])
			require.Contains(t, sql, "pg_stat_statements")
			return &fakeQueryTopRows{rows: [][]any{
				{int64(1), "SELECT 1", int64(100), float64(500.0), float64(5.0), int64(1000), int64(50), int64(10)},
				{int64(2), "INSERT INTO users", int64(50), float64(200.0), float64(4.0), int64(50), int64(30), int64(5)},
			}}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "pg_stat_statements", data["source"])
	assert.Equal(t, 2, data["count"])

	queries := data["queries"].([]queryTopResult)
	require.Len(t, queries, 2)
	assert.Equal(t, "SELECT 1", queries[0].Query)
	require.NotNil(t, queries[0].QueryID)
	assert.Equal(t, int64(1), *queries[0].QueryID)
	require.NotNil(t, queries[0].Calls)
	assert.Equal(t, int64(100), *queries[0].Calls)
	require.NotNil(t, queries[0].TotalExecTime)
	assert.Equal(t, 500.0, *queries[0].TotalExecTime)
	require.NotNil(t, queries[0].MeanExecTime)
	assert.Equal(t, 5.0, *queries[0].MeanExecTime)
	require.NotNil(t, queries[0].Rows)
	assert.Equal(t, int64(1000), *queries[0].Rows)
	require.NotNil(t, queries[0].SharedBlksHit)
	assert.Equal(t, int64(50), *queries[0].SharedBlksHit)
	require.NotNil(t, queries[0].SharedBlksRead)
	assert.Equal(t, int64(10), *queries[0].SharedBlksRead)
}

func TestQueryTopTaskPgStatStatementsSortByCalls(t *testing.T) {
	t.Parallel()

	var capturedSQL string
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			require.Len(t, args, 1)
			require.Equal(t, 5, args[0])
			return &fakeQueryTopRows{rows: [][]any{
				{int64(3), "CALL proc()", int64(1000), float64(100.0), float64(0.1), int64(0), int64(0), int64(0)},
			}}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{
		"limit":   5,
		"sort_by": "calls",
	})
	require.NoError(t, err)
	require.True(t, res.Success)

	require.Contains(t, capturedSQL, "ORDER BY calls DESC")
	data := res.Data.(map[string]any)
	queries := data["queries"].([]queryTopResult)
	require.Len(t, queries, 1)
	assert.Equal(t, "CALL proc()", queries[0].Query)
}

func TestQueryTopTaskPgStatStatementsSortByMeanTime(t *testing.T) {
	t.Parallel()

	var capturedSQL string
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			return &fakeQueryTopRows{rows: [][]any{}}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"sort_by": "mean_time"})
	require.NoError(t, err)
	require.True(t, res.Success)
	require.Contains(t, capturedSQL, "ORDER BY mean_exec_time DESC")
}

func TestQueryTopTaskPgStatStatementsSortByRows(t *testing.T) {
	t.Parallel()

	var capturedSQL string
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			return &fakeQueryTopRows{rows: [][]any{}}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"sort_by": "rows"})
	require.NoError(t, err)
	require.True(t, res.Success)
	require.Contains(t, capturedSQL, "ORDER BY rows DESC")
}

func TestQueryTopTaskFallbackToPgStatActivity(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			callCount++
			if callCount == 1 {
				// pg_stat_statements fails
				return nil, errors.New("relation \"pg_stat_statements\" does not exist")
			}
			// pg_stat_activity fallback
			require.Len(t, args, 1)
			require.Equal(t, 10, args[0])
			require.Contains(t, sql, "pg_stat_activity")
			return &fakeQueryTopRows{rows: [][]any{
				{int(42), "SELECT * FROM users", "active", float64(1500.0), "appuser", "mydb", "myapp", "10.0.0.1"},
			}}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "pg_stat_activity", data["source"])
	assert.Equal(t, 1, data["count"])

	queries := data["queries"].([]queryTopResult)
	require.Len(t, queries, 1)
	assert.Equal(t, "SELECT * FROM users", queries[0].Query)
	require.NotNil(t, queries[0].PID)
	assert.Equal(t, int(42), *queries[0].PID)
	require.NotNil(t, queries[0].State)
	assert.Equal(t, "active", *queries[0].State)
	require.NotNil(t, queries[0].DurationMs)
	assert.Equal(t, 1500.0, *queries[0].DurationMs)
	require.NotNil(t, queries[0].UserName)
	assert.Equal(t, "appuser", *queries[0].UserName)
	require.NotNil(t, queries[0].DatabaseName)
	assert.Equal(t, "mydb", *queries[0].DatabaseName)
	require.NotNil(t, queries[0].ApplicationName)
	assert.Equal(t, "myapp", *queries[0].ApplicationName)
	require.NotNil(t, queries[0].ClientAddr)
	assert.Equal(t, "10.0.0.1", *queries[0].ClientAddr)
}

func TestQueryTopTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &queryTopTask{client: &fakeQueryTopClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestQueryTopTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &fakeQueryTopRows{rows: [][]any{}}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	require.Len(t, capturedArgs, 1)
	assert.Equal(t, 1000, capturedArgs[0])
}

func TestQueryTopTaskPgStatStatementsQueryError(t *testing.T) {
	t.Parallel()

	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestQueryTopTaskPgStatStatementsScanError(t *testing.T) {
	t.Parallel()

	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeQueryTopRows{rows: [][]any{{"bad"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestQueryTopTaskPgStatStatementsRowsError(t *testing.T) {
	t.Parallel()

	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeQueryTopRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}

func TestQueryTopTaskPgStatActivityQueryError(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("pg_stat_statements missing")
			}
			return nil, errors.New("activity query failed")
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "activity query failed")
}

func TestQueryTopTaskPgStatActivityScanError(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("pg_stat_statements missing")
			}
			return &fakeQueryTopRows{rows: [][]any{{"bad"}}, scanErr: errors.New("activity scan failed")}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "activity scan failed")
}

func TestQueryTopTaskPgStatActivityRowsError(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("pg_stat_statements missing")
			}
			return &fakeQueryTopRows{rowErr: errors.New("activity rows failed")}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "activity rows failed")
}

func TestQueryTopTaskSortByInvalid(t *testing.T) {
	t.Parallel()

	var capturedSQL string
	client := &fakeQueryTopClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			return &fakeQueryTopRows{rows: [][]any{}}, nil
		},
	}
	task := &queryTopTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"sort_by": "invalid"})
	require.NoError(t, err)
	require.True(t, res.Success)
	// Invalid sort_by defaults to total_exec_time
	require.Contains(t, capturedSQL, "ORDER BY total_exec_time DESC")
}
