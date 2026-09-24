package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadKernelCmdline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "cmdline"), []byte("root=/dev/sda1\x00quiet\x00splash\x00"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelCmdline(r)
	require.NoError(t, err)

	assert.Equal(t, "root=/dev/sda1\x00quiet\x00splash\x00", data["raw"])
	assert.Equal(t, []string{"root=/dev/sda1", "quiet", "splash"}, data["args"])
	assert.Equal(t, 3, data["count"])
}

func TestReadKernelCmdlineSpaceSeparated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "cmdline"), []byte("root=/dev/sda1 quiet splash\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelCmdline(r)
	require.NoError(t, err)

	assert.Equal(t, []string{"root=/dev/sda1", "quiet", "splash"}, data["args"])
}
