package linux

import (
	"context"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdUnitDependenciesTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service", `[Unit]
Description=SSH Server
After=network.target auditd.service
Before=shutdown.target
Requires=sshd-keygen.service
Wants=systemd-logind.service
Conflicts=rescue.service
Requisite=network-online.target
`)

	p := &Provider{reader: &procfs.Reader{Root: dir}}
	task := &systemdUnitDependenciesTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "sshd.service"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "sshd.service", data["unit"])
	assert.Equal(t, []string{"network.target", "auditd.service"}, data["after"])
	assert.Equal(t, []string{"shutdown.target"}, data["before"])
	assert.Equal(t, []string{"sshd-keygen.service"}, data["requires"])
	assert.Equal(t, []string{"systemd-logind.service"}, data["wants"])
	assert.Equal(t, []string{"rescue.service"}, data["conflicts"])
	assert.Equal(t, []string{"network-online.target"}, data["requisite"])
}

func TestSystemdUnitDependenciesTaskCommaSeparated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/foo.service", "[Unit]\nWants=a.service,b.service, c.service\n")

	p := &Provider{reader: &procfs.Reader{Root: dir}}
	task := &systemdUnitDependenciesTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "foo.service"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, []string{"a.service", "b.service", "c.service"}, data["wants"])
}

func TestSystemdUnitDependenciesTaskEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/foo.service", "[Unit]\nDescription=No deps\n")

	p := &Provider{reader: &procfs.Reader{Root: dir}}
	task := &systemdUnitDependenciesTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "foo.service"})
	require.NoError(t, err)
	require.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Empty(t, data["after"])
	assert.Empty(t, data["wants"])
}

func TestSystemdUnitDependenciesTaskNotFound(t *testing.T) {
	t.Parallel()

	p := &Provider{reader: &procfs.Reader{Root: t.TempDir()}}
	task := &systemdUnitDependenciesTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"unit": "missing.service"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}

func TestSplitDependencyList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"a", "b", "c"}, splitDependencyList("a, b, c"))
	assert.Equal(t, []string{"a", "b"}, splitDependencyList("a b"))
	assert.Equal(t, []string{"a"}, splitDependencyList("a, a, a"))
	assert.Empty(t, splitDependencyList(""))
	assert.Empty(t, splitDependencyList("  , , "))
}
