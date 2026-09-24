package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListJournalBoots(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	machineDir := filepath.Join(dir, "var", "log", "journal", "machineid123")
	require.NoError(t, os.MkdirAll(machineDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(machineDir, "boot1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(machineDir, "boot2"), 0755))

	r := &procfs.Reader{Root: dir}
	boots, err := listJournalBoots(r, "/var/log/journal")
	require.NoError(t, err)

	ids := make(map[string]bool)
	for _, b := range boots {
		ids[b.ID] = true
		assert.Equal(t, "/var/log/journal", b.Source)
	}
	assert.True(t, ids["boot1"])
	assert.True(t, ids["boot2"])
	assert.Len(t, boots, 2)
}

func TestListJournalBootsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	boots, err := listJournalBoots(r, "/nonexistent")
	require.NoError(t, err)
	assert.Empty(t, boots)
}

func TestListJournalBootsDefaultDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	boots, err := listJournalBoots(r, "")
	require.NoError(t, err)
	assert.Empty(t, boots)
}
