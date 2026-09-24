package linux

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesystemListDirectorySuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bb"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths:    []string{dir},
		MaxDirectoryEntries: 100,
		MaxDirectoryDepth:   3,
	})

	task := &filesystemListDirectoryTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": dir})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	entries := data["entries"].([]map[string]any)
	assert.Len(t, entries, 3)
	assert.Equal(t, 3, data["count"])
}

func TestFilesystemListDirectoryPathValidation(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{"/etc"},
	})

	task := &filesystemListDirectoryTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": "/tmp"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not in allowed read paths")
}

func TestFilesystemListDirectoryNotADirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("x"), 0644))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{dir},
	})

	task := &filesystemListDirectoryTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": testFile})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "is not a directory")
}

func TestFilesystemListDirectoryMaxEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.Base(filepath.Join(dir, string(rune('a'+i))+".txt"))), []byte("x"), 0644))
	}
	// Create files a.txt, b.txt, c.txt, d.txt, e.txt
	for i := 0; i < 5; i++ {
		name := string(rune('a'+i)) + ".txt"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths:    []string{dir},
		MaxDirectoryEntries: 3,
		MaxDirectoryDepth:   1,
	})

	task := &filesystemListDirectoryTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": dir})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 3, data["count"])
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 5, data["total"])
}
