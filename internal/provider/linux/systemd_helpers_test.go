package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeUnitFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0644))
}

func TestFindUnitFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service", "[Unit]\nDescription=SSH\n")

	r := &procfs.Reader{Root: dir}
	path, content, err := findUnitFile(r, "sshd.service")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "etc", "systemd", "system", "sshd.service"), path)
	assert.Contains(t, string(content), "Description=SSH")
}

func TestFindUnitFilePrecedence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/foo.service", "[Unit]\nDescription=etc\n")
	writeUnitFile(t, dir, "usr/lib/systemd/system/foo.service", "[Unit]\nDescription=usr\n")

	r := &procfs.Reader{Root: dir}
	path, content, err := findUnitFile(r, "foo.service")
	require.NoError(t, err)
	assert.Contains(t, path, "etc/systemd/system/foo.service")
	assert.Contains(t, string(content), "Description=etc")
}

func TestFindUnitFileNotFound(t *testing.T) {
	t.Parallel()

	r := &procfs.Reader{Root: t.TempDir()}
	_, _, err := findUnitFile(r, "missing.service")
	require.Error(t, err)
}

func TestParseUnitFile(t *testing.T) {
	t.Parallel()

	content := []byte(`# comment
; another comment
[Unit]
Description=Example Service
After=network.target
Wants=foo.service

[Service]
ExecStart=/usr/bin/example --flag
Restart=on-failure
`)
	sections := parseUnitFile(content)

	require.Contains(t, sections, "Unit")
	require.Contains(t, sections, "Service")
	assert.Equal(t, "Example Service", sections["Unit"]["Description"])
	assert.Equal(t, "network.target", sections["Unit"]["After"])
	assert.Equal(t, "foo.service", sections["Unit"]["Wants"])
	assert.Equal(t, "/usr/bin/example --flag", sections["Service"]["ExecStart"])
	assert.Equal(t, "on-failure", sections["Service"]["Restart"])
}

func TestParseUnitFileMultiline(t *testing.T) {
	t.Parallel()

	content := []byte("[Service]\nExecStart=/usr/bin/foo \\\n  --bar \\\n  --baz\n")
	sections := parseUnitFile(content)

	assert.Equal(t, "/usr/bin/foo --bar --baz", sections["Service"]["ExecStart"])
}

func TestListUnitDropIns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service.d/override.conf", "[Service]\nRestart=always\n")
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service.d/10-extra.conf", "[Service]\nTimeoutSec=30\n")
	writeUnitFile(t, dir, "etc/systemd/system/sshd.service.d/notes.txt", "not a conf\n")

	r := &procfs.Reader{Root: dir}
	dropIns := listUnitDropIns(r, "sshd.service")

	require.Len(t, dropIns, 2)
	assert.Equal(t, "10-extra.conf", dropIns[0].Name)
	assert.Equal(t, "override.conf", dropIns[1].Name)
	assert.Contains(t, dropIns[0].Content, "TimeoutSec=30")
	assert.Contains(t, dropIns[1].Content, "Restart=always")
}

func TestListUnitDropInsNone(t *testing.T) {
	t.Parallel()

	r := &procfs.Reader{Root: t.TempDir()}
	assert.Empty(t, listUnitDropIns(r, "sshd.service"))
}
