package procfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReaderPath(t *testing.T) {
	t.Parallel()
	r := &Reader{Root: "/tmp/testroot"}
	assert.Equal(t, "/tmp/testroot/proc/uptime", r.Path("proc", "uptime"))
}

func TestReaderReadFileString(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0644))

	r := &Reader{Root: dir}
	content, err := r.ReadFileString("hello.txt")
	require.NoError(t, err)
	assert.Equal(t, "world", content)
}

func TestReaderReadFileLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lines.txt"), []byte("line1\nline2\nline3\n"), 0644))

	r := &Reader{Root: dir}
	lines, err := r.ReadFileLines("lines.txt")
	require.NoError(t, err)
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
}

func TestReaderReadDirNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir1"), 0755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir2"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644))

	r := &Reader{Root: dir}
	names, err := r.ReadDirNames()
	require.NoError(t, err)
	assert.Len(t, names, 3)
	assert.Contains(t, names, "subdir1")
	assert.Contains(t, names, "subdir2")
	assert.Contains(t, names, "file.txt")
}

func TestReaderExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("x"), 0644))

	r := &Reader{Root: dir}
	assert.True(t, r.Exists("exists.txt"))
	assert.False(t, r.Exists("missing.txt"))
}

func TestReaderIsDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644))

	r := &Reader{Root: dir}
	assert.True(t, r.IsDir("subdir"))
	assert.False(t, r.IsDir("file.txt"))
}

func TestReaderReadInt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "int.txt"), []byte("42\n"), 0644))

	r := &Reader{Root: dir}
	val, err := r.ReadInt("int.txt")
	require.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestReaderReadUint64(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "uint.txt"), []byte("18446744073709551615\n"), 0644))

	r := &Reader{Root: dir}
	val, err := r.ReadUint64("uint.txt")
	require.NoError(t, err)
	assert.Equal(t, uint64(18446744073709551615), val)
}

func TestDefaultReader(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/", DefaultReader.Root)
}
