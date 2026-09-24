package linux

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupIPv4RouteLongestPrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "net"), 0755))

	// /proc/net/route lines:
	// Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT
	// 10.0.0.0/8 via 10.0.0.1, and 10.1.2.0/24 via 10.1.2.1, plus default.
	route := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0100000A\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth0\t0000000A\t0100000A\t0003\t0\t0\t100\t000000FF\t0\t0\t0\n" +
		"eth1\t0002010A\t0102010A\t0003\t0\t0\t50\t00FFFFFF\t0\t0\t0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "net", "route"), []byte(route), 0644))

	r := &procfs.Reader{Root: dir}

	// 10.1.2.3 should match the /24 route (most specific).
	got, err := lookupRoute(r, net.ParseIP("10.1.2.3"))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "10.1.2.0/24", got.Destination)
	assert.Equal(t, "10.1.2.1", got.Gateway)
	assert.Equal(t, "eth1", got.Interface)
	assert.Equal(t, 50, got.Metric)
}

func TestLookupIPv4RouteDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "net"), 0755))

	route := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0100000A\t0003\t0\t0\t100\t00000000\t0\t0\t0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "net", "route"), []byte(route), 0644))

	r := &procfs.Reader{Root: dir}

	got, err := lookupRoute(r, net.ParseIP("8.8.8.8"))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "default", got.Destination)
	assert.Equal(t, "10.0.0.1", got.Gateway)
	assert.Equal(t, "eth0", got.Interface)
}

func TestLookupIPv4RouteNoMatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "net"), 0755))

	route := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"eth0\t0000000A\t00000000\t0003\t0\t0\t100\t000000FF\t0\t0\t0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "net", "route"), []byte(route), 0644))

	r := &procfs.Reader{Root: dir}

	got, err := lookupRoute(r, net.ParseIP("192.168.1.1"))
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLookupIPv6Route(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "net"), 0755))

	// /proc/net/ipv6_route format (10 fields):
	// dest_net dest_prefix src_net src_prefix next_hop metric refcnt flags ifindex iface
	// A default ::/0 route on eth0.
	route := "00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 00000000 00000000 00000000 00000000 eth0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "net", "ipv6_route"), []byte(route), 0644))

	r := &procfs.Reader{Root: dir}

	got, err := lookupRoute(r, net.ParseIP("2001:db8::1"))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "default", got.Destination)
	assert.Equal(t, "eth0", got.Interface)
}

func TestIPv4InNetwork(t *testing.T) {
	t.Parallel()

	ip := net.ParseIP("10.1.2.3")
	dest := net.ParseIP("10.1.2.0")
	mask := net.ParseIP("255.255.255.0")
	assert.True(t, ipv4InNetwork(ip, dest, mask))

	other := net.ParseIP("10.1.3.3")
	assert.False(t, ipv4InNetwork(other, dest, mask))
}

func TestIPv6InNetwork(t *testing.T) {
	t.Parallel()

	ip := net.ParseIP("2001:db8:1:2::1")
	dest := net.ParseIP("2001:db8:1:2::")
	assert.True(t, ipv6InNetwork(ip, dest, 64))
	assert.False(t, ipv6InNetwork(ip, dest, 128))

	other := net.ParseIP("2001:db8:1:3::1")
	assert.False(t, ipv6InNetwork(other, dest, 64))
}
