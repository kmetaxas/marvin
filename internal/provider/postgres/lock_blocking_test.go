package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockBlockingRows implements pgx.Rows for lock blocking tests.
type lockBlockingRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *lockBlockingRows) Close()                                       { r.closed = true }
func (r *lockBlockingRows) Err() error                                   { return r.rowErr }
func (r *lockBlockingRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *lockBlockingRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *lockBlockingRows) Values() ([]any, error)                       { return nil, nil }
func (r *lockBlockingRows) RawValues() [][]byte                          { return nil }
func (r *lockBlockingRows) Conn() *pgx.Conn                              { return nil }
func (r *lockBlockingRows) TypeMap() *pgtype.Map                         { return nil }
func (r *lockBlockingRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *lockBlockingRows) Scan(dest ...any) error {
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
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

// lockBlockingClient is a PostgresClient whose Query returns canned rows.
type lockBlockingClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *lockBlockingClient) Ping(ctx context.Context) error { return nil }
func (c *lockBlockingClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}
func (c *lockBlockingClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &errorRow{err: errors.New("not used")}
}
func (c *lockBlockingClient) Close() error { return nil }
func (c *lockBlockingClient) Guardrails() config.PostgresGuardrails {
	return config.PostgresGuardrails{}
}
func (c *lockBlockingClient) Stat() PoolStats { return PoolStats{} }

func TestLockBlockingTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &lockBlockingTask{}
	assert.Equal(t, "postgres.lock.blocking", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestLockBlockingTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &lockBlockingTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestLockBlockingTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &lockBlockingClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			require.Equal(t, 1, len(args))
			require.Equal(t, 50, args[0])
			return &lockBlockingRows{rows: [][]any{
				{
					int32(100), "blocked_user", "blocked_app",
					int32(200), "blocking_user", "blocking_app",
					"SELECT * FROM foo", "UPDATE foo SET x = 1",
					"AccessShareLock", "relation", float64(1234.5),
				},
			}}, nil
		},
	}
	task := &lockBlockingTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 50, data["limit"])
	assert.Equal(t, 1, data["count"])

	locks := data["locks"].([]BlockingLock)
	require.Len(t, locks, 1)
	assert.Equal(t, int32(100), locks[0].BlockedPID)
	assert.Equal(t, "blocked_user", locks[0].BlockedUser)
	assert.Equal(t, "blocked_app", locks[0].BlockedApplication)
	assert.Equal(t, int32(200), locks[0].BlockingPID)
	assert.Equal(t, "blocking_user", locks[0].BlockingUser)
	assert.Equal(t, "blocking_app", locks[0].BlockingApplication)
	assert.Equal(t, "SELECT * FROM foo", locks[0].BlockedStatement)
	assert.Equal(t, "UPDATE foo SET x = 1", locks[0].BlockingStatement)
	assert.Equal(t, "AccessShareLock", locks[0].LockMode)
	assert.Equal(t, "relation", locks[0].LockType)
	assert.Equal(t, float64(1234.5), locks[0].DurationMs)
}

func TestLockBlockingTaskCustomLimit(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &lockBlockingClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &lockBlockingRows{}, nil
		},
	}
	task := &lockBlockingTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 10})
	require.NoError(t, err)
	require.True(t, res.Success)

	assert.Equal(t, 10, capturedArgs[0])
	data := res.Data.(map[string]any)
	assert.Equal(t, 10, data["limit"])
	assert.Equal(t, 0, data["count"])
}

func TestLockBlockingTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &lockBlockingTask{client: &lockBlockingClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestLockBlockingTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &lockBlockingClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &lockBlockingRows{}, nil
		},
	}
	task := &lockBlockingTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 1000, capturedArgs[0])
}

func TestLockBlockingTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &lockBlockingClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &lockBlockingTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestLockBlockingTaskScanError(t *testing.T) {
	t.Parallel()

	client := &lockBlockingClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &lockBlockingRows{rows: [][]any{{"only-one"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &lockBlockingTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestLockBlockingTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &lockBlockingClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &lockBlockingRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &lockBlockingTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}

func TestLockBlockingTaskTruncateQuery(t *testing.T) {
	t.Parallel()

	longQuery := strings.Repeat("a", 300)
	client := &lockBlockingClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &lockBlockingRows{rows: [][]any{
				{
					int32(100), "u1", "app1",
					int32(200), "u2", "app2",
					longQuery, longQuery,
					"AccessShareLock", "relation", float64(0),
				},
			}}, nil
		},
	}
	task := &lockBlockingTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	locks := res.Data.(map[string]any)["locks"].([]BlockingLock)
	require.Len(t, locks, 1)
	assert.Len(t, locks[0].BlockedStatement, 256)
	assert.Len(t, locks[0].BlockingStatement, 256)
	assert.Equal(t, strings.Repeat("a", 256), locks[0].BlockedStatement)
}

func TestLockBlockingTaskQueryContainsExpectedJoins(t *testing.T) {
	t.Parallel()

	assert.Contains(t, lockBlockingQuery, "blocked_locks.pid")
	assert.Contains(t, lockBlockingQuery, "blocking_locks.pid")
	assert.Contains(t, lockBlockingQuery, "pg_catalog.pg_locks")
	assert.Contains(t, lockBlockingQuery, "pg_catalog.pg_stat_activity")
	assert.Contains(t, lockBlockingQuery, "NOT blocked_locks.granted")
	assert.Contains(t, lockBlockingQuery, "blocking_locks.granted")
	assert.Contains(t, lockBlockingQuery, "LIMIT $1")
}
