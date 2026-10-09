package postgres

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*replicationStatusTask)(nil)

// ReplicationConnection represents a single replication connection on a primary.
type ReplicationConnection struct {
	ApplicationName string `json:"application_name"`
	ClientAddr      string `json:"client_addr"`
	State           string `json:"state"`
	SentLsn         string `json:"sent_lsn"`
	WriteLsn        string `json:"write_lsn"`
	FlushLsn        string `json:"flush_lsn"`
	ReplayLsn       string `json:"replay_lsn"`
	WriteLag        string `json:"write_lag"`
	FlushLag        string `json:"flush_lag"`
	ReplayLag       string `json:"replay_lag"`
}

// WALReceiverInfo represents the WAL receiver status on a replica.
type WALReceiverInfo struct {
	ConnInfo      string `json:"conninfo"`
	ReceivedLsn   string `json:"received_lsn"`
	LatestEndLsn  string `json:"latest_end_lsn"`
	LatestEndTime string `json:"latest_end_time"`
	SenderHost    string `json:"sender_host"`
	SenderPort    int32  `json:"sender_port"`
}

const replicationStatusIsRecoveryQuery = `SELECT pg_is_in_recovery()`

const replicationStatusPrimaryQuery = `SELECT
    application_name,
    COALESCE(client_addr::text, ''),
    state,
    COALESCE(sent_lsn::text, ''),
    COALESCE(write_lsn::text, ''),
    COALESCE(flush_lsn::text, ''),
    COALESCE(replay_lsn::text, ''),
    COALESCE(write_lag::text, ''),
    COALESCE(flush_lag::text, ''),
    COALESCE(replay_lag::text, '')
FROM pg_stat_replication`

const replicationStatusReplicaQuery = `SELECT
    conninfo,
    COALESCE(received_lsn::text, ''),
    COALESCE(latest_end_lsn::text, ''),
    COALESCE(latest_end_time::text, ''),
    COALESCE(sender_host, ''),
    COALESCE(sender_port, 0)
FROM pg_stat_wal_receiver`

type replicationStatusTask struct {
	provider *Provider
	client   PostgresClient
}

func (t *replicationStatusTask) currentClient() PostgresClient {
	if t.client != nil {
		return t.client
	}
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return nil
}

func (t *replicationStatusTask) Name() string { return "postgres.replication.status" }

func (t *replicationStatusTask) JSONSchema() string {
	return `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL Replication Status Parameters",
  "description": "Return streaming replication lag and status.",
  "properties": {}
}`
}

func (t *replicationStatusTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = params

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	var isInRecovery bool
	if err := client.QueryRow(ctx, replicationStatusIsRecoveryQuery).Scan(&isInRecovery); err != nil {
		return common.TaskFailure(fmt.Errorf("query pg_is_in_recovery: %w", err))
	}

	if !isInRecovery {
		// Primary: query pg_stat_replication
		rows, err := client.Query(ctx, replicationStatusPrimaryQuery)
		if err != nil {
			return common.TaskFailure(fmt.Errorf("query pg_stat_replication: %w", err))
		}
		defer rows.Close()

		replicas := make([]ReplicationConnection, 0)
		for rows.Next() {
			var r ReplicationConnection
			if err := rows.Scan(
				&r.ApplicationName,
				&r.ClientAddr,
				&r.State,
				&r.SentLsn,
				&r.WriteLsn,
				&r.FlushLsn,
				&r.ReplayLsn,
				&r.WriteLag,
				&r.FlushLag,
				&r.ReplayLag,
			); err != nil {
				return common.TaskFailure(fmt.Errorf("scan pg_stat_replication: %w", err))
			}
			replicas = append(replicas, r)
		}
		if err := rows.Err(); err != nil {
			return common.TaskFailure(fmt.Errorf("iterate pg_stat_replication: %w", err))
		}

		if len(replicas) == 0 {
			return common.SuccessResult(map[string]any{
				"replication_enabled": false,
				"message":             "Replication not configured",
			}), nil
		}

		return common.SuccessResult(map[string]any{
			"replication_enabled": true,
			"role":                "primary",
			"replica_count":       len(replicas),
			"replicas":            replicas,
		}), nil
	}

	// Replica: query pg_stat_wal_receiver
	rows, err := client.Query(ctx, replicationStatusReplicaQuery)
	if err != nil {
		return common.TaskFailure(fmt.Errorf("query pg_stat_wal_receiver: %w", err))
	}
	defer rows.Close()

	var walReceiver WALReceiverInfo
	var found bool
	for rows.Next() {
		if err := rows.Scan(
			&walReceiver.ConnInfo,
			&walReceiver.ReceivedLsn,
			&walReceiver.LatestEndLsn,
			&walReceiver.LatestEndTime,
			&walReceiver.SenderHost,
			&walReceiver.SenderPort,
		); err != nil {
			return common.TaskFailure(fmt.Errorf("scan pg_stat_wal_receiver: %w", err))
		}
		found = true
		break
	}
	if err := rows.Err(); err != nil {
		return common.TaskFailure(fmt.Errorf("iterate pg_stat_wal_receiver: %w", err))
	}

	if !found {
		return common.SuccessResult(map[string]any{
			"replication_enabled": false,
			"message":             "Replication not configured",
		}), nil
	}

	return common.SuccessResult(map[string]any{
		"replication_enabled": true,
		"role":                "replica",
		"wal_receiver":        walReceiver,
	}), nil
}
