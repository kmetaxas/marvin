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
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionListRows implements pgx.Rows for session list tests.
type sessionListRows struct {
	rows    [][]any
	idx     int
	scanErr error
	rowErr  error
	closed  bool
}

func (r *sessionListRows) Close()                                       { r.closed = true }
func (r *sessionListRows) Err() error                                   { return r.rowErr }
func (r *sessionListRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *sessionListRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *sessionListRows) Values() ([]any, error)                       { return nil, nil }
func (r *sessionListRows) RawValues() [][]byte                          { return nil }
func (r *sessionListRows) Conn() *pgx.Conn                              { return nil }
func (r *sessionListRows) TypeMap() *pgtype.Map                         { return nil }

func (r *sessionListRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *sessionListRows) Scan(dest ...any) error {
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
		case **string:
			if values[i] == nil {
				*p = nil
			} else {
				v := values[i].(string)
				*p = &v
			}
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

// fakeSessionListClient is a narrow client whose Query returns canned rows.
type fakeSessionListClient struct {
	queryFunc  func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	guardrails config.PostgresGuardrails
}

func (c *fakeSessionListClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}

func (c *fakeSessionListClient) Guardrails() config.PostgresGuardrails {
	return c.guardrails
}

func TestSessionListTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &sessionListTask{}
	assert.Equal(t, "postgres.session.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
	assert.Contains(t, task.JSONSchema(), `"limit"`)
	assert.Contains(t, task.JSONSchema(), `"state"`)
}

func TestSessionListTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &sessionListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestSessionListTaskSuccess(t *testing.T) {
	t.Parallel()

	queryStart := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	xactStart := time.Date(2024, 6, 1, 9, 59, 0, 0, time.UTC)

	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			require.Len(t, args, 1)
			require.Equal(t, 50, args[0])
			require.Contains(t, sql, "pg_stat_activity")
			require.Contains(t, sql, "state <> 'idle'")
			return &sessionListRows{rows: [][]any{
				{
					int32(1234), "appuser", "myapp", "10.0.0.5", "active",
					queryStart, xactStart, "Lock", "relation", "SELECT * FROM orders",
				},
			}}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 50, data["limit"])
	assert.Equal(t, "", data["state"])
	assert.Equal(t, 1, data["count"])

	sessions := data["sessions"].([]Session)
	require.Len(t, sessions, 1)
	assert.Equal(t, int32(1234), sessions[0].PID)
	require.NotNil(t, sessions[0].UserName)
	assert.Equal(t, "appuser", *sessions[0].UserName)
	require.NotNil(t, sessions[0].ApplicationName)
	assert.Equal(t, "myapp", *sessions[0].ApplicationName)
	require.NotNil(t, sessions[0].ClientAddr)
	assert.Equal(t, "10.0.0.5", *sessions[0].ClientAddr)
	require.NotNil(t, sessions[0].State)
	assert.Equal(t, "active", *sessions[0].State)
	require.NotNil(t, sessions[0].QueryStart)
	assert.Equal(t, queryStart, *sessions[0].QueryStart)
	require.NotNil(t, sessions[0].XactStart)
	assert.Equal(t, xactStart, *sessions[0].XactStart)
	require.NotNil(t, sessions[0].WaitEventType)
	assert.Equal(t, "Lock", *sessions[0].WaitEventType)
	require.NotNil(t, sessions[0].WaitEvent)
	assert.Equal(t, "relation", *sessions[0].WaitEvent)
	assert.Equal(t, "SELECT * FROM orders", sessions[0].Query)
}

func TestSessionListTaskStateFilter(t *testing.T) {
	t.Parallel()

	var capturedSQL string
	var capturedArgs []any
	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			capturedArgs = args
			return &sessionListRows{}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"state": "idle in transaction"})
	require.NoError(t, err)
	require.True(t, res.Success)

	assert.Contains(t, capturedSQL, "state = $2")
	assert.NotContains(t, capturedSQL, "state <> 'idle'")
	require.Len(t, capturedArgs, 2)
	assert.Equal(t, 50, capturedArgs[0])
	assert.Equal(t, "idle in transaction", capturedArgs[1])

	data := res.Data.(map[string]any)
	assert.Equal(t, "idle in transaction", data["state"])
}

func TestSessionListTaskCustomLimit(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &sessionListRows{}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 10})
	require.NoError(t, err)
	require.True(t, res.Success)

	require.Len(t, capturedArgs, 1)
	assert.Equal(t, 10, capturedArgs[0])
	assert.Equal(t, 10, res.Data.(map[string]any)["limit"])
}

func TestSessionListTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &sessionListTask{client: &fakeSessionListClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestSessionListTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &sessionListRows{}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 1000, capturedArgs[0])
}

func TestSessionListTaskGuardrailsLimit(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeSessionListClient{
		guardrails: config.PostgresGuardrails{MaxRows: 25},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &sessionListRows{}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 25, capturedArgs[0])
	assert.Equal(t, 25, res.Data.(map[string]any)["limit"])
}

func TestSessionListTaskGuardrailsClamp(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeSessionListClient{
		guardrails: config.PostgresGuardrails{MaxRows: 25},
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &sessionListRows{}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 500})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 25, capturedArgs[0])
}

func TestSessionListTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestSessionListTaskScanError(t *testing.T) {
	t.Parallel()

	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &sessionListRows{rows: [][]any{{"only-one"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestSessionListTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &sessionListRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}

func TestSessionListTaskTruncatesQuery(t *testing.T) {
	t.Parallel()

	longQuery := strings.Repeat("a", 300)
	client := &fakeSessionListClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &sessionListRows{rows: [][]any{
				{
					int32(1), nil, nil, nil, "active",
					nil, nil, nil, nil, longQuery,
				},
			}}, nil
		},
	}
	task := &sessionListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	sessions := res.Data.(map[string]any)["sessions"].([]Session)
	require.Len(t, sessions, 1)
	assert.Len(t, sessions[0].Query, 256)
	assert.Equal(t, strings.Repeat("a", 256), sessions[0].Query)
}

func TestSessionListTaskQueryContainsExpectedFilters(t *testing.T) {
	t.Parallel()

	assert.Contains(t, sessionListBaseQuery, "pg_stat_activity")
	assert.Contains(t, sessionListBaseQuery, "LEFT(query, 256)")
	assert.Contains(t, sessionListActiveFilter, "state IS NOT NULL")
	assert.Contains(t, sessionListActiveFilter, "state <> 'idle'")
	assert.Contains(t, sessionListActiveFilter, "ORDER BY query_start DESC")
	assert.Contains(t, sessionListActiveFilter, "LIMIT $1")
	assert.Contains(t, sessionListStateFilter, "state = $2")
	assert.Contains(t, sessionListStateFilter, "LIMIT $1")
}
