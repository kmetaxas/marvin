package linux

import (
	"context"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdUnitDefinitionTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service", "[Unit]\nDescription=SSH Server\n")
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service.d/override.conf", "[Service]\nRestart=always\n")

	p := &Provider{reader: &procfs.Reader{Root: dir}}
	task := &systemdUnitDefinitionTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "sshd.service"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "sshd.service", data["name"])
	assert.Contains(t, data["file_path"], "sshd.service")
	assert.Contains(t, data["content"], "Description=SSH Server")

	dropIns := data["drop_ins"].([]unitDropIn)
	require.Len(t, dropIns, 1)
	assert.Equal(t, "override.conf", dropIns[0].Name)
	assert.Contains(t, dropIns[0].Content, "Restart=always")
}

func TestSystemdUnitDefinitionTaskNoDropIns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service", "[Unit]\nDescription=SSH Server\n")

	p := &Provider{reader: &procfs.Reader{Root: dir}}
	task := &systemdUnitDefinitionTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "sshd.service"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Empty(t, data["drop_ins"].([]unitDropIn))
}

func TestSystemdUnitDefinitionTaskNotFound(t *testing.T) {
	t.Parallel()

	p := &Provider{reader: &procfs.Reader{Root: t.TempDir()}}
	task := &systemdUnitDefinitionTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "missing.service"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
