package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// walFakeRow is a pgx.Row that copies canned values into scan destinations,
// supporting the nullable types used by the WAL archiver task.
type walFakeRow struct {
	values []any
	err    error
}

func (r *walFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("scan destination count mismatch")
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			*p = r.values[i].(string)
		case *int64:
			*p = r.values[i].(int64)
		case **string:
			if r.values[i] == nil {
				*p = nil
			} else {
				v := r.values[i].(string)
				*p = &v
			}
		case **time.Time:
			if r.values[i] == nil {
				*p = nil
			} else {
				v := r.values[i].(time.Time)
				*p = &v
			}
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

// walFakeClient routes QueryRow calls by SQL so tests can control the
// archive_mode probe and the pg_stat_archiver query independently.
type walFakeClient struct {
	archiveModeRow func(ctx context.Context, sql string, args ...any) pgx.Row
	archiverRow    func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (c *walFakeClient) Ping(ctx context.Context) error { return nil }

func (c *walFakeClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (c *walFakeClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if sql == archiveModeQuery {
		if c.archiveModeRow == nil {
			return &walFakeRow{err: errors.New("no archiveModeRow configured")}
		}
		return c.archiveModeRow(ctx, sql, args...)
	}
	if c.archiverRow == nil {
		return &walFakeRow{err: errors.New("no archiverRow configured")}
	}
	return c.archiverRow(ctx, sql, args...)
}

func (c *walFakeClient) Close() error { return nil }

func (c *walFakeClient) Guardrails() config.PostgresGuardrails {
	return config.PostgresGuardrails{}
}

func TestWalArchiverTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &walArchiverTask{}
	assert.Equal(t, "postgres.wal.archiver", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestWalArchiverTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &walArchiverTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestWalArchiverTaskArchivingDisabled(t *testing.T) {
	t.Parallel()

	client := &walFakeClient{
		archiveModeRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{values: []any{"off"}}
		},
	}
	task := &walArchiverTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["archiving_enabled"])
	assert.Equal(t, "WAL archiving not enabled", data["message"])
}

func TestWalArchiverTaskSuccess(t *testing.T) {
	t.Parallel()

	archivedAt := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	failedAt := time.Date(2024, 6, 2, 11, 0, 0, 0, time.UTC)
	resetAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	client := &walFakeClient{
		archiveModeRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{values: []any{"on"}}
		},
		archiverRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{values: []any{
				int64(42),
				"000000010000000000000001",
				archivedAt,
				int64(2),
				"000000010000000000000002",
				failedAt,
				resetAt,
			}}
		},
	}
	task := &walArchiverTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["archiving_enabled"])
	assert.Equal(t, int64(42), data["archived_count"])
	assert.Equal(t, int64(2), data["failed_count"])

	lastArchivedWAL := data["last_archived_wal"].(*string)
	require.NotNil(t, lastArchivedWAL)
	assert.Equal(t, "000000010000000000000001", *lastArchivedWAL)

	lastArchivedTime := data["last_archived_time"].(*time.Time)
	require.NotNil(t, lastArchivedTime)
	assert.Equal(t, archivedAt, *lastArchivedTime)

	lastFailedWAL := data["last_failed_wal"].(*string)
	require.NotNil(t, lastFailedWAL)
	assert.Equal(t, "000000010000000000000002", *lastFailedWAL)

	lastFailedTime := data["last_failed_time"].(*time.Time)
	require.NotNil(t, lastFailedTime)
	assert.Equal(t, failedAt, *lastFailedTime)

	statsReset := data["stats_reset"].(*time.Time)
	require.NotNil(t, statsReset)
	assert.Equal(t, resetAt, *statsReset)
}

func TestWalArchiverTaskSuccessNullFields(t *testing.T) {
	t.Parallel()

	client := &walFakeClient{
		archiveModeRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{values: []any{"always"}}
		},
		archiverRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{values: []any{
				int64(0), nil, nil, int64(0), nil, nil, nil,
			}}
		},
	}
	task := &walArchiverTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["archiving_enabled"])
	assert.Nil(t, data["last_archived_wal"])
	assert.Nil(t, data["last_archived_time"])
	assert.Nil(t, data["last_failed_wal"])
	assert.Nil(t, data["last_failed_time"])
	assert.Nil(t, data["stats_reset"])
}

func TestWalArchiverTaskArchiveModeError(t *testing.T) {
	t.Parallel()

	client := &walFakeClient{
		archiveModeRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{err: errors.New("permission denied")}
		},
	}
	task := &walArchiverTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "permission denied")
}

func TestWalArchiverTaskArchiverQueryError(t *testing.T) {
	t.Parallel()

	client := &walFakeClient{
		archiveModeRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{values: []any{"on"}}
		},
		archiverRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &walFakeRow{err: errors.New("relation does not exist")}
		},
	}
	task := &walArchiverTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "relation does not exist")
}
