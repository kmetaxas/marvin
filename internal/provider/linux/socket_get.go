package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// socketGetTask implements the linux.socket.get capability.
type socketGetTask struct{ provider *Provider }

const socketGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Socket Get Parameters",
  "description": "Parameters for retrieving detailed information about a particular socket.",
  "properties": {
    "local_address": {
      "type": "string",
      "description": "Local IP address of the socket."
    },
    "local_port": {
      "type": "integer",
      "description": "Local port number of the socket.",
      "minimum": 1,
      "maximum": 65535
    },
    "remote_address": {
      "type": "string",
      "description": "Remote IP address of the socket."
    },
    "remote_port": {
      "type": "integer",
      "description": "Remote port number of the socket.",
      "minimum": 1,
      "maximum": 65535
    },
    "protocol": {
      "type": "string",
      "description": "Socket protocol.",
      "enum": ["tcp", "tcp6", "udp", "unix"]
    }
  }
}`

func (t *socketGetTask) Name() string       { return "linux.socket.get" }
func (t *socketGetTask) JSONSchema() string { return socketGetSchema }

func (t *socketGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("socket.get starting", "capability", t.Name())

	localAddr, _ := params["local_address"].(string)
	localPortRaw, _ := params["local_port"]
	remoteAddr, _ := params["remote_address"].(string)
	remotePortRaw, _ := params["remote_port"]
	protocol, _ := params["protocol"].(string)

	var localPort, remotePort int
	if localPortRaw != nil {
		localPort = intParam(localPortRaw)
	}
	if remotePortRaw != nil {
		remotePort = intParam(remotePortRaw)
	}

	reader := t.provider.CurrentReader()

	sockets, err := listSockets(reader, protocol, "")
	if err != nil {
		slog.Info("socket.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	var found *SocketInfo
	for i := range sockets {
		s := &sockets[i]
		match := true
		if localAddr != "" && s.LocalAddress != localAddr {
			match = false
		}
		if localPort > 0 && s.LocalPort != localPort {
			match = false
		}
		if remoteAddr != "" && s.RemoteAddress != remoteAddr {
			match = false
		}
		if remotePort > 0 && s.RemotePort != remotePort {
			match = false
		}
		if match {
			found = s
			break
		}
	}

	if found == nil {
		return task.Result{
			Success: false,
			Error:   fmt.Sprintf("socket not found: %s:%d -> %s:%d (%s)", localAddr, localPort, remoteAddr, remotePort, protocol),
		}, nil
	}

	result := map[string]any{
		"socket": found,
	}
	slog.Info("socket.get succeeded", "capability", t.Name())
	return common.SuccessResult(result), nil
}

var _ task.Task = (*socketGetTask)(nil)

func intParam(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}
