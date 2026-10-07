package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*walArchiverTask)(nil)

// walArchiverTask reports WAL archiving statistics from pg_stat_archiver.
// It never enables archiving; it only observes the current state.
type walArchiverTask struct {
	provider *Provider
	client   PostgresClient
}

func (t *walArchiverTask) currentClient() PostgresClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}

func (t *walArchiverTask) Name() string { return "postgres.wal.archiver" }

func (t *walArchiverTask) JSONSchema() string { return walArchiverSchema }

// archiveModeQuery reads the archive_mode GUC. Values are 'off', 'on', or
// 'always'. Archiving is only considered enabled when it is not 'off'.
const archiveModeQuery = `SELECT current_setting('archive_mode')`

// walArchiverQuery returns the single row of archiver statistics.
const walArchiverQuery = `SELECT archived_count, last_archived_wal, last_archived_time, failed_count, last_failed_wal, last_failed_time, stats_reset FROM pg_stat_archiver`

func (t *walArchiverTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = params

	client := t.currentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("postgres client is not configured"))
	}

	var archiveMode string
	if err := client.QueryRow(ctx, archiveModeQuery).Scan(&archiveMode); err != nil {
		return common.TaskFailure(fmt.Errorf("query archive_mode: %w", err))
	}
	if archiveMode == "off" {
		return common.SuccessResult(map[string]any{
			"archiving_enabled": false,
			"message":           "WAL archiving not enabled",
		}), nil
	}

	var (
		archivedCount    int64
		lastArchivedWAL  *string
		lastArchivedTime *time.Time
		failedCount      int64
		lastFailedWAL    *string
		lastFailedTime   *time.Time
		statsReset       *time.Time
	)

	if err := client.QueryRow(ctx, walArchiverQuery).Scan(
		&archivedCount,
		&lastArchivedWAL,
		&lastArchivedTime,
		&failedCount,
		&lastFailedWAL,
		&lastFailedTime,
		&statsReset,
	); err != nil {
		return common.TaskFailure(fmt.Errorf("query pg_stat_archiver: %w", err))
	}

	return common.SuccessResult(map[string]any{
		"archiving_enabled":  true,
		"archived_count":     archivedCount,
		"last_archived_wal":  lastArchivedWAL,
		"last_archived_time": lastArchivedTime,
		"failed_count":       failedCount,
		"last_failed_wal":    lastFailedWAL,
		"last_failed_time":   lastFailedTime,
		"stats_reset":        statsReset,
	}), nil
}

const walArchiverSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "PostgreSQL WAL Archiver Parameters",
  "description": "Check WAL archiving status and last archived WAL.",
  "properties": {}
}`
