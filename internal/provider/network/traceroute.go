package network

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/task"
)

type tracerouteTask struct{}

const tracerouteSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Traceroute Parameters",
  "description": "Parameters for performing a traceroute to trace network path hops.",
  "properties": {
    "target": {
      "type": "string",
      "description": "Hostname or IP address to trace route to."
    },
    "max_hops": {
      "type": "integer",
      "description": "Maximum number of hops to trace.",
      "minimum": 1,
      "maximum": 64,
      "default": 30
    },
    "timeout": {
      "type": "string",
      "description": "Timeout per probe (e.g., '2s', '1s').",
      "default": "2s"
    },
    "protocol": {
      "type": "string",
      "description": "Protocol to use for probes. UDP traceroute uses unprivileged UDP sockets with kernel ICMP error queues where supported.",
      "enum": ["udp"],
      "default": "udp"
    }
  },
  "required": ["target"]
}`

const (
	defaultTracerouteMaxHops = 30
	maxTracerouteHops        = 64
	defaultTracerouteTimeout = 2 * time.Second
)

type tracerouteHop struct {
	TTL      int     `json:"ttl"`
	Address  string  `json:"address,omitempty"`
	Hostname string  `json:"hostname,omitempty"`
	RTTMs    float64 `json:"rtt_ms,omitempty"`
	Reached  bool    `json:"reached,omitempty"`
	Error    string  `json:"error,omitempty"`
}

func (t *tracerouteTask) Name() string { return "network.traceroute" }

func (t *tracerouteTask) JSONSchema() string { return tracerouteSchema }

func (t *tracerouteTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	start := time.Now()
	result := task.Result{Success: true, Timestamp: start}

	config, err := parseTracerouteParams(params)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, nil
	}

	slog.Info("traceroute starting", "capability", t.Name(), "target", config.target, "max_hops", config.maxHops, "timeout", config.timeout, "protocol", config.protocol)

	targetIP, err := resolveTracerouteTarget(ctx, config.target)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		slog.Info("traceroute failed: target resolution failed", "capability", t.Name(), "target", config.target, "error", result.Error)
		return result, nil
	}

	hops, reached, err := runPlatformTraceroute(ctx, targetIP, config)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		slog.Info("traceroute failed", "capability", t.Name(), "target", config.target, "target_ip", targetIP.String(), "error", result.Error)
		return result, nil
	}

	result.Data = map[string]any{
		"hops":        hops,
		"target":      config.target,
		"completed":   reached,
		"target_ip":   targetIP.String(),
		"protocol":    config.protocol,
		"duration_ms": time.Since(start).Milliseconds(),
	}
	slog.Info("traceroute finished", "capability", t.Name(), "target", config.target, "target_ip", targetIP.String(), "completed", reached, "hops", len(hops))
	return result, nil
}

type tracerouteConfig struct {
	target   string
	maxHops  int
	timeout  time.Duration
	protocol string
}

func parseTracerouteParams(params map[string]any) (tracerouteConfig, error) {
	target, ok := params["target"].(string)
	if !ok || strings.TrimSpace(target) == "" {
		return tracerouteConfig{}, errors.New("missing required parameter: target")
	}

	maxHops, err := optionalTracerouteInt(params, "max_hops", defaultTracerouteMaxHops)
	if err != nil {
		return tracerouteConfig{}, err
	}
	if maxHops < 1 || maxHops > maxTracerouteHops {
		return tracerouteConfig{}, fmt.Errorf("max_hops must be between 1 and %d", maxTracerouteHops)
	}

	timeout := defaultTracerouteTimeout
	if raw, ok := params["timeout"]; ok && raw != nil {
		timeoutStr, ok := raw.(string)
		if !ok || strings.TrimSpace(timeoutStr) == "" {
			return tracerouteConfig{}, errors.New("parameter timeout must be a duration string")
		}
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil || timeout <= 0 {
			return tracerouteConfig{}, fmt.Errorf("invalid timeout duration: %s", timeoutStr)
		}
	}

	protocol := "udp"
	if raw, ok := params["protocol"]; ok && raw != nil {
		value, ok := raw.(string)
		if !ok {
			return tracerouteConfig{}, errors.New("parameter protocol must be a string")
		}
		protocol = strings.ToLower(strings.TrimSpace(value))
	}
	if protocol == "" {
		protocol = "udp"
	}
	if protocol != "udp" {
		return tracerouteConfig{}, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	return tracerouteConfig{target: strings.TrimSpace(target), maxHops: maxHops, timeout: timeout, protocol: protocol}, nil
}

func optionalTracerouteInt(params map[string]any, key string, defaultValue int) (int, error) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return defaultValue, nil
	}
	switch value := raw.(type) {
	case int:
		return value, nil
	case int32:
		return int(value), nil
	case int64:
		return int(value), nil
	case float64:
		if value != float64(int(value)) {
			return 0, fmt.Errorf("parameter %s must be an integer", key)
		}
		return int(value), nil
	default:
		return 0, fmt.Errorf("parameter %s must be an integer", key)
	}
}

func resolveTracerouteTarget(ctx context.Context, target string) (net.IP, error) {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, target)
	if err != nil {
		return nil, err
	}
	for _, addr := range ips {
		if ip := addr.IP.To4(); ip != nil {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address found for target %q", target)
}
