package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLargestRelationsClient struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *fakeLargestRelationsClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.queryFunc(ctx, sql, args...)
}

func TestStorageLargestRelationsTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &storageLargestRelationsTask{}
	assert.Equal(t, "postgres.storage.largest_relations", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStorageLargestRelationsTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &storageLargestRelationsTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStorageLargestRelationsTaskSuccess(t *testing.T) {
	t.Parallel()

	var capturedSQL string
	client := &fakeLargestRelationsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			require.Equal(t, "public", args[0])
			require.Equal(t, 50, args[1])
			return &fakeRows{rows: [][]any{
				{
					"public", "orders", "r",
					int64(1048576), int64(524288), int64(262144), int64(262144),
					"1024 kB",
				},
				{
					"public", "orders_pkey", "i",
					int64(262144), int64(262144), int64(0), int64(0),
					"256 kB",
				},
			}}, nil
		},
	}
	task := &storageLargestRelationsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success)

	assert.Contains(t, capturedSQL, "c.relkind IN ('r', 'i', 't')")
	assert.NotContains(t, capturedSQL, "AND c.relkind = 'r'")

	data := res.Data.(map[string]any)
	assert.Equal(t, "public", data["schema"])
	assert.Equal(t, true, data["include_indexes"])
	assert.Equal(t, 2, data["count"])

	relations := data["relations"].([]LargestRelation)
	require.Len(t, relations, 2)
	assert.Equal(t, "orders", relations[0].RelationName)
	assert.Equal(t, "r", relations[0].RelationType)
	assert.Equal(t, int64(1048576), relations[0].TotalSizeBytes)
	assert.Equal(t, int64(524288), relations[0].TableSizeBytes)
	assert.Equal(t, int64(262144), relations[0].IndexSizeBytes)
	assert.Equal(t, int64(262144), relations[0].ToastSizeBytes)
	assert.Equal(t, "1024 kB", relations[0].SizePretty)
	assert.Equal(t, "orders_pkey", relations[1].RelationName)
	assert.Equal(t, "i", relations[1].RelationType)
}

func TestStorageLargestRelationsTaskCustomSchemaAndLimit(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeLargestRelationsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &fakeRows{}, nil
		},
	}
	task := &storageLargestRelationsTask{client: client}

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

func TestStorageLargestRelationsTaskExcludeIndexes(t *testing.T) {
	t.Parallel()

	var capturedSQL string
	client := &fakeLargestRelationsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			return &fakeRows{}, nil
		},
	}
	task := &storageLargestRelationsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"include_indexes": false})
	require.NoError(t, err)
	require.True(t, res.Success)

	assert.True(t, strings.Contains(capturedSQL, "AND c.relkind = 'r'"))
	assert.True(t, strings.Contains(capturedSQL, "ORDER BY pg_total_relation_size(c.oid) DESC LIMIT $2"))

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["include_indexes"])
}

func TestStorageLargestRelationsTaskLimitValidation(t *testing.T) {
	t.Parallel()

	task := &storageLargestRelationsTask{client: &fakeLargestRelationsClient{}}
	res, err := task.Execute(context.Background(), map[string]any{"limit": 0})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than 0")
}

func TestStorageLargestRelationsTaskLimitClamped(t *testing.T) {
	t.Parallel()

	var capturedArgs []any
	client := &fakeLargestRelationsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedArgs = args
			return &fakeRows{}, nil
		},
	}
	task := &storageLargestRelationsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 5000})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, 1000, capturedArgs[1])
}

func TestStorageLargestRelationsTaskQueryError(t *testing.T) {
	t.Parallel()

	client := &fakeLargestRelationsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	task := &storageLargestRelationsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "connection refused")
}

func TestStorageLargestRelationsTaskScanError(t *testing.T) {
	t.Parallel()

	client := &fakeLargestRelationsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{rows: [][]any{{"public"}}, scanErr: errors.New("scan failed")}, nil
		},
	}
	task := &storageLargestRelationsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "scan failed")
}

func TestStorageLargestRelationsTaskRowsError(t *testing.T) {
	t.Parallel()

	client := &fakeLargestRelationsClient{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{rowErr: errors.New("rows failed")}, nil
		},
	}
	task := &storageLargestRelationsTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "rows failed")
}
