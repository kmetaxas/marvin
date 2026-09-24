package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadKernelSysctlGetSingle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "sys", "kernel"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "sys", "kernel", "ostype"), []byte("Linux\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelSysctlGet(r, []string{"kernel.ostype"})
	require.NoError(t, err)

	values, ok := data["values"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "Linux", values["kernel.ostype"])
	assert.Equal(t, 1, data["count"])
	assert.Empty(t, data["failures"])
}

func TestReadKernelSysctlGetMultiple(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "sys", "kernel"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "sys", "net", "core"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "sys", "kernel", "hostname"), []byte("testhost\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "sys", "net", "core", "somaxconn"), []byte("4096\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelSysctlGet(r, []string{"kernel.hostname", "net.core.somaxconn"})
	require.NoError(t, err)

	values, ok := data["values"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "testhost", values["kernel.hostname"])
	assert.Equal(t, "4096", values["net.core.somaxconn"])
	assert.Equal(t, 2, data["count"])
}

func TestReadKernelSysctlGetMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "sys", "kernel"), 0755))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelSysctlGet(r, []string{"kernel.nosuchkey"})
	require.NoError(t, err)

	values, ok := data["values"].(map[string]string)
	require.True(t, ok)
	assert.Empty(t, values)
	assert.Equal(t, 0, data["count"])
	failures, ok := data["failures"].([]string)
	require.True(t, ok)
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0], "kernel.nosuchkey")
}

func TestReadKernelSysctlGetInvalidKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	data, err := readKernelSysctlGet(r, []string{"kernel/../../etc/passwd"})
	require.NoError(t, err)

	values, ok := data["values"].(map[string]string)
	require.True(t, ok)
	assert.Empty(t, values)
	failures, ok := data["failures"].([]string)
	require.True(t, ok)
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0], "invalid key")
}
