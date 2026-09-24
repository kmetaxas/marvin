//go:build linux

package network

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestParseSockExtendedErrTimeExceeded(t *testing.T) {
	t.Parallel()

	data := extendedErrFixture(icmpTimeExceeded, 0, net.IPv4(192, 0, 2, 1))
	ip, reached, ok := parseSockExtendedErr(data)

	require.True(t, ok)
	assert.False(t, reached)
	assert.Equal(t, "192.0.2.1", ip.String())
}

func TestParseSockExtendedErrPortUnreachableReached(t *testing.T) {
	t.Parallel()

	data := extendedErrFixture(icmpDestUnreach, icmpPortUnreach, net.IPv4(198, 51, 100, 2))
	ip, reached, ok := parseSockExtendedErr(data)

	require.True(t, ok)
	assert.True(t, reached)
	assert.Equal(t, "198.51.100.2", ip.String())
}

func TestParseSockExtendedErrIgnoresNonICMPOrigins(t *testing.T) {
	t.Parallel()

	data := extendedErrFixture(icmpTimeExceeded, 0, net.IPv4(203, 0, 113, 3))
	data[4] = 1

	_, _, ok := parseSockExtendedErr(data)
	assert.False(t, ok)
}

func TestDurationToTimevalRoundsUpToMicrosecond(t *testing.T) {
	t.Parallel()

	timeval := durationToTimeval(time.Nanosecond)
	assert.Equal(t, int64(0), timeval.Sec)
	assert.Equal(t, int64(1), timeval.Usec)
}

func extendedErrFixture(icmpType, icmpCode byte, offender net.IP) []byte {
	data := make([]byte, 32)
	data[4] = icmpOrigin
	data[5] = icmpType
	data[6] = icmpCode
	data[16] = unix.AF_INET
	copy(data[20:24], offender.To4())
	return data
}
