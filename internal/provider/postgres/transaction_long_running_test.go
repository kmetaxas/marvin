package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transactionLongRunningRows implements pgx.Rows for long-running transaction tests.
type transactionLongRunningRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *transactionLongRunningRows) Close()                                       { r.closed = true }
func (r *transactionLongRunningRows) Err() error                                   { return r.rowErr }
func (r *transactionLongRunningRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *transactionLongRunningRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *transactionLongRunningRows) Values() ([]any, error)                       { return nil, nil }
func (r *transactionLongRunningRows) RawValues() [][]byte                          { return nil }
func (r *transactionLongRunningRows) Conn() *pgx.Conn                              { return nil }
func (r *transactionLongRunningRows) TypeMap() *pgtype.Map                         { return nil }

func (r *transactionLongRunningRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *transactionLongRunningRows) Scan(dest ...any) error {
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

// fakeTransactionLongRunningClient is a narrow client whose Query returns canned rows.
type fakeTransactionLongRunningClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *fakeTransactionLongRunningClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}

func TestTransactionLongRunningTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &transactionLongRunningTask{}
	assert.Equal(t, "postgres.transaction.long_running", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTransactionLongRunningTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &transactionLongRunningTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestTransactionLongRunningTaskSuccess(t *testing.T) {
	t.Parallel()

	xactStart := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	queryStart := time.Date(2024, 6, 1, 10, 5, 0, 0, time.UTC)

	client := &fakeTransactionLongRunningClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			require.Len(t, args, 2)
			require.Equal(t, 50, args[0])
			require.Equal(t, 5*time.Minute, args[1])
			require.Contains(t, sql, "pg_stat_activity")
			return &transactionLongRunningRows{rows: [][]any{
				{
					int32(1234), "appuser", "myapp", "active",
					xactStart, queryStart, float64(600.5), "SELECT * FROM orders",
				},
			}}, nil
		},
	}
	task := &transactionLongRunningTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "5m", data["min_duration"])
	assert.Equal(t, 50, data["limit"])
	assert.Equal(t, 1, data["count"])

	txs := data["transactions"].([]LongRunningTransaction)
	require.Len(t, txs, 1)
	assert.Equal(t, int32(1234), txs[0].PID)
	assert.Equal(t, "appuser", txs[0].UserName)
	assert.Equal(t, "myapp", txs[0].ApplicationName)
	assert.Equal(t, "active", txs[0].State)
	require.NotNil(t, txs[0].XactStart)
	assert.Equal(t, xactStart, *txs[0].XactStart)
	require.NotNil(t, txs[0].QueryStart)
	assert.Equal(t, queryStart, *txs[0].QueryStart)
	assert.Equal(t, float64(600.5), txs[0].DurationSeconds)
	assert.Equal(t, "SELECT * FROM orders", txs[0].Query)
}

func TestTransactionLongRunningTaskCustomParams(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeTransactionLongRunningClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &transactionLongRunningRows{}, nil
		},
	}
	task := &transactionLongRunningTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{
		"min_duration": "30s",
		"limit":        10,
	})
	require.NoError(t, err)
	require.True(t, res.Success)

	require.Len(t, capturedArgs, 2)
	assert.Equal(t, 10, capturedArgs[0])
	assert.Equal(t, 30*time.Second, capturedArgs[1])

	data := res.Data.(map[string]any)
	assert.Equal(t, "30s", data["min_duration"])
	assert.Equal(t, 10, data["limit"])
	assert.Equal(t, 0, data["count"])
}

func TestTransactionLongRunningTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &transactionLongRunningTask{client: &fakeTransactionLongRunningClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestTransactionLongRunningTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeTransactionLongRunningClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &transactionLongRunningRows{}, nil
		},
	}
	task := &transactionLongRunningTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 1000, capturedArgs[0])
}

func TestTransactionLongRunningTaskInvalidDuration(t *testing.T) {
	t.Parallel()

	task := &transactionLongRunningTask{client: &fakeTransactionLongRunningClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"min_duration": "not-a-duration"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "min_duration must be a valid duration")
}

func TestTransactionLongRunningTaskNonPositiveDuration(t *testing.T) {
	t.Parallel()

	task := &transactionLongRunningTask{client: &fakeTransactionLongRunningClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"min_duration": "0s"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "min_duration must be greater than 0")
}

func TestTransactionLongRunningTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakeTransactionLongRunningClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &transactionLongRunningTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestTransactionLongRunningTaskScanError(t *testing.T) {
	t.Parallel()

	client := &fakeTransactionLongRunningClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &transactionLongRunningRows{rows: [][]any{{"only-one"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &transactionLongRunningTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestTransactionLongRunningTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &fakeTransactionLongRunningClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &transactionLongRunningRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &transactionLongRunningTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}

func TestTransactionLongRunningTaskTruncatesQuery(t *testing.T) {
	t.Parallel()

	longQuery := strings.Repeat("a", 300)
	client := &fakeTransactionLongRunningClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &transactionLongRunningRows{rows: [][]any{
				{
					int32(1), "u", "app", "active",
					nil, nil, float64(1), longQuery,
				},
			}}, nil
		},
	}
	task := &transactionLongRunningTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	txs := res.Data.(map[string]any)["transactions"].([]LongRunningTransaction)
	require.Len(t, txs, 1)
	assert.Len(t, txs[0].Query, 256)
	assert.Equal(t, strings.Repeat("a", 256), txs[0].Query)
}

func TestTransactionLongRunningTaskQueryContainsExpectedFilters(t *testing.T) {
	t.Parallel()

	assert.Contains(t, transactionLongRunningQuery, "pg_stat_activity")
	assert.Contains(t, transactionLongRunningQuery, "xact_start IS NOT NULL")
	assert.Contains(t, transactionLongRunningQuery, "state <> 'idle'")
	assert.Contains(t, transactionLongRunningQuery, "ORDER BY xact_start")
	assert.Contains(t, transactionLongRunningQuery, "LIMIT $1")
	assert.Contains(t, transactionLongRunningQuery, "$2")
}
