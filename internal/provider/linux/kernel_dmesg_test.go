package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadKernelDmesg(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dev"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dev", "kmsg"), []byte("msg1\nmsg2\nmsg3\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelDmesg(r, 100, "")
	require.NoError(t, err)

	messages, ok := data["messages"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"msg1", "msg2", "msg3"}, messages)
	assert.Equal(t, 3, data["count"])
	assert.Equal(t, "/dev/kmsg", data["source"])
}

func TestReadKernelDmesgLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dev"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dev", "kmsg"), []byte("a\nb\nc\nd\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelDmesg(r, 2, "")
	require.NoError(t, err)

	messages, ok := data["messages"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, messages)
	assert.Equal(t, 2, data["count"])
}

func TestReadKernelDmesgFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dev"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dev", "kmsg"), []byte("error here\ninfo there\nerror again\n"), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelDmesg(r, 100, "error")
	require.NoError(t, err)

	messages, ok := data["messages"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"error here", "error again"}, messages)
	assert.Equal(t, 2, data["count"])
}

func TestReadKernelDmesgMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	_, err := readKernelDmesg(r, 100, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/dev/kmsg")
}
