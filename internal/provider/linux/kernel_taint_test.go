package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadKernelTaintClean(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "sys", "kernel"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "sys", "kernel", "tainted"), []byte("0\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelTaint(r)
	require.NoError(t, err)

	assert.Equal(t, uint64(0), data["tainted"])
	assert.Equal(t, 0, data["flag_count"])
	assert.Equal(t, false, data["is_tainted"])
}

func TestReadKernelTaintWithFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "sys", "kernel"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "sys", "kernel", "tainted"), []byte("3\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelTaint(r)
	require.NoError(t, err)

	assert.Equal(t, uint64(3), data["tainted"])
	assert.Equal(t, 2, data["flag_count"])
	assert.Equal(t, true, data["is_tainted"])

	flags, ok := data["flags"].([]string)
	require.True(t, ok)
	assert.Contains(t, flags, "proprietary_module")
	assert.Contains(t, flags, "non_gpl_module")
}
