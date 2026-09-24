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

func TestFilesystemStatSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{dir},
	})

	task := &filesystemStatTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": testFile})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, testFile, data["path"])
	assert.Equal(t, true, data["exists"])
	assert.Equal(t, int64(11), data["size"])
	assert.Equal(t, false, data["is_dir"])
	assert.Equal(t, true, data["is_regular"])
}

func TestFilesystemStatPathValidation(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{"/etc"},
	})

	task := &filesystemStatTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": "/tmp/secret.txt"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not in allowed read paths")
}

func TestFilesystemStatMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{dir},
	})

	task := &filesystemStatTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": filepath.Join(dir, "missing.txt")})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "stat path:")
}
