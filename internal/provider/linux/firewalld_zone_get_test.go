package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFirewalldZoneFull(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "etc", "firewalld", "zones"), 0755))

	xml := `<zone target="ACCEPT">
  <short>Public</short>
  <description>For use in public areas.</description>
  <service name="ssh"/>
  <service name="dhcpv6-client"/>
  <port port="80" protocol="tcp"/>
  <port port="443" protocol="tcp"/>
  <port port="53" protocol="udp"/>
  <source address="192.168.1.0/24"/>
  <interface name="eth0"/>
  <forward/>
  <masquerade/>
</zone>`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etc", "firewalld", "zones", "public.xml"), []byte(xml), 0644))

	r := &procfs.Reader{Root: dir}
	detail, err := readFirewalldZone(r, "public")
	require.NoError(t, err)

	assert.Equal(t, "public", detail["name"])
	assert.Equal(t, "Public", detail["short"])
	assert.Equal(t, "For use in public areas.", detail["description"])
	assert.Equal(t, []string{"ssh", "dhcpv6-client"}, detail["services"])
	assert.Equal(t, []map[string]string{
		{"protocol": "tcp", "port": "80"},
		{"protocol": "tcp", "port": "443"},
		{"protocol": "udp", "port": "53"},
	}, detail["ports"])
	assert.Equal(t, []string{"192.168.1.0/24"}, detail["sources"])
	assert.Equal(t, []string{"eth0"}, detail["interfaces"])
	assert.Equal(t, true, detail["forward"])
	assert.Equal(t, true, detail["masquerade"])
}

func TestReadFirewalldZoneMinimal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "etc", "firewalld", "zones"), 0755))

	xml := `<zone target="default">
  <short>Trusted</short>
</zone>`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etc", "firewalld", "zones", "trusted.xml"), []byte(xml), 0644))

	r := &procfs.Reader{Root: dir}
	detail, err := readFirewalldZone(r, "trusted")
	require.NoError(t, err)

	assert.Equal(t, "trusted", detail["name"])
	assert.Equal(t, "Trusted", detail["short"])
	assert.Empty(t, detail["description"])
	assert.Empty(t, detail["services"])
	assert.Empty(t, detail["ports"])
	assert.Empty(t, detail["sources"])
	assert.Empty(t, detail["interfaces"])
	assert.Equal(t, false, detail["forward"])
	assert.Equal(t, false, detail["masquerade"])
}

func TestReadFirewalldZoneMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}

	_, err := readFirewalldZone(r, "nonexistent")
	require.Error(t, err)
}

func TestValidateZoneName(t *testing.T) {
	t.Parallel()

	assert.NoError(t, validateZoneName("public"))
	assert.NoError(t, validateZoneName("dmz-internal"))
	assert.Error(t, validateZoneName("../etc/passwd"))
	assert.Error(t, validateZoneName("a/b"))
	assert.Error(t, validateZoneName(".."))
}
