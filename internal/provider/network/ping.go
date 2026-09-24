package network

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/marvin-agent/marvin/internal/task"
	"github.com/prometheus-community/pro-bing"
)

// pingTask sends ICMP echo requests using pro-bing (pure Go).
type pingTask struct{}

const pingSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ICMP Echo Request Parameters",
  "description": "Parameters for sending ICMP echo requests (ping).",
  "properties": {
    "target": {
      "type": "string",
      "description": "Hostname or IP address to ping."
    },
    "count": {
      "type": "integer",
      "description": "Number of ICMP packets to send.",
      "minimum": 1,
      "maximum": 30,
      "default": 4
    }
  },
  "required": ["target"]
}`

const maxPingCount = 30

func (p *pingTask) Name() string { return "network.icmp.echo_request" }

func (p *pingTask) JSONSchema() string { return pingSchema }

func (p *pingTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	start := time.Now()
	result := task.Result{
		Success:   true,
		Timestamp: start,
	}

	target, ok := params["target"].(string)
	if !ok || target == "" {
		result.Success = false
		result.Error = "missing required parameter: target"
		return result, nil
	}

	count := 4
	if c, ok := params["count"]; ok {
		switch v := c.(type) {
		case int:
			count = v
		case int64:
			count = int(v)
		case float64:
			count = int(v)
		}
	}

	if count <= 0 {
		count = 4
	}
	if count > maxPingCount {
		result.Success = false
		result.Error = fmt.Sprintf("count exceeds maximum allowed (%d)", maxPingCount)
		slog.Info("ping failed: count exceeds maximum", "capability", p.Name(), "target", target, "count", count)
		return result, nil
	}

	slog.Info("ping starting", "capability", p.Name(), "target", target, "count", count)

	pinger, err := probing.NewPinger(target)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create pinger: %v", err)
		slog.Info("ping failed: could not create pinger", "capability", p.Name(), "target", target, "error", err)
		return result, nil
	}

	pinger.Count = count
	pinger.Timeout = 10 * time.Second
	pinger.SetPrivileged(false)

	slog.Debug("pinger configured", "capability", p.Name(), "target", target, "count", count, "privileged", false)

	// Run in background and respect context cancellation
	done := make(chan error, 1)
	go func() {
		done <- pinger.Run()
	}()

	select {
	case <-ctx.Done():
		pinger.Stop()
		result.Success = false
		result.Error = ctx.Err().Error()
		slog.Info("ping cancelled by context", "capability", p.Name(), "target", target, "error", result.Error)
		return result, nil
	case err := <-done:
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("ping failed: %v", err)
			slog.Info("ping failed", "capability", p.Name(), "target", target, "error", result.Error)
			return result, nil
		}
	}

	stats := pinger.Statistics()
	resolved := ""
	if ip := pinger.IPAddr(); ip != nil {
		resolved = ip.String()
	}

	result.Data = map[string]any{
		"packets_sent":        stats.PacketsSent,
		"packets_received":    stats.PacketsRecv,
		"packet_loss_percent": stats.PacketLoss,
		"min_rtt_ms":          durationMs(stats.MinRtt),
		"max_rtt_ms":          durationMs(stats.MaxRtt),
		"avg_rtt_ms":          durationMs(stats.AvgRtt),
		"target_resolved":     resolved,
		"duration_ms":         time.Since(start).Milliseconds(),
	}
	slog.Info("ping succeeded", "capability", p.Name(), "target", target, "resolved", resolved, "packets_sent", stats.PacketsSent, "packets_received", stats.PacketsRecv)
	slog.Debug("ping statistics", "capability", p.Name(), "target", target, "packet_loss_percent", stats.PacketLoss, "min_rtt_ms", durationMs(stats.MinRtt), "avg_rtt_ms", durationMs(stats.AvgRtt), "max_rtt_ms", durationMs(stats.MaxRtt))
	return result, nil
}

func durationMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
