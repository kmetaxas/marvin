package linux

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesystemReadSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{dir},
		MaxReadBytes:     1024,
	})

	task := &filesystemReadTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": testFile})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data, ok := res.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, testFile, data["path"])
	assert.Equal(t, "hello world", data["content"])
	assert.Equal(t, false, data["truncated"])
}

func TestFilesystemReadPathValidation(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{"/etc"},
		MaxReadBytes:     1024,
	})

	task := &filesystemReadTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": "/tmp/secret.txt"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not in allowed read paths")
}

func TestFilesystemReadOffset(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{dir},
		MaxReadBytes:     1024,
	})

	task := &filesystemReadTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": testFile, "offset": 6})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "world", data["content"])
	assert.Equal(t, 6, data["offset"])
}

func TestFilesystemReadMaxBytesTruncation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte(strings.Repeat("a", 100)), 0644))

	p := NewProvider(config.LinuxConfig{
		AllowedReadPaths: []string{dir},
		MaxReadBytes:     50,
	})

	task := &filesystemReadTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"path": testFile})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, strings.Repeat("a", 50), data["content"])
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 50, data["bytes_read"])
}
