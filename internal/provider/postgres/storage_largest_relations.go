package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*storageLargestRelationsTask)(nil)

// storageLargestRelationsClient is the narrow client surface required by the
// largest relations task. It is satisfied by PostgresClient and can be faked in
// tests without a live database.
type storageLargestRelationsClient interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// storageLargestRelationsBaseQuery returns the largest relations (tables,
// indexes, and toast tables) in a schema by total on-disk size.
const storageLargestRelationsBaseQuery = `SELECT n.nspname as schema_name, c.relname as relation_name, c.relkind as relation_type, pg_total_relation_size(c.oid) as total_size_bytes, pg_relation_size(c.oid) as table_size_bytes, pg_indexes_size(c.oid) as index_size_bytes, pg_total_relation_size(c.reltoastrelid) as toast_size_bytes, pg_size_pretty(pg_total_relation_size(c.oid)) as size_pretty FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relkind IN ('r', 'i', 't')`

// storageLargestRelationsTablesOnlyFilter restricts results to ordinary tables
// when indexes are excluded.
const storageLargestRelationsTablesOnlyFilter = ` AND c.relkind = 'r'`

// storageLargestRelationsOrderLimit orders by total size descending and limits
// the number of rows returned.
const storageLargestRelationsOrderLimit = ` ORDER BY pg_total_relation_size(c.oid) DESC LIMIT $2`

// LargestRelation holds size information for a single relation.
type LargestRelation struct {
	SchemaName     string `json:"schema_name"`
	RelationName   string `json:"relation_name"`
	RelationType   string `json:"relation_type"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
	TableSizeBytes int64  `json:"table_size_bytes"`
	IndexSizeBytes int64  `json:"index_size_bytes"`
	ToastSizeBytes int64  `json:"toast_size_bytes"`
	SizePretty     string `json:"size_pretty"`
}

type storageLargestRelationsTask struct {
	provider *Provider
	client   storageLargestRelationsClient
}

func (t *storageLargestRelationsTask) currentClient() storageLargestRelationsClient {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *storageLargestRelationsTask) Name() string { return "postgres.storage.largest_relations" }

func (t *storageLargestRelationsTask) JSONSchema() string { return storageLargestRelationsSchema }

func (t *storageLargestRelationsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	schema, err := common.OptionalString(params, "schema", "public")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := common.OptionalInt(params, "limit", 50)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than 0"))
	}
	if limit > 1000 {
		limit = 1000
	}
	includeIndexes, err := common.OptionalBool(params, "include_indexes", true)
	if err != nil {
		return common.TaskFailure(err)
	}

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	query := storageLargestRelationsBaseQuery
	if !includeIndexes {
		query += storageLargestRelationsTablesOnlyFilter
	}
	query += storageLargestRelationsOrderLimit

	rows, err := client.Query(ctx, query, schema, limit)
	if err != nil {
		return common.TaskFailure(err)
	}
	defer rows.Close()

	relations := make([]LargestRelation, 0)
	for rows.Next() {
		var r LargestRelation
		if err := rows.Scan(
			&r.SchemaName,
			&r.RelationName,
			&r.RelationType,
			&r.TotalSizeBytes,
			&r.TableSizeBytes,
			&r.IndexSizeBytes,
			&r.ToastSizeBytes,
			&r.SizePretty,
		); err != nil {
			return common.TaskFailure(err)
		}
		relations = append(relations, r)
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"schema":          schema,
		"include_indexes": includeIndexes,
		"relations":       relations,
		"count":           len(relations),
	}), nil
}

const storageLargestRelationsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Largest Relations Parameters",
  "description": "Find largest tables and indexes by on-disk size.",
  "properties": {
    "schema": {
      "type": "string",
      "default": "public",
      "description": "Schema name to inspect."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 50,
      "description": "Maximum number of relations to return, ordered by total size descending."
    },
    "include_indexes": {
      "type": "boolean",
      "default": true,
      "description": "Include indexes and toast tables in addition to ordinary tables."
    }
  }
}`
