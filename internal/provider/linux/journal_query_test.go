package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
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
