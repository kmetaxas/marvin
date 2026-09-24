package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFirewalldStatusInstalledAndRunning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "usr", "bin"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "run", "firewalld"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "etc", "firewalld", "zones"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "usr", "bin", "firewalld"), []byte(""), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run", "firewalld", "firewalld.pid"), []byte("1234\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etc", "firewalld", "firewalld.conf"),
		[]byte("# comment\nDefaultZone=public\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etc", "firewalld", "zones", "public.xml"), []byte("<zone/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etc", "firewalld", "zones", "internal.xml"), []byte("<zone/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etc", "firewalld", "zones", "README"), []byte(""), 0644))

	r := &procfs.Reader{Root: dir}
	status, err := readFirewalldStatus(r)
	require.NoError(t, err)

	assert.Equal(t, true, status["installed"])
	assert.Equal(t, true, status["running"])
	assert.Equal(t, 1234, status["pid"])
	assert.Equal(t, "public", status["default_zone"])
	assert.Equal(t, []string{"internal", "public"}, status["zones"])
	assert.Equal(t, "unknown", status["version"])
}

func TestReadFirewalldStatusNotInstalled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}

	status, err := readFirewalldStatus(r)
	require.NoError(t, err)

	assert.Equal(t, false, status["installed"])
	assert.Equal(t, false, status["running"])
	assert.Equal(t, 0, status["pid"])
	assert.Equal(t, "", status["default_zone"])
	assert.Empty(t, status["zones"])
}

func TestReadFirewalldDefaultZoneMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "etc", "firewalld"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etc", "firewalld", "firewalld.conf"),
		[]byte("# only a comment\n"), 0644))

	r := &procfs.Reader{Root: dir}
	assert.Equal(t, "", readFirewalldDefaultZone(r))
}
