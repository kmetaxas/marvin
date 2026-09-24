package linux

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdUnitLogsTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	machineDir := filepath.Join(dir, "var", "log", "journal", "machineid123")
	require.NoError(t, os.MkdirAll(machineDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "system.journal"), []byte("data"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(machineDir, "user-1000.journal"), []byte("data"), 0644))

	p := &Provider{
		reader: &procfs.Reader{Root: dir},
		config: config.LinuxConfig{JournalDirectory: "/var/log/journal"},
	}
	task := &systemdUnitLogsTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "user"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "user", data["unit"])
	files := data["journal_files"].([]journalFile)
	require.Len(t, files, 1)
	assert.Equal(t, "user-1000.journal", files[0].Name)
	assert.NotEmpty(t, data["note"])
}

func TestSystemdUnitLogsTaskNoFiles(t *testing.T) {
	t.Parallel()

	p := &Provider{
		reader: &procfs.Reader{Root: t.TempDir()},
		config: config.LinuxConfig{JournalDirectory: "/var/log/journal"},
	}
	task := &systemdUnitLogsTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "sshd.service"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Empty(t, data["journal_files"].([]journalFile))
}

func TestSystemdUnitLogsTaskMissingUnit(t *testing.T) {
	t.Parallel()

	p := &Provider{
		reader: &procfs.Reader{Root: t.TempDir()},
		config: config.LinuxConfig{JournalDirectory: "/var/log/journal"},
	}
	task := &systemdUnitLogsTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
