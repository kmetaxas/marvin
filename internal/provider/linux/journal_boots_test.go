package linux

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJournalBootsTask(t *testing.T) {
	t.Parallel()

	const bootIDHex = "dddddddddddddddddddddddddddddddd"
	base := uint64(1_700_000_000_000_000) // fixed usec, not "now"
	fixture := []testJournalEntry{
		{message: "first", priority: "6", realtime: base},
		{message: "middle", priority: "6", realtime: base + 1_000_000},
		{message: "last", priority: "6", realtime: base + 2_000_000},
	}
	dir := t.TempDir()
	writeTestJournalAtMachineDir(t, dir, mustTestUUID(t, bootIDHex), fixture)

	task := &journalBootsTask{provider: testJournalProvider(dir)}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success, "error: %v", res.Error)

	data := res.Data.(map[string]any)
	boots := data["boots"].([]journalBoot)
	assert.Equal(t, len(boots), data["count"])

	var found *journalBoot
	for i := range boots {
		if boots[i].ID == mustTestUUID(t, bootIDHex).String() {
			found = &boots[i]
		}
	}
	require.NotNil(t, found, "fixture boot must be listed")
	assert.Equal(t, time.UnixMicro(int64(base)).UTC().Format(time.RFC3339), found.FirstEntry)
	assert.Equal(t, time.UnixMicro(int64(base+2_000_000)).UTC().Format(time.RFC3339), found.LastEntry)
	assert.Empty(t, found.Source)
}

func TestJournalBootsTaskEmpty(t *testing.T) {
	t.Parallel()

	// The task also scans /run/log/journal, so a live host journald would
	// contribute boots; only assert the empty case when it cannot.
	if hostRuntimeJournalsExist() {
		t.Skip("host /run/log/journal contains journals; empty-result assertion is environment-dependent")
	}

	task := &journalBootsTask{provider: testJournalProvider(t.TempDir())}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, res.Success, "error: %v", res.Error)

	data := res.Data.(map[string]any)
	assert.Empty(t, data["boots"].([]journalBoot))
	assert.Equal(t, 0, data["count"])
}

// hostRuntimeJournalsExist reports whether /run/log/journal contains journal
// data on the machine running the tests.
func hostRuntimeJournalsExist() bool {
	entries, err := os.ReadDir("/run/log/journal")
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasSuffix(name, ".journal") || strings.HasSuffix(name, ".journal~") {
			return true
		}
	}
	return false
}
