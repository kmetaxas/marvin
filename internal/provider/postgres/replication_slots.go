package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*replicationSlotsTask)(nil)

// replicationSlotsQuerier is the narrow client surface required by the
// replication slots task. It is satisfied by PostgresClient and can be faked in
// tests without a live database.
type replicationSlotsQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// replicationSlotsQuery lists replication slots, optionally filtered by
// slot_type. An empty slot_type returns every slot.
const replicationSlotsQuery = `SELECT slot_name, plugin, slot_type, datoid, database, active, active_pid, xmin, restart_lsn, confirmed_flush_lsn, wal_status, safe_wal_size FROM pg_replication_slots WHERE ($2 = '' OR slot_type = $2) LIMIT $1`

// ReplicationSlot holds the state of a single PostgreSQL replication slot.
type ReplicationSlot struct {
	SlotName          string  `json:"slot_name"`
	Plugin            *string `json:"plugin,omitempty"`
	SlotType          string  `json:"slot_type"`
	DatabaseOID       *uint32 `json:"datoid,omitempty"`
	Database          *string `json:"database,omitempty"`
	Active            bool    `json:"active"`
	ActivePID         *int32  `json:"active_pid,omitempty"`
	Xmin              *uint32 `json:"xmin,omitempty"`
	RestartLSN        *string `json:"restart_lsn,omitempty"`
	ConfirmedFlushLSN *string `json:"confirmed_flush_lsn,omitempty"`
	WalStatus         *string `json:"wal_status,omitempty"`
	SafeWalSize       *int64  `json:"safe_wal_size,omitempty"`
}

type replicationSlotsTask struct {
	provider *Provider
	client   replicationSlotsQuerier
}

func (t *replicationSlotsTask) currentClient() replicationSlotsQuerier {
	if t.provider != nil {
		c := t.provider.CurrentClient()
		if c == nil {
			return nil
		}
		return c
	}
	return t.client
}

func (t *replicationSlotsTask) Name() string { return "postgres.replication.slots" }

func (t *replicationSlotsTask) JSONSchema() string { return replicationSlotsSchema }

func (t *replicationSlotsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	slotType, err := common.OptionalString(params, "slot_type", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	if slotType != "" && slotType != "physical" && slotType != "logical" {
		return common.TaskFailure(fmt.Errorf("parameter slot_type must be one of: physical, logical"))
	}

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	rows, err := client.Query(ctx, replicationSlotsQuery, limit, slotType)
	if err != nil {
		return common.TaskFailure(err)
	}
	defer rows.Close()

	slots := make([]ReplicationSlot, 0)
	for rows.Next() {
		var s ReplicationSlot
		if err := rows.Scan(
			&s.SlotName,
			&s.Plugin,
			&s.SlotType,
			&s.DatabaseOID,
			&s.Database,
			&s.Active,
			&s.ActivePID,
			&s.Xmin,
			&s.RestartLSN,
			&s.ConfirmedFlushLSN,
			&s.WalStatus,
			&s.SafeWalSize,
		); err != nil {
			return common.TaskFailure(err)
		}
		slots = append(slots, s)
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"limit":     limit,
		"slot_type": slotType,
		"slots":     slots,
		"count":     len(slots),
	}), nil
}

const replicationSlotsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Replication Slots Parameters",
  "description": "List replication slots and their consumption status.",
  "properties": {
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 50,
      "description": "Maximum number of replication slots to return."
    },
    "slot_type": {
      "type": "string",
      "enum": ["physical", "logical"],
      "description": "Optional filter by replication slot type. Omit to return all slots."
    }
  }
}`
