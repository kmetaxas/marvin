package linux

import (
	"context"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdUnitShowTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service", "[Unit]\nDescription=SSH Server\nAfter=network.target\n\n[Service]\nExecStart=/usr/sbin/sshd\n")

	p := &Provider{reader: &procfs.Reader{Root: dir}}
	task := &systemdUnitShowTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "sshd.service"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "sshd.service", data["unit"])
	props := data["properties"].(map[string]any)
	assert.Equal(t, "SSH Server", props["Unit.Description"])
	assert.Equal(t, "network.target", props["Unit.After"])
	assert.Equal(t, "/usr/sbin/sshd", props["Service.ExecStart"])
}

func TestSystemdUnitShowTaskMissingUnit(t *testing.T) {
	t.Parallel()

	p := &Provider{reader: &procfs.Reader{Root: t.TempDir()}}
	task := &systemdUnitShowTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
}

func TestSystemdUnitShowTaskNotFound(t *testing.T) {
	t.Parallel()

	p := &Provider{reader: &procfs.Reader{Root: t.TempDir()}}
	task := &systemdUnitShowTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "missing.service"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
