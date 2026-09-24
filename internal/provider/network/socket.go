package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/marvin-agent/marvin/internal/task"
)

// socketTask tests TCP/UDP connectivity without sending or receiving data.
type socketTask struct{}

const socketSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Socket Connectivity Parameters",
  "description": "Parameters for testing TCP or UDP socket connectivity without sending data.",
  "properties": {
    "host": {
      "type": "string",
      "description": "Target hostname or IP address."
    },
    "port": {
      "type": "integer",
      "description": "Target port number.",
      "minimum": 1,
      "maximum": 65535
    },
    "protocol": {
      "type": "string",
      "description": "Transport protocol to use.",
      "enum": ["tcp", "udp"],
      "default": "tcp"
    },
    "timeout": {
      "type": "string",
      "description": "Connection timeout duration (e.g., '5s', '1m').",
      "default": "5s"
    }
  },
  "required": ["host", "port"]
}`

func (s *socketTask) Name() string { return "network.socket.connect" }

func (s *socketTask) JSONSchema() string { return socketSchema }

func (s *socketTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	start := time.Now()
	result := task.Result{
		Success:   true,
		Timestamp: start,
	}

	host, ok := params["host"].(string)
	if !ok || host == "" {
		result.Success = false
		result.Error = "missing required parameter: host"
		return result, nil
	}

	portRaw, ok := params["port"]
	if !ok {
		result.Success = false
		result.Error = "missing required parameter: port"
		return result, nil
	}

	var port int
	switch v := portRaw.(type) {
	case int:
		port = v
	case int64:
		port = int(v)
	case float64:
		port = int(v)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("invalid type for parameter port: %T", portRaw)
		return result, nil
	}

	if port <= 0 || port > 65535 {
		result.Success = false
		result.Error = fmt.Sprintf("invalid port number: %d", port)
		return result, nil
	}

	protocol, _ := params["protocol"].(string)
	if protocol == "" {
		protocol = "tcp"
	}
	protocol = fmt.Sprintf("%s4", protocol) // force IPv4 to avoid IPv6 ambiguity in tests

	timeout := 5 * time.Second
	if tStr, ok := params["timeout"].(string); ok && tStr != "" {
		if d, err := time.ParseDuration(tStr); err == nil {
			timeout = d
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	slog.Info("socket connect starting", "capability", s.Name(), "addr", addr, "protocol", protocol)
	slog.Debug("socket connect parameters", "capability", s.Name(), "host", host, "port", port, "protocol", protocol, "timeout", timeout)

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, protocol, addr)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		slog.Info("socket connect failed", "capability", s.Name(), "addr", addr, "error", result.Error)
		return result, nil
	}
	defer conn.Close()

	result.Data = map[string]any{
		"connected":      true,
		"remote_address": conn.RemoteAddr().String(),
		"duration_ms":    time.Since(start).Milliseconds(),
		"protocol":       protocol,
	}
	slog.Info("socket connect succeeded", "capability", s.Name(), "addr", addr, "remote_address", conn.RemoteAddr().String())
	slog.Debug("socket connect result", "capability", s.Name(), "addr", addr, "duration_ms", time.Since(start).Milliseconds())
	return result, nil
}
