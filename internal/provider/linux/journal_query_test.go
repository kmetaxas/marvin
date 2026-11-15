package linux

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	sdjournal "github.com/netdata/systemd-journal-sdk/go/journal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListJournalFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	machineDir := filepath.Join(dir, "var", "log", "journal", "machineid123")
	require.NoError(t, os.MkdirAll(machineDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "system.journal"), []byte("data"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "user-1000.journal"), []byte("data"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "system@boot1.journal~"), []byte("data"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "not-a-journal.txt"), []byte("data"), 0644))

	r := &procfs.Reader{Root: dir}
	files, err := listJournalFiles(r, "/var/log/journal", "")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, f := range files {
		names[f.Name] = true
		assert.True(t, f.Size > 0)
		assert.NotEmpty(t, f.Path)
	}
	assert.True(t, names["system.journal"])
	assert.True(t, names["user-1000.journal"])
	assert.True(t, names["system@boot1.journal~"])
	assert.False(t, names["not-a-journal.txt"])
	assert.Len(t, files, 3)
}

func TestListJournalFilesServiceFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	machineDir := filepath.Join(dir, "var", "log", "journal", "machineid123")
	require.NoError(t, os.MkdirAll(machineDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "system.journal"), []byte("data"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "user-1000.journal"), []byte("data"), 0644))

	r := &procfs.Reader{Root: dir}
	files, err := listJournalFiles(r, "/var/log/journal", "user")
	require.NoError(t, err)

	require.Len(t, files, 1)
	assert.Equal(t, "user-1000.journal", files[0].Name)
}

func TestListJournalFilesEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	files, err := listJournalFiles(r, "/nonexistent", "")
	require.NoError(t, err)
	assert.Empty(t, files)
}

// --- Shared journal fixture helpers (SDK writer generates real journals) ---

const (
	testJournalMachineDir = "0123456789abcdef0123456789abcdef"
	testJournalMachineID  = "0123456789abcdef0123456789abcdef"
	testJournalBootID     = "aaaaaaa1aaaaaaa2aaaaaaa3aaaaaaa4"
)

// testJournalEntry describes one fixture journal entry.
type testJournalEntry struct {
	message  string
	priority string // empty: entry carries no PRIORITY field
	unit     string // empty: entry carries no _SYSTEMD_UNIT field
	realtime uint64 // microseconds, fixed and deterministic
}

func mustTestUUID(t *testing.T, s string) sdjournal.UUID {
	t.Helper()
	id, err := sdjournal.ParseUUID(s)
	require.NoError(t, err)
	return id
}

// writeTestJournal generates a real journal file with the SDK writer. The
// MonotonicUsec value must be non-zero, so a deterministic sequence is used.
func writeTestJournal(t *testing.T, path string, bootID sdjournal.UUID, entries []testJournalEntry) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	w, err := sdjournal.Create(path, sdjournal.Options{
		MachineID: mustTestUUID(t, testJournalMachineID),
		BootID:    bootID,
	})
	require.NoError(t, err)

	for i, te := range entries {
		fields := []sdjournal.Field{sdjournal.StringField("MESSAGE", te.message)}
		if te.priority != "" {
			fields = append(fields, sdjournal.StringField("PRIORITY", te.priority))
		}
		if te.unit != "" {
			fields = append(fields, sdjournal.StringField("_SYSTEMD_UNIT", te.unit))
		}
		require.NoError(t, w.Append(fields, sdjournal.EntryOptions{
			RealtimeUsec:  te.realtime,
			MonotonicUsec: uint64(i+1) * 1000,
		}))
	}
	require.NoError(t, w.CloseOffline())
}

// writeTestJournalAtMachineDir writes the fixture under <dir>/<machine-id>/.
func writeTestJournalAtMachineDir(t *testing.T, dir string, bootID sdjournal.UUID, entries []testJournalEntry) {
	t.Helper()
	writeTestJournal(t, filepath.Join(dir, testJournalMachineDir, "system.journal"), bootID, entries)
}

func testJournalProvider(dir string) *Provider {
	return &Provider{
		reader: procfs.DefaultReader,
		config: config.LinuxConfig{JournalDirectory: dir},
	}
}

func TestJournalQueryTask(t *testing.T) {
	t.Parallel()

	const (
		alpha = "marvin-fixture-alpha.service"
		beta  = "marvin-fixture-beta.service"
	)
	base := uint64(1_700_000_000_000_000) // fixed usec, not "now"
	fixture := []testJournalEntry{
		{message: "e00", priority: "3", unit: alpha, realtime: base},
		{message: "e01", priority: "4", unit: beta, realtime: base + 1_000_000},
		{message: "e02", priority: "6", unit: alpha, realtime: base + 2_000_000},
		{message: "e03", priority: "3", unit: alpha, realtime: base + 3_000_000},
		{message: "e04", unit: alpha, realtime: base + 4_000_000}, // no PRIORITY
		{message: "e05", priority: "2", unit: beta, realtime: base + 5_000_000},
		{message: "e06", priority: "0", unit: alpha, realtime: base + 6_000_000},
		{message: "e07", priority: "3", unit: beta, realtime: base + 7_000_000},
		{message: "e08", priority: "3", unit: alpha, realtime: base + 8_000_000},
		{message: "e09", priority: "7", unit: alpha, realtime: base + 9_000_000},
		{message: "e10", priority: "1", unit: alpha, realtime: base + 10_000_000},
		{message: "e11", priority: "3", unit: beta, realtime: base + 11_000_000},
	}
	dir := t.TempDir()
	writeTestJournalAtMachineDir(t, dir, mustTestUUID(t, testJournalBootID), fixture)

	task := &journalQueryTask{provider: testJournalProvider(dir)}
	sinceStr := time.UnixMicro(int64(base + 3_000_000)).UTC().Format(time.RFC3339)
	res, err := task.Execute(context.Background(), map[string]any{
		"service":  alpha,
		"priority": "err",
		"since":    sinceStr,
	})
	require.NoError(t, err)
	require.True(t, res.Success, "error: %v", res.Error)

	data := res.Data.(map[string]any)
	assert.Equal(t, alpha, data["service"])
	assert.Equal(t, "err", data["priority"])
	assert.Equal(t, sinceStr, data["since"])
	assert.Equal(t, "", data["until"])
	assert.Equal(t, 50, data["limit"])
	assert.Equal(t, 4, data["count"])
	_, hasNote := data["note"]
	assert.False(t, hasNote, "result must not contain the legacy note key")

	// alpha + PRIORITY present and <= 3 + realtime >= since:
	// e03, e06, e08, e10; newest first.
	entries := data["entries"].([]JournalEntry)
	require.Len(t, entries, 4)
	for i, want := range []string{"e10", "e08", "e06", "e03"} {
		assert.Equal(t, want, entries[i].Message, "entry %d", i)
		assert.Equal(t, alpha, entries[i].Unit, "entry %d", i)
		assert.NotEmpty(t, entries[i].Priority, "entry %d", i)
		assert.Equal(t, want, entries[i].Fields["MESSAGE"], "entry %d", i)
		assert.Equal(t, alpha, entries[i].Fields["_SYSTEMD_UNIT"], "entry %d", i)
	}
	assert.Equal(t, "1", entries[0].Priority)
	assert.Equal(t,
		time.UnixMicro(int64(base+10_000_000)).UTC().Format(time.RFC3339),
		entries[0].Timestamp)

	// listJournalFiles filters by file name, so the "files" listing is only
	// environment-dependent; just sanity check its shape.
	files := data["files"].([]journalFile)
	for _, f := range files {
		assert.NotEmpty(t, f.Name)
		assert.NotEmpty(t, f.Path)
	}
}

func TestJournalQueryTaskInvalidParams(t *testing.T) {
	t.Parallel()

	task := &journalQueryTask{provider: testJournalProvider(t.TempDir())}

	for name, params := range map[string]map[string]any{
		"unknown priority": {"priority": "bogus"},
		"bad since":        {"since": "not-a-timestamp"},
		"bad until":        {"until": "2026-13-45"},
		"zero limit":       {"limit": 0},
		"string limit":     {"limit": "10"},
	} {
		res, err := task.Execute(context.Background(), params)
		require.NoError(t, err)
		assert.False(t, res.Success, "case %q must fail", name)
	}
}
