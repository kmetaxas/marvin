package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFailedServices(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "run", "systemd", "failed"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sys", "fs", "cgroup", "system.slice"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "1", "comm"), []byte("systemd\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run", "systemd", "failed", "foo.service"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run", "systemd", "failed", "bar.service"), []byte(""), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sys", "fs", "cgroup", "system.slice", "baz.service"), 0755))

	r := &procfs.Reader{Root: dir}
	failed, err := listFailedServices(r)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, f := range failed {
		names[f.Name] = true
		assert.Equal(t, "failed", f.State)
	}
	assert.True(t, names["foo.service"])
	assert.True(t, names["bar.service"])
	assert.True(t, names["baz.service"])
}

func TestListFailedServicesNotSystemd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "1"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "1", "comm"), []byte("init\n"), 0644))

	r := &procfs.Reader{Root: dir}
	failed, err := listFailedServices(r)
	require.NoError(t, err)
	assert.Empty(t, failed)
}

func TestListFailedServicesNoSystemd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	failed, err := listFailedServices(r)
	require.NoError(t, err)
	assert.Empty(t, failed)
}

func TestIsSystemdInit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "1"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "1", "comm"), []byte("systemd\n"), 0644))

	r := &procfs.Reader{Root: dir}
	assert.True(t, isSystemdInit(r))
}
