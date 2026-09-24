package network

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocketConnectSuccess(t *testing.T) {
	t.Parallel()

	// Start a local TCP listener
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	s := &socketTask{}
	res, err := s.Execute(context.Background(), map[string]any{
		"host": "127.0.0.1",
		"port": float64(ln.Addr().(*net.TCPAddr).Port),
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.NotNil(t, res.Data)

	data, ok := res.Data.(map[string]any)
	require.True(t, ok)
	assert.True(t, data["connected"].(bool))
	assert.Contains(t, data["remote_address"].(string), ln.Addr().String())
}

func TestSocketMissingHost(t *testing.T) {
	t.Parallel()

	s := &socketTask{}
	res, err := s.Execute(context.Background(), map[string]any{"port": 80})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "host")
}

func TestSocketMissingPort(t *testing.T) {
	t.Parallel()

	s := &socketTask{}
	res, err := s.Execute(context.Background(), map[string]any{"host": "127.0.0.1"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "port")
}

func TestSocketInvalidPort(t *testing.T) {
	t.Parallel()

	s := &socketTask{}
	res, err := s.Execute(context.Background(), map[string]any{
		"host": "127.0.0.1",
		"port": 99999,
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "invalid port")
}

func TestSocketTimeout(t *testing.T) {
	t.Parallel()

	s := &socketTask{}
	// Use a non-routable IP to force a timeout quickly
	res, err := s.Execute(context.Background(), map[string]any{
		"host":    "192.0.2.1",
		"port":    12345,
		"timeout": "100ms",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	// Should have timed out
	assert.NotEmpty(t, res.Error)
}

func TestSocketNoDataExchange(t *testing.T) {
	t.Parallel()

	// Start a listener that will fail the test if it receives data
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	readCh := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		readCh <- buf[:n]
	}()

	s := &socketTask{}
	res, err := s.Execute(context.Background(), map[string]any{
		"host": "127.0.0.1",
		"port": float64(ln.Addr().(*net.TCPAddr).Port),
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	select {
	case data := <-readCh:
		assert.Empty(t, data, "socket task must not send any data")
	case <-time.After(500 * time.Millisecond):
		// expected: no data was sent
	}
}
