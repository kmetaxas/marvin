package linux

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdUnitLogsTaskTail(t *testing.T) {
	t.Parallel()

	const (
		alpha = "marvin-fixture-alpha.service"
		beta  = "marvin-fixture-beta.service"
		gamma = "marvin-fixture-gamma.service"
	)
	base := uint64(1_700_000_000_000_000) // fixed usec, not "now"
	units := []string{alpha, beta, gamma}
	var fixture []testJournalEntry
	for i := 0; i < 30; i++ {
		fixture = append(fixture, testJournalEntry{
			message:  fmt.Sprintf("entry-%02d", i),
			priority: fmt.Sprintf("%d", i%8),
			unit:     units[i%3],
			realtime: base + uint64(i)*1_000_000,
		})
	}
	dir := t.TempDir()
	writeTestJournalAtMachineDir(t, dir, mustTestUUID(t, testJournalBootID), fixture)

	task := &systemdUnitLogsTask{provider: testJournalProvider(dir)}
	res, err := task.Execute(context.Background(), map[string]any{"unit": alpha, "limit": 4})
	require.NoError(t, err)
	require.True(t, res.Success, "error: %v", res.Error)

	data := res.Data.(map[string]any)
	assert.Equal(t, alpha, data["unit"])
	assert.Equal(t, 4, data["count"])
	_, hasNote := data["note"]
	assert.False(t, hasNote, "result must not contain the legacy note key")

	// alpha entries are fixture indexes 0,3,...,27; tail returns the newest
	// 4 first: 27, 24, 21, 18.
	entries := data["entries"].([]JournalEntry)
	require.Len(t, entries, 4)
	for i, idx := range []int{27, 24, 21, 18} {
		assert.Equal(t, fmt.Sprintf("entry-%02d", idx), entries[i].Message, "entry %d", i)
		assert.Equal(t, alpha, entries[i].Unit, "entry %d", i)
		assert.Equal(t, fmt.Sprintf("%d", idx%8), entries[i].Priority, "entry %d", i)
		assert.Equal(t, entries[i].Message, entries[i].Fields["MESSAGE"], "entry %d", i)
	}
}

func TestSystemdUnitLogsTaskLimitCap(t *testing.T) {
	t.Parallel()

	const unit = "marvin-fixture-cap.service"
	base := uint64(1_700_000_000_000_000)
	var fixture []testJournalEntry
	for i := 0; i < 520; i++ {
		fixture = append(fixture, testJournalEntry{
			message:  fmt.Sprintf("entry-%03d", i),
			priority: "6",
			unit:     unit,
			realtime: base + uint64(i)*1_000,
		})
	}
	dir := t.TempDir()
	writeTestJournalAtMachineDir(t, dir, mustTestUUID(t, testJournalBootID), fixture)

	task := &systemdUnitLogsTask{provider: testJournalProvider(dir)}
	res, err := task.Execute(context.Background(), map[string]any{"unit": unit, "limit": 1000})
	require.NoError(t, err)
	require.True(t, res.Success, "error: %v", res.Error)

	data := res.Data.(map[string]any)
	assert.Equal(t, 500, data["count"], "limit must be capped at 500")
	entries := data["entries"].([]JournalEntry)
	require.Len(t, entries, 500)
	assert.Equal(t, "entry-519", entries[0].Message)
	assert.Equal(t, "entry-020", entries[499].Message)
}

func TestSystemdUnitLogsTaskNoFiles(t *testing.T) {
	t.Parallel()

	task := &systemdUnitLogsTask{provider: testJournalProvider(t.TempDir())}
	res, err := task.Execute(context.Background(), map[string]any{"unit": "marvin-fixture-none.service"})
	require.NoError(t, err)
	require.True(t, res.Success, "error: %v", res.Error)

	data := res.Data.(map[string]any)
	assert.Empty(t, data["journal_files"].([]journalFile))
	assert.Empty(t, data["entries"].([]JournalEntry))
	assert.Equal(t, 0, data["count"])
}

func TestSystemdUnitLogsTaskMissingUnit(t *testing.T) {
	t.Parallel()

	task := &systemdUnitLogsTask{provider: testJournalProvider(t.TempDir())}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
}

func TestSystemdUnitLogsTaskBadLimit(t *testing.T) {
	t.Parallel()

	task := &systemdUnitLogsTask{provider: testJournalProvider(t.TempDir())}
	res, err := task.Execute(context.Background(), map[string]any{
		"unit":  "marvin-fixture-alpha.service",
		"limit": -1,
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
