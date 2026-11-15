package journal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	sdjournal "github.com/netdata/systemd-journal-sdk/go/journal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testMachineID = mustParseUUID("0123456789abcdef0123456789abcdef")
	testBootID    = mustParseUUID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1")
)

// fixtureEntry describes one journal entry to append in a test fixture.
type fixtureEntry struct {
	message  string
	priority string // empty: entry carries no PRIORITY field
	unit     string // empty: entry carries no _SYSTEMD_UNIT field
	realtime uint64 // microseconds
}

func mustParseUUID(s string) sdjournal.UUID {
	id, err := sdjournal.ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// writeJournal generates a real journal file with the SDK writer.
func writeJournal(t *testing.T, path string, bootID sdjournal.UUID, entries []fixtureEntry) {
	t.Helper()

	w, err := sdjournal.Create(path, sdjournal.Options{
		MachineID: testMachineID,
		BootID:    bootID,
	})
	require.NoError(t, err)

	for i, fe := range entries {
		fields := []sdjournal.Field{sdjournal.StringField("MESSAGE", fe.message)}
		if fe.priority != "" {
			fields = append(fields, sdjournal.StringField("PRIORITY", fe.priority))
		}
		if fe.unit != "" {
			fields = append(fields, sdjournal.StringField("_SYSTEMD_UNIT", fe.unit))
		}
		require.NoError(t, w.Append(fields, sdjournal.EntryOptions{
			RealtimeUsec:  fe.realtime,
			MonotonicUsec: uint64(i+1) * 1000,
		}))
	}
	require.NoError(t, w.CloseOffline())
}

func TestQueryUnitTail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var fixture []fixtureEntry
	base := uint64(1_700_000_000_000_000)
	units := []string{"alpha.service", "beta.service", "gamma.service"}
	priorities := []string{"0", "1", "2", "3", "4", "5", "6", "7"}
	for i := 0; i < 30; i++ {
		fixture = append(fixture, fixtureEntry{
			message:  fmt.Sprintf("entry-%02d", i),
			priority: priorities[i%len(priorities)],
			unit:     units[i%len(units)],
			realtime: base + uint64(i)*1_000_000,
		})
	}
	writeJournal(t, filepath.Join(dir, "system.journal"), testBootID, fixture)

	r, err := Open([]string{dir})
	require.NoError(t, err)
	defer r.Close()

	entries, err := r.Query(context.Background(), Query{
		MatchUnit:   "beta.service",
		PriorityMax: -1,
		Limit:       10,
	})
	require.NoError(t, err)
	require.Len(t, entries, 10)

	for i, e := range entries {
		assert.Equal(t, "beta.service", e.Fields["_SYSTEMD_UNIT"], "entry %d", i)
		assert.NotEmpty(t, e.Fields["MESSAGE"], "entry %d", i)
		assert.NotEmpty(t, e.Fields["PRIORITY"], "entry %d", i)
		assert.NotEmpty(t, e.Cursor, "entry %d", i)
		assert.Equal(t, time.UnixMicro(int64(e.Realtime)).UTC(), e.Timestamp, "entry %d", i)
		if i > 0 {
			assert.Greater(t, entries[i-1].Realtime, e.Realtime, "entries must be newest-first")
		}
	}

	// beta.service entries are fixture indexes 1,4,...,28; newest-first order
	// means the first result is index 28 and the last is index 1.
	assert.Equal(t, fixture[28].message, entries[0].Fields["MESSAGE"])
	assert.Equal(t, fixture[1].message, entries[9].Fields["MESSAGE"])
}

func TestQuerySinceUntil(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	base := uint64(1_700_000_000_000_000)
	var fixture []fixtureEntry
	for i := 0; i < 20; i++ {
		fixture = append(fixture, fixtureEntry{
			message:  fmt.Sprintf("entry-%02d", i),
			priority: "6",
			realtime: base + uint64(i)*1_000_000,
		})
	}
	writeJournal(t, filepath.Join(dir, "system.journal"), testBootID, fixture)

	r, err := Open([]string{dir})
	require.NoError(t, err)
	defer r.Close()

	since := time.UnixMicro(int64(base + 5_000_000)).UTC()
	until := time.UnixMicro(int64(base + 10_000_000)).UTC()
	entries, err := r.Query(context.Background(), Query{
		PriorityMax: -1,
		Since:       &since,
		Until:       &until,
		Limit:       100,
	})
	require.NoError(t, err)

	// Window [base+5s, base+10s] selects indexes 5..10, newest-first.
	require.Len(t, entries, 6)
	for i, offset := range []uint64{10, 9, 8, 7, 6, 5} {
		assert.Equal(t, base+offset*1_000_000, entries[i].Realtime, "entry %d", i)
		assert.Equal(t, fixture[offset].message, entries[i].Fields["MESSAGE"], "entry %d", i)
	}
}

func TestQueryPriorityCeiling(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	base := uint64(1_700_000_000_000_000)
	fixture := []fixtureEntry{
		{message: "p3", priority: "3", realtime: base},
		{message: "p4", priority: "4", realtime: base + 1_000_000},
		{message: "nopri", realtime: base + 2_000_000},
		{message: "p6", priority: "6", realtime: base + 3_000_000},
		{message: "p0", priority: "0", realtime: base + 4_000_000},
	}
	writeJournal(t, filepath.Join(dir, "system.journal"), testBootID, fixture)

	r, err := Open([]string{dir})
	require.NoError(t, err)
	defer r.Close()

	entries, err := r.Query(context.Background(), Query{
		PriorityMax: 3,
		Limit:       10,
	})
	require.NoError(t, err)

	// Only PRIORITY<=3 with a present PRIORITY field: p0 (newest) and p3.
	require.Len(t, entries, 2)
	assert.Equal(t, "p0", entries[0].Fields["MESSAGE"])
	assert.Equal(t, "p3", entries[1].Fields["MESSAGE"])
	for _, e := range entries {
		assert.NotEqual(t, "nopri", e.Fields["MESSAGE"], "entries without PRIORITY must be excluded")
	}
}

func TestQueryEmptyDirs(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "nonexistent")
	r, err := Open([]string{missing})
	require.NoError(t, err)
	require.NoError(t, r.Close())

	entries, err := r.Query(context.Background(), Query{PriorityMax: -1, Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, entries)

	// Zero-value Reader must be safe to close and query.
	var zero Reader
	assert.NoError(t, zero.Close())
	entries, err = zero.Query(context.Background(), Query{PriorityMax: -1, Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestBoots(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	base := uint64(1_700_000_000_000_000)
	bootID := mustParseUUID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2")
	fixture := []fixtureEntry{
		{message: "first", priority: "6", realtime: base},
		{message: "middle", priority: "6", realtime: base + 1_000_000},
		{message: "last", priority: "6", realtime: base + 2_000_000},
	}
	writeJournal(t, filepath.Join(dir, "system.journal"), bootID, fixture)

	r, err := Open([]string{dir})
	require.NoError(t, err)
	defer r.Close()

	boots, err := r.Boots(context.Background())
	require.NoError(t, err)
	require.Len(t, boots, 1)

	assert.Equal(t, bootID.String(), boots[0].ID)
	assert.Equal(t, time.UnixMicro(int64(base)).UTC(), boots[0].FirstEntry)
	assert.Equal(t, time.UnixMicro(int64(base+2_000_000)).UTC(), boots[0].LastEntry)
}

func TestOpenSkipsBadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	base := uint64(1_700_000_000_000_000)
	fixture := []fixtureEntry{
		{message: "good-1", priority: "6", realtime: base},
		{message: "good-2", priority: "6", realtime: base + 1_000_000},
	}
	validPath := filepath.Join(dir, "system.journal")
	writeJournal(t, validPath, testBootID, fixture)

	// Copy the valid journal and truncate the copy so OpenFile fails on it
	// deterministically (header is smaller than 16 bytes).
	data, err := os.ReadFile(validPath)
	require.NoError(t, err)
	badPath := filepath.Join(dir, "bad.journal")
	require.NoError(t, os.WriteFile(badPath, data, 0644))
	require.NoError(t, os.Truncate(badPath, 8))

	r, err := Open([]string{dir})
	require.NoError(t, err)
	defer r.Close()

	entries, err := r.Query(context.Background(), Query{PriorityMax: -1, Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "good-2", entries[0].Fields["MESSAGE"])
	assert.Equal(t, "good-1", entries[1].Fields["MESSAGE"])
}

func TestQueryContextCanceled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeJournal(t, filepath.Join(dir, "system.journal"), testBootID, []fixtureEntry{
		{message: "m", priority: "6", realtime: 1_700_000_000_000_000},
	})

	r, err := Open([]string{dir})
	require.NoError(t, err)
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = r.Query(ctx, Query{PriorityMax: -1, Limit: 10})
	assert.ErrorIs(t, err, context.Canceled)

	_, err = r.Boots(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}
