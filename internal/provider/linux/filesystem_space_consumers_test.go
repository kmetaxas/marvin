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

func TestFilesystemSpaceConsumersSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "small.txt"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "large.txt"), []byte("large content here"), 0644))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths:  []string{dir},
		MaxDirectoryDepth: 3,
	})

	task := &filesystemSpaceConsumersTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": dir, "limit": 10})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	consumers := data["consumers"].([]consumer)
	assert.Len(t, consumers, 2)
	// Should be sorted by size descending
	assert.True(t, consumers[0].Size >= consumers[1].Size)
}

func TestFilesystemSpaceConsumersPathValidation(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{"/etc"},
	})

	task := &filesystemSpaceConsumersTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": "/tmp"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not in allowed read paths")
}

func TestFilesystemSpaceConsumersNotADirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("x"), 0644))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{dir},
	})

	task := &filesystemSpaceConsumersTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": testFile})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "is not a directory")
}
